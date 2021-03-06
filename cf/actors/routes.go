package actors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudfoundry/cli/cf/api"
	"github.com/cloudfoundry/cli/cf/errors"
	. "github.com/cloudfoundry/cli/cf/i18n"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/cloudfoundry/cli/cf/terminal"
)

//go:generate counterfeiter . RouteActor

type RouteActor interface {
	CreateRandomTCPRoute(domain models.DomainFields) (models.Route, error)
	FindOrCreateRoute(hostname string, domain models.DomainFields, path string, port int, useRandomPort bool) (models.Route, error)
	BindRoute(app models.Application, route models.Route) error
	UnbindAll(app models.Application) error
	FindDomain(routeName string) (string, models.DomainFields, error)
	FindPath(routeName string) (string, string)
	FindPort(routeName string) (string, int, error)
	FindAndBindRoute(routeName string, app models.Application, appParamsFromContext models.AppParams) error
}

type routeActor struct {
	ui         terminal.UI
	routeRepo  api.RouteRepository
	domainRepo api.DomainRepository
}

func NewRouteActor(ui terminal.UI, routeRepo api.RouteRepository, domainRepo api.DomainRepository) routeActor {
	return routeActor{
		ui:         ui,
		routeRepo:  routeRepo,
		domainRepo: domainRepo,
	}
}

func (routeActor routeActor) CreateRandomTCPRoute(domain models.DomainFields) (models.Route, error) {
	routeActor.ui.Say(T("Creating random route for {{.Domain}}", map[string]interface{}{
		"Domain": terminal.EntityNameColor(domain.Name),
	}) + "...")

	route, err := routeActor.routeRepo.Create("", domain, "", 0, true)
	if err != nil {
		return models.Route{}, err
	}

	return route, nil
}

func (routeActor routeActor) FindOrCreateRoute(hostname string, domain models.DomainFields, path string, port int, useRandomPort bool) (models.Route, error) {
	route, err := routeActor.routeRepo.Find(hostname, domain, path, port)

	switch err.(type) {
	case nil:
		routeActor.ui.Say(
			T("Using route {{.RouteURL}}",
				map[string]interface{}{
					"RouteURL": terminal.EntityNameColor(route.URL()),
				}),
		)
	case *errors.ModelNotFoundError:
		if useRandomPort {
			route, err = routeActor.CreateRandomTCPRoute(domain)
		} else {
			routeActor.ui.Say(
				T("Creating route {{.Hostname}}...",
					map[string]interface{}{
						"Hostname": terminal.EntityNameColor(domain.URLForHostAndPath(hostname, path, port)),
					}),
			)

			route, err = routeActor.routeRepo.Create(hostname, domain, path, port, useRandomPort)
		}

		routeActor.ui.Ok()
		routeActor.ui.Say("")
	}

	return route, err
}

func (routeActor routeActor) BindRoute(app models.Application, route models.Route) error {
	if !app.HasRoute(route) {
		routeActor.ui.Say(T(
			"Binding {{.URL}} to {{.AppName}}...",
			map[string]interface{}{
				"URL":     terminal.EntityNameColor(route.URL()),
				"AppName": terminal.EntityNameColor(app.Name),
			}),
		)

		err := routeActor.routeRepo.Bind(route.GUID, app.GUID)
		switch err := err.(type) {
		case nil:
			routeActor.ui.Ok()
			routeActor.ui.Say("")
			return nil
		case errors.HTTPError:
			if err.ErrorCode() == errors.InvalidRelation {
				return errors.New(T(
					"The route {{.URL}} is already in use.\nTIP: Change the hostname with -n HOSTNAME or use --random-route to generate a new route and then push again.",
					map[string]interface{}{
						"URL": route.URL(),
					}),
				)
			}
		}
		return err
	}
	return nil
}

