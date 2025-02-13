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

type ServiceDescriptionPoller interface {
	Start(ctx context.Context)
	Register(account string, region string, role *string, cluster string, service string)
	Unregister(account string, region string, role *string, cluster string, service string)
	Latest(account string, region string, role *string, cluster string, service string) *PollService
	AwaitLatest(account string, region string, role *string, cluster string, service string) *PollService
}

type PollService struct {
	service *types.Service
	failure *types.Failure
}
type pollRecord struct {
	count int
	value *PollService
}
type pollServices map[string]*pollRecord
type pollClusters map[string]pollServices
type pollRoles map[string]pollClusters
type pollRegions map[string]pollRoles
type pollAccounts map[string]pollRegions

type EcsServiceDescriptionPoller struct {
	apiClientProvider func(account string, region string, role *string) (ecs.DescribeServicesAPIClient, error)
	ticker            *time.Ticker
	m                 *sync.RWMutex
	c                 *sync.Cond
	polls             pollAccounts
}

var _ ServiceDescriptionPoller = EcsServiceDescriptionPoller{}

func NewServiceDescriptionPoller() *EcsServiceDescriptionPoller {
	m := sync.RWMutex{}
	return &EcsServiceDescriptionPoller{
		apiClientProvider: defaultDescribeServiceProvider,
		ticker:            time.NewTicker(5 * time.Second),
		m:                 &m,
		c:                 sync.NewCond(&m),
		polls:             pollAccounts{},
	}
}

func (p EcsServiceDescriptionPoller) Start(ctx context.Context) {
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

func (p EcsServiceDescriptionPoller) Register(account string, region string, role *string, cluster string, service string) {
	var ok bool
	p.m.Lock()
	defer p.m.Unlock()
	log.Debug().Msgf("register service %s", service)

	var regions pollRegions
	if regions, ok = p.polls[account]; !ok {
		regions = make(pollRegions)
		p.polls[account] = regions
	}

	var roles pollRoles
	if roles, ok = regions[region]; !ok {
		roles = make(pollRoles)
		regions[region] = roles
	}

	var clusters pollClusters
	roleOrEmpty := ""
	if role != nil {
		roleOrEmpty = *role
	}
	if clusters, ok = roles[roleOrEmpty]; !ok {
		clusters = make(pollClusters)
		roles[roleOrEmpty] = clusters
	}

	var services pollServices
	if services, ok = clusters[cluster]; !ok {
		services = make(pollServices)
		clusters[cluster] = services
	}

	record, ok := services[service]
	if ok {
		record.count = record.count + 1
	} else {
		services[service] = &pollRecord{}
	}
}

func (p EcsServiceDescriptionPoller) Unregister(account string, region string, role *string, cluster string, service string) {
	p.m.Lock()
	defer p.m.Unlock()
	log.Debug().Msgf("unregister service %s", service)
	roleOrEmpty := ""
	if role != nil {
		roleOrEmpty = *role
	}
	if regions, ok := p.polls[account]; ok {
		if roles, ok := regions[region]; ok {
			if clusters, ok := roles[roleOrEmpty]; ok {
				if services, ok := clusters[cluster]; ok {
					if record, ok := services[service]; ok {
						if record.count > 0 {
							record.count = record.count - 1
						} else {
							delete(services, service)
							if len(services) == 0 {
								delete(clusters, cluster)
							}
						}
					}
				}
				if len(clusters) == 0 {
					delete(roles, roleOrEmpty)
				}
			}
			if len(roles) == 0 {
				delete(regions, region)
			}
		}
		if len(regions) == 0 {
			delete(p.polls, account)
		}
	}
	p.c.Broadcast()
}

func (p EcsServiceDescriptionPoller) Latest(account string, region string, role *string, cluster string, service string) *PollService {
	p.m.RLock()
	defer p.m.RUnlock()
	roleOrEmpty := ""
	if role != nil {
		roleOrEmpty = *role
	}
	if regions, ok := p.polls[account]; ok {
		if roles, ok := regions[region]; ok {
			if clusters, ok := roles[roleOrEmpty]; ok {
				if services, ok := clusters[cluster]; ok {
					if record, ok := services[service]; ok {
						return record.value
					}
				}
			}
		}
	}
	return nil
}

func (p EcsServiceDescriptionPoller) AwaitLatest(account string, region string, role *string, cluster string, service string) *PollService {
	p.m.Lock()
	defer p.m.Unlock()
	roleOrEmpty := ""
	if role != nil {
		roleOrEmpty = *role
	}
	for {
		regions, ok := p.polls[account]
		if !ok {
			return nil
		}
		roles, ok := regions[region]
		if !ok {
			return nil
		}
		clusters, ok := roles[roleOrEmpty]
		if !ok {
			return nil
		}
		services, ok := clusters[cluster]
		if !ok {
			return nil
		}
		record := services[service]
		if record != nil && record.value != nil {
			return record.value
		}
		p.c.Wait()
	}
}

func (p EcsServiceDescriptionPoller) pollAll(ctx context.Context) {
	p.m.Lock()
	defer p.m.Unlock()
	startTime := time.Now()

	for account, regions := range p.polls {
		for region, roles := range regions {
			for roleOrEmpty, clusters := range roles {
				var role *string
				if roleOrEmpty != "" {
					role = &roleOrEmpty
				}
				client, err := p.apiClientProvider(account, region, role)
				if err != nil {
					log.Warn().TimeDiff("duration", time.Now(), startTime).Err(err).Msg("could not create api client")
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
							log.Warn().TimeDiff("duration", time.Now(), startTime).Err(err).Msg("api call failed")
							continue
						}

						for _, service := range descriptions.Services {
							services[aws.ToString(service.ServiceArn)].value = &PollService{
								service: &service,
							}
						}
						for _, failure := range descriptions.Failures {
							services[aws.ToString(failure.Arn)].value = &PollService{
								failure: &failure,
							}
						}
					}
				}
			}
		}
	}
	p.c.Broadcast()
}

func defaultDescribeServiceProvider(account string, region string, role *string) (ecs.DescribeServicesAPIClient, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return ecs.NewFromConfig(awsAccess.AwsConfig), nil
}
