// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/cloudfoundry/cli/cf/v3/repository"
)

type FakeTokenHandler struct {
	DoStub        func(cb func() ([]byte, error)) ([]byte, error)
	doMutex       sync.RWMutex
	doArgsForCall []struct {
		cb func() ([]byte, error)
	}
	doReturns struct {
		result1 []byte
		result2 error
	}
}

func (fake *FakeTokenHandler) Do(cb func() ([]byte, error)) ([]byte, error) {
	fake.doMutex.Lock()
	fake.doArgsForCall = append(fake.doArgsForCall, struct {
		cb func() ([]byte, error)
	}{cb})
	fake.doMutex.Unlock()
	if fake.DoStub != nil {
		return fake.DoStub(cb)
	} else {
		return fake.doReturns.result1, fake.doReturns.result2
	}
}

func (fake *FakeTokenHandler) DoCallCount() int {
	fake.doMutex.RLock()
	defer fake.doMutex.RUnlock()
	return len(fake.doArgsForCall)
}

func (fake *FakeTokenHandler) DoArgsForCall(i int) func() ([]byte, error) {
	fake.doMutex.RLock()
	defer fake.doMutex.RUnlock()
	return fake.doArgsForCall[i].cb
}

func (fake *FakeTokenHandler) DoReturns(result1 []byte, result2 error) {
	fake.DoStub = nil
	fake.doReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

var _ repository.TokenHandler = new(FakeTokenHandler)