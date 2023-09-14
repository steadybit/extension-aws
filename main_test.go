package main

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"golang.org/x/exp/slices"
	"net/http"
	"os"
	"reflect"
	"testing"
)

func createConfig(ec2 bool, fis bool, lambda bool, rds bool, zone bool) config.Specification {
	return config.Specification{
		EnrichEc2DataForTargetTypes: []string{"com.steadybit.extension_aws.test"},
		DiscoveryDisabledEc2:        ec2,
		DiscoveryIntervalEc2:        10,
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
			name:   "disabledAllButEc2",
			config: createConfig(false, true, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.ec2_instance.state",
				"/common/discovery/attribute-descriptions",
				"/ec2/instance/discovery",
				"/ec2/instance/discovery/attribute-descriptions",
				"/ec2/instance/discovery/rules/ec2-to-com.steadybit.extension_aws.test",
				"/ec2/instance/discovery/rules/ec2-to-host",
				"/ec2/instance/discovery/target-description",
			},
		},
		{
			name:   "disabledAllButFis",
			config: createConfig(true, false, true, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.fis.start_experiment",
				"/common/discovery/attribute-descriptions",
				"/fis/template/discovery",
				"/fis/template/discovery/attribute-descriptions",
				"/fis/template/discovery/target-description",
			},
		},
		{
			name:   "disabledAllButLambda",
			config: createConfig(true, true, false, true, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.lambda.denylist",
				"/com.steadybit.extension_aws.lambda.diskspace",
				"/com.steadybit.extension_aws.lambda.exception",
				"/com.steadybit.extension_aws.lambda.latency",
				"/com.steadybit.extension_aws.lambda.statusCode",
				"/common/discovery/attribute-descriptions",
				"/lambda/discovery",
				"/lambda/discovery/attribute-descriptions",
				"/lambda/discovery/target-description",
			},
		},
		{
			name:   "disabledAllButRds",
			config: createConfig(true, true, true, false, true),
			wantedRoutes: []string{
				"/com.steadybit.extension_aws.rds.cluster.failover",
				"/com.steadybit.extension_aws.rds.instance.reboot",
				"/com.steadybit.extension_aws.rds.instance.stop",
				"/common/discovery/attribute-descriptions",
				"/rds/cluster/discovery",
				"/rds/cluster/discovery/attribute-descriptions",
				"/rds/cluster/discovery/target-description",
				"/rds/instance/discovery",
				"/rds/instance/discovery/attribute-descriptions",
				"/rds/instance/discovery/target-description",
			},
		},
		{
			name:   "disabledAllButZone",
			config: createConfig(true, true, true, true, false),
			wantedRoutes: []string{
				"/az/discovery",
				"/az/discovery/target-description",
				"/com.steadybit.extension_aws.az.blackhole",
				"/common/discovery/attribute-descriptions",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils.Accounts = &utils.AwsAccounts{
				RootAccount: utils.AwsAccount{
					AccountNumber: "123456789012",
					AwsConfig:     aws.Config{},
				},
				Accounts: make(map[string]utils.AwsAccount),
			}
			action_kit_sdk.ClearRegisteredActions()
			http.DefaultServeMux = http.NewServeMux()
			config.Config = tt.config
			stopCh := make(chan os.Signal)
			registerHandlers(stopCh)

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
			stopCh <- os.Interrupt

			if !reflect.DeepEqual(routes, tt.wantedRoutes) {
				t.Errorf("routes = %v, want %v", routes, tt.wantedRoutes)
			}
		})
	}
}