func (routeActor routeActor) UnbindAll(app models.Application) error {
	for _, route := range app.Routes {
		routeActor.ui.Say(T(
			"Removing route {{.URL}}...",
			map[string]interface{}{
				"URL": terminal.EntityNameColor(route.URL()),
			}),
		)
		err := routeActor.routeRepo.Unbind(route.GUID, app.GUID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (routeActor routeActor) FindDomain(routeName string) (string, models.DomainFields, error) {
	host, domain, continueSearch, err := parseRoute(routeName, routeActor.domainRepo.FindPrivateByName)
	if continueSearch {
		host, domain, _, err = parseRoute(routeName, routeActor.domainRepo.FindSharedByName)
	}
	return host, domain, err
}

func (routeActor routeActor) FindPath(routeName string) (string, string) {
	routeSlice := strings.Split(routeName, "/")
	return routeSlice[0], strings.Join(routeSlice[1:], "/")
}

func (routeActor routeActor) FindPort(routeName string) (string, int, error) {
	var err error
	routeSlice := strings.Split(routeName, ":")
	port := 0
	if len(routeSlice) == 2 {
		port, err = strconv.Atoi(routeSlice[1])
		if err != nil {
			return "", 0, errors.New(T("Invalid port for route {{.RouteName}}",
				map[string]interface{}{
					"RouteName": routeName,
				},
			))
		}
	}
	return routeSlice[0], port, nil
}

func (routeActor routeActor) FindAndBindRoute(routeName string, app models.Application, appParamsFromContext models.AppParams) error {
	routeWithoutPath, path := routeActor.FindPath(routeName)

	routeWithoutPathAndPort, port, err := routeActor.FindPort(routeWithoutPath)
	if err != nil {
		return err
	}

	hostname, domain, err := routeActor.FindDomain(routeWithoutPathAndPort)
	if err != nil {
		return err
	}

	err = validateRoute(routeName, domain.RouterGroupType, port, path)
	if err != nil {
		return err
	}

	replaceHostname(domain.RouterGroupType, appParamsFromContext.Hosts, &hostname)

	route, err := routeActor.FindOrCreateRoute(hostname, domain, path, port, false)
	if err != nil {
		return err
	}

	return routeActor.BindRoute(app, route)
}

func validateRoute(routeName string, domainType string, port int, path string) error {
	if domainType == "tcp" && path != "" {
		return fmt.Errorf(T("Path not allowed in TCP route {{.RouteName}}",
			map[string]interface{}{
				"RouteName": routeName,
			},
		))
	}

	if domainType == "" && port != 0 {
		return fmt.Errorf(T("Port not allowed in HTTP route {{.RouteName}}",
			map[string]interface{}{
				"RouteName": routeName,
			},
		))
	}

	return nil
}

func replaceHostname(domainType string, hosts []string, hostname *string) {
	if domainType == "" && len(hosts) > 0 && hosts[0] != "" {
		*hostname = hosts[0]
	}
}

func validateFoundDomain(domain models.DomainFields, err error) (bool, error) {
	switch err.(type) {
	case *errors.ModelNotFoundError:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

func parseRoute(routeName string, findFunc func(domainName string) (models.DomainFields, error)) (string, models.DomainFields, bool, error) {
	var domain models.DomainFields
	var err error
	var found bool
	domain, err = findFunc(routeName)
	found, err = validateFoundDomain(domain, err)
	if err != nil {
		return "", models.DomainFields{}, false, err
	}
	if found {
		return "", domain, false, nil
	}

	routeParts := strings.Split(routeName, ".")
	domain, err = findFunc(strings.Join(routeParts[1:], "."))
	found, err = validateFoundDomain(domain, err)
	if err != nil {
		return "", models.DomainFields{}, false, err
	}
	if found {
		return routeParts[0], domain, false, nil
	}

	return "", models.DomainFields{}, true, fmt.Errorf(T(
		"The route {{.RouteName}} did not match any existing domains.",
		map[string]interface{}{
			"RouteName": routeName,
		},
	))
}
