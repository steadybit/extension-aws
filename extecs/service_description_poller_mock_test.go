package extecs

import (
	"context"
	"github.com/stretchr/testify/mock"
)

type ServiceDescriptionPollerMock struct {
	mock.Mock
}

func (m *ServiceDescriptionPollerMock) Start(ctx context.Context) {
	m.Called(ctx)
}

func (m *ServiceDescriptionPollerMock) Register(account string, cluster string, service string) {
	m.Called(account, cluster, service)
}

func (m *ServiceDescriptionPollerMock) Unregister(account string, cluster string, service string) {
	m.Called(account, cluster, service)
}

func (m *ServiceDescriptionPollerMock) Latest(account string, cluster string, service string) *PollService {
	args := m.Called(account, cluster, service)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*PollService)
}

func (m *ServiceDescriptionPollerMock) AwaitLatest(account string, cluster string, service string) *PollService {
	args := m.Called(account, cluster, service)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*PollService)
}
