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
	region := "region"
	cluster := "clusterArn"
	service := "serviceArn"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller := NewServiceDescriptionPoller()
	poller.ticker = time.NewTicker(1 * time.Millisecond)
	poller.apiClientProvider = func(account string, region string) (ecsDescribeServicesApi, error) {
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

	poller.Register(account, region, cluster, service)
	latest := poller.Latest(account, region, cluster, service)
	assert.Nil(t, latest)

	poller.Start(ctx)
	latest = poller.AwaitLatest(account, region, cluster, service)

	assert.NotNil(t, latest)
	assert.NotNil(t, latest.service)
	assert.Equal(t, *latest.service.ServiceArn, service)
}

func TestServiceDescriptionPoller_registers_and_unregisters_services(t *testing.T) {
	p := NewServiceDescriptionPoller()
	p.Register("a", "b", "c", "d")
	p.Register("a", "b", "c", "e")
	assert.Len(t, p.polls, 1)
	assert.Len(t, p.polls["a"], 1)
	assert.Len(t, p.polls["a"]["b"], 1)
	assert.Len(t, p.polls["a"]["b"]["c"], 2)
	assert.NotNil(t, p.polls["a"]["b"]["c"]["d"])
	assert.NotNil(t, p.polls["a"]["b"]["c"]["e"])

	p.Unregister("a", "b", "c", "d")
	assert.Len(t, p.polls["a"]["b"]["c"], 1)

	p.Unregister("a", "b", "c", "e")
	assert.Len(t, p.polls, 0)
}

func TestServiceDescriptionPoller_registers_and_unregisters_service_multiple_times(t *testing.T) {
	p := NewServiceDescriptionPoller()
	p.Register("a", "b", "c", "d")
	p.Register("a", "b", "c", "d")
	assert.Len(t, p.polls, 1)
	assert.Len(t, p.polls["a"], 1)
	assert.Len(t, p.polls["a"]["b"], 1)
	assert.Len(t, p.polls["a"]["b"]["c"], 1)

	record := &pollRecord{
		count: 1,
	}
	assert.Equal(t, record, p.polls["a"]["b"]["c"]["d"])

	p.Unregister("a", "b", "c", "d")
	assert.Len(t, p.polls["a"]["b"]["c"], 1)

	p.Unregister("a", "b", "c", "d")
	assert.Len(t, p.polls, 0)
}
