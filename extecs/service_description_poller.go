package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extutil"
	"golang.org/x/exp/maps"
	"sync"
	"time"
)

const maxServicePageSize = 10

type ecsDescribeServicesApi interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

type PollService struct {
	service *types.Service
	failure *types.Failure
}
type pollServices map[string]*PollService
type pollClusters map[string]pollServices
type pollAccounts map[string]pollClusters

type ServiceDescriptionPoller struct {
	apiClientProvider func(account string) (ecsDescribeServicesApi, error)
	ticker            *time.Ticker
	m                 *sync.RWMutex
	c                 *sync.Cond
	polls             pollAccounts
	lastPolled        time.Time
}

func NewServiceDescriptionPoller() *ServiceDescriptionPoller {
	m := sync.RWMutex{}
	return &ServiceDescriptionPoller{
		apiClientProvider: defaultDescribeServiceProvider,
		ticker:            time.NewTicker(5 * time.Second),
		m:                 &m,
		c:                 sync.NewCond(&m),
		polls:             pollAccounts{},
	}
}

func (p *ServiceDescriptionPoller) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.pollAll(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (p *ServiceDescriptionPoller) Register(account string, cluster string, service string) {
	var ok bool
	p.m.Lock()
	defer p.m.Unlock()
	log.Debug().Msgf("register service %s", service)

	var clusters pollClusters
	if clusters, ok = p.polls[account]; !ok {
		clusters = make(pollClusters)
		p.polls[account] = clusters
	}

	var services pollServices
	if services, ok = clusters[cluster]; !ok {
		services = make(pollServices)
		clusters[cluster] = services
	}

	if _, ok = services[service]; !ok {
		services[service] = nil
	}
}

func (p *ServiceDescriptionPoller) Unregister(account string, cluster string, service string) {
	p.m.Lock()
	defer p.m.Unlock()
	log.Debug().Msgf("unregister service %s", service)
	if clusters, ok := p.polls[account]; ok {
		if services, ok := clusters[cluster]; ok {
			delete(services, service)
			if len(services) == 0 {
				delete(clusters, cluster)
			}
		}
		if len(clusters) == 0 {
			delete(p.polls, account)
		}
	}
	p.c.Broadcast()
}

func (p *ServiceDescriptionPoller) Latest(account string, cluster string, service string) *PollService {
	p.m.RLock()
	defer p.m.RUnlock()
	if clusters, ok := p.polls[account]; ok {
		if services, ok := clusters[cluster]; ok {
			return services[service]
		}
	}
	return nil
}

func (p *ServiceDescriptionPoller) AwaitLatest(account string, cluster string, service string) *PollService {
	p.m.Lock()
	defer p.m.Unlock()
	for {
		clusters, ok := p.polls[account]
		if !ok {
			return nil
		}
		services, ok := clusters[cluster]
		if !ok {
			return nil
		}
		latest := services[service]
		if latest != nil {
			return latest
		}
		p.c.Wait()
	}
}

func (p *ServiceDescriptionPoller) pollAll(ctx context.Context) {
	p.m.Lock()
	defer p.m.Unlock()
	lastPolled := time.Now()

	for account, clusters := range p.polls {
		client, err := p.apiClientProvider(account)
		if err != nil {
			log.Warn().TimeDiff("duration", time.Now(), lastPolled).Err(err).Msg("could not create api client")
			continue
		}

		for cluster, services := range clusters {
			servicesPages := utils.SplitIntoPages(maps.Keys(services), maxServicePageSize)
			for _, servicePage := range servicesPages {
				descriptions, err := client.DescribeServices(ctx, &ecs.DescribeServicesInput{
					Services: servicePage,
					Cluster:  extutil.Ptr(cluster),
				})
				if err != nil {
					log.Warn().TimeDiff("duration", time.Now(), lastPolled).Err(err).Msg("api call failed")
					continue
				}

				for _, service := range descriptions.Services {
					services[aws.ToString(service.ServiceArn)] = &PollService{
						service: &service,
					}
				}
				for _, failure := range descriptions.Failures {
					services[aws.ToString(failure.Arn)] = &PollService{
						failure: &failure,
					}
				}
			}
		}
	}
	p.lastPolled = lastPolled
	p.c.Broadcast()
}

func defaultDescribeServiceProvider(account string) (ecsDescribeServicesApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccount.AwsConfig), nil
}
