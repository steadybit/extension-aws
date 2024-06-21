package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

type ecsDescribeServicesApiMock struct {
	mock.Mock
}

func (m *ecsDescribeServicesApiMock) DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func TestServiceDescriptionPoller_awaits_first_response(t *testing.T) {
	account := "awsAccount"
	cluster := "clusterArn"
	service := "serviceArn"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller := NewServiceDescriptionPoller()
	poller.ticker = time.NewTicker(1 * time.Millisecond)
	poller.apiClientProvider = func(account string) (ecsDescribeServicesApi, error) {
		mockedApi := new(ecsDescribeServicesApiMock)
		mockedApi.On("DescribeServices", mock.Anything, mock.Anything).Return(&ecs.DescribeServicesOutput{
			Services: []types.Service{{
				ServiceArn:   extutil.Ptr(service),
				DesiredCount: 1,
				RunningCount: 1,
			}},
		}, nil)
		return mockedApi, nil
	}

	poller.Register(account, cluster, service)
	latest := poller.Latest(account, cluster, service)
	assert.Nil(t, latest)

	poller.Start(ctx)
	latest = poller.AwaitLatest(account, cluster, service)

	assert.NotNil(t, latest)
	assert.NotNil(t, latest.service)
	assert.Equal(t, *latest.service.ServiceArn, service)
}

func TestServiceDescriptionPoller_registers_and_unregisters_services(t *testing.T) {
	p := NewServiceDescriptionPoller()
	p.Register("a", "b", "c")
	p.Register("a", "b", "e")
	assert.Len(t, p.polls, 1)
	assert.Len(t, p.polls["a"], 1)
	assert.Len(t, p.polls["a"]["b"], 2)
	assert.NotNil(t, p.polls["a"]["b"]["c"])
	assert.NotNil(t, p.polls["a"]["b"]["e"])

	p.Unregister("a", "b", "c")
	assert.Len(t, p.polls["a"]["b"], 1)

	p.Unregister("a", "b", "e")
	assert.Len(t, p.polls, 0)
}

func TestServiceDescriptionPoller_registers_and_unregisters_service_multiple_times(t *testing.T) {
	p := NewServiceDescriptionPoller()
	p.Register("a", "b", "c")
	p.Register("a", "b", "c")
	assert.Len(t, p.polls, 1)
	assert.Len(t, p.polls["a"], 1)
	assert.Len(t, p.polls["a"]["b"], 1)

	record := &pollRecord{
		count: 1,
	}
	assert.Equal(t, record, p.polls["a"]["b"]["c"])

	p.Unregister("a", "b", "c")
	assert.Len(t, p.polls["a"]["b"], 1)

	p.Unregister("a", "b", "c")
	assert.Len(t, p.polls, 0)
}
