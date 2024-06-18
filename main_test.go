package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
	"net/http"
	"testing"
)

func createConfig(ec2 bool, ecs bool, elb bool, fis bool, lambda bool, rds bool, zone bool) config.Specification {
	return config.Specification{
		EnrichEc2DataForTargetTypes: []string{"com.steadybit.extension_aws.test"},
		DiscoveryDisabledEc2:        ec2,
		DiscoveryIntervalEc2:        10,
		DiscoveryDisabledEcs:        ecs,
		DiscoveryIntervalEcsTask:    10,
		DiscoveryIntervalEcsService: 10,
		DiscoveryDisabledElb:        elb,
		DiscoveryIntervalElbAlb:     10,
		DiscoveryDisabledFis:        fis,
		DiscoveryIntervalFis:        10,
		DiscoveryDisabledLambda:     lambda,
		DiscoveryIntervalLambda:     10,
		DiscoveryDisabledRds:        rds,
		DiscoveryIntervalRds:        10,
		DiscoveryDisabledZone:       zone,
		DiscoveryIntervalZone:       10,
	}
}

func Test_getExtensionList(t *testing.T) {
	tests := []struct {
		name         string
		config       config.Specification
		wantedRoutes []string
	}{
		{
			name:   "disabled all but ec2",
			config: createConfig(false, true, true, true, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.ec2_instance.state",
				"/com.steadybit.extension_aws.ec2-instance/discovery",
				"/com.steadybit.extension_aws.ec2-instance/discovery/target-description",
				"/discovery/attributes",
				"/discovery/enrichment-rules/com.steadybit.extension_aws.ec2-instance-to-com.steadybit.extension_aws.test",
				"/discovery/enrichment-rules/com.steadybit.extension_aws.ec2-instance-to-com.steadybit.extension_host.host",
				"/discovery/enrichment-rules/com.steadybit.extension_aws.ec2-instance-to-com.steadybit.extension_kubernetes.kubernetes-node",
			},
		},
		{
			name:   "disabled all but ecs",
			config: createConfig(true, false, true, true, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.ecs-task.stop",
				"/com.steadybit.extension_aws.ecs-service.scale",
				"/com.steadybit.extension_aws.ecs-service.task_count_check",
				"/com.steadybit.extension_aws.ecs-task.fill_disk",
				"/com.steadybit.extension_aws.ecs-task.stress_cpu",
				"/com.steadybit.extension_aws.ecs-task.stress_mem",
				"/com.steadybit.extension_aws.ecs-task.stress_io",
				"/com.steadybit.extension_aws.ecs-task/discovery",
				"/com.steadybit.extension_aws.ecs-task/discovery/target-description",
				"/com.steadybit.extension_aws.ecs-service/discovery",
				"/com.steadybit.extension_aws.ecs-service/discovery/target-description",
				"/discovery/attributes",
			},
		},
		{
			name:   "disabled all but elb",
			config: createConfig(true, true, false, true, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.alb/discovery",
				"/com.steadybit.extension_aws.alb/discovery/target-description",
				"/discovery/attributes",
			},
		},
		{
			name:   "disabled all but fis",
			config: createConfig(true, true, true, false, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.fis.start_experiment",
				"/com.steadybit.extension_aws.fis-experiment-template/discovery",
				"/com.steadybit.extension_aws.fis-experiment-template/discovery/target-description",
				"/discovery/attributes",
			},
		},
		{
			name:   "disabled all but lambda",
			config: createConfig(true, true, true, true, false, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.lambda.denylist",
				"/com.steadybit.extension_aws.lambda.diskspace",
				"/com.steadybit.extension_aws.lambda.exception",
				"/com.steadybit.extension_aws.lambda.latency",
				"/com.steadybit.extension_aws.lambda.statusCode",
				"/com.steadybit.extension_aws.lambda/discovery",
				"/com.steadybit.extension_aws.lambda/discovery/target-description",
				"/discovery/attributes",
			},
		},
		{
			name:   "disabled all but rds",
			config: createConfig(true, true, true, true, true, false, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.rds.cluster.failover",
				"/com.steadybit.extension_aws.rds.instance.reboot",
				"/com.steadybit.extension_aws.rds.instance.stop",
				"/com.steadybit.extension_aws.rds.cluster/discovery",
				"/com.steadybit.extension_aws.rds.cluster/discovery/target-description",
				"/com.steadybit.extension_aws.rds.instance/discovery",
				"/com.steadybit.extension_aws.rds.instance/discovery/target-description",
				"/discovery/attributes",
			},
		},
		{
			name:   "disabled all but zone",
			config: createConfig(true, true, true, true, true, true, false),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.az.blackhole",
				"/com.steadybit.extension_aws.zone/discovery",
				"/com.steadybit.extension_aws.zone/discovery/target-description",
				"/discovery/attributes",
			},
		},
	}
	utils.Accounts = &utils.AwsAccounts{
		RootAccount: utils.AwsAccount{
			AccountNumber: "123456789012",
			AwsConfig:     aws.Config{},
		},
		Accounts: make(map[string]utils.AwsAccount),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action_kit_sdk.ClearRegisteredActions()
			discovery_kit_sdk.ClearRegisteredDiscoveries()
			http.DefaultServeMux = http.NewServeMux()
			config.Config = tt.config
			background, cancel := context.WithCancel(context.Background())
			defer cancel()
			registerHandlers(background)

			got := getExtensionList()
			routes := make([]string, 0)
			for _, route := range got.Actions {
				routes = append(routes, route.Path)
			}
			for _, route := range got.Discoveries {
				routes = append(routes, route.Path)
			}
			for _, route := range got.TargetTypes {
				routes = append(routes, route.Path)
			}
			for _, route := range got.TargetAttributes {
				routes = append(routes, route.Path)
			}
			for _, route := range got.TargetEnrichmentRules {
				routes = append(routes, route.Path)
			}
			slices.Sort(routes)
			slices.Sort(tt.wantedRoutes)

			assert.Equal(t, tt.wantedRoutes, routes)
		})
	}
}
