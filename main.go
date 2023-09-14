// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/extaz"
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/extfis"
	"github.com/steadybit/extension-aws/extlambda"
	"github.com/steadybit/extension-aws/extrds"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthealth"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"github.com/steadybit/extension-kit/extruntime"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)
	exthealth.StartProbes(8086)
	exthealth.SetReady(false)

	config.ParseConfiguration()
	utils.InitializeAwsAccountAccess(config.Config)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	registerHandlers(stopCh)

	action_kit_sdk.InstallSignalHandler()
	action_kit_sdk.RegisterCoverageEndpoints()
	exthealth.SetReady(true)

	exthttp.Listen(exthttp.ListenOpts{
		Port: 8085,
	})
}

func registerHandlers(stopCh chan os.Signal) {
	cfg := config.Config
	utils.RegisterCommonDiscoveryHandlers()
	if !cfg.DiscoveryDisabledRds {
		extrds.RegisterInstanceDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extrds.NewRdsInstanceRebootAttack())
		action_kit_sdk.RegisterAction(extrds.NewRdsInstanceStopAttack())

		extrds.RegisterClusterDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extrds.NewRdsClusterFailoverAttack())
	}

	if !cfg.DiscoveryDisabledZone {
		extaz.RegisterDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extaz.NewAzBlackholeAction())
	}

	if !cfg.DiscoveryDisabledEc2 {
		extec2.RegisterDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extec2.NewEc2InstanceStateAction())
	}

	if !cfg.DiscoveryDisabledFis {
		extfis.RegisterFisInstanceDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extfis.NewFisExperimentAction())
	}

	if !cfg.DiscoveryDisabledLambda {
		extlambda.RegisterDiscoveryHandlers(stopCh)
		action_kit_sdk.RegisterAction(extlambda.NewInjectStatusCodeAction())
		action_kit_sdk.RegisterAction(extlambda.NewInjectExceptionAction())
		action_kit_sdk.RegisterAction(extlambda.NewInjectLatencyAction())
		action_kit_sdk.RegisterAction(extlambda.NewFillDiskspaceAction())
		action_kit_sdk.RegisterAction(extlambda.NewDenylistAction())
	}

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
}

type ExtensionListResponse struct {
	action_kit_api.ActionList
	discovery_kit_api.DiscoveryList
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		ActionList: action_kit_sdk.GetActionList(),
		DiscoveryList: discovery_kit_api.DiscoveryList{
			Discoveries:           getDiscoveries(),
			TargetTypes:           getTargetTypes(),
			TargetAttributes:      getTargetAttributes(),
			TargetEnrichmentRules: getEnrichmentRules(),
		},
	}
}

func getDiscoveries() []discovery_kit_api.DescribingEndpointReference {
	cfg := config.Config
	discoveries := make([]discovery_kit_api.DescribingEndpointReference, 0)
	if !cfg.DiscoveryDisabledRds {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/instance/discovery",
		})
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/cluster/discovery",
		})
	}
	if !cfg.DiscoveryDisabledEc2 {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/ec2/instance/discovery",
		})
	}
	if !cfg.DiscoveryDisabledZone {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/az/discovery",
		})
	}
	if !cfg.DiscoveryDisabledFis {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/fis/template/discovery",
		})
	}
	if !cfg.DiscoveryDisabledLambda {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/lambda/discovery",
		})
	}
	return discoveries
}

func getTargetTypes() []discovery_kit_api.DescribingEndpointReference {
	cfg := config.Config
	targetTypes := make([]discovery_kit_api.DescribingEndpointReference, 0)
	if !cfg.DiscoveryDisabledRds {
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/instance/discovery/target-description",
		})
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/cluster/discovery/target-description",
		})
	}
	if !cfg.DiscoveryDisabledEc2 {
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/ec2/instance/discovery/target-description",
		})
	}
	if !cfg.DiscoveryDisabledZone {
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/az/discovery/target-description",
		})
	}
	if !cfg.DiscoveryDisabledFis {
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/fis/template/discovery/target-description",
		})
	}
	if !cfg.DiscoveryDisabledLambda {
		targetTypes = append(targetTypes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/lambda/discovery/target-description",
		})
	}
	return targetTypes
}

func getTargetAttributes() []discovery_kit_api.DescribingEndpointReference {
	cfg := config.Config
	targetAttributes := make([]discovery_kit_api.DescribingEndpointReference, 0)
	targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
		Method: "GET",
		Path:   "/common/discovery/attribute-descriptions",
	})
	if !cfg.DiscoveryDisabledRds {
		targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/instance/discovery/attribute-descriptions",
		})
		targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/cluster/discovery/attribute-descriptions",
		})
	}
	if !cfg.DiscoveryDisabledEc2 {
		targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/ec2/instance/discovery/attribute-descriptions",
		})
	}
	if !cfg.DiscoveryDisabledFis {
		targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/fis/template/discovery/attribute-descriptions",
		})
	}
	if !cfg.DiscoveryDisabledLambda {
		targetAttributes = append(targetAttributes, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/lambda/discovery/attribute-descriptions",
		})
	}
	return targetAttributes
}

func getEnrichmentRules() []discovery_kit_api.DescribingEndpointReference {
	cfg := config.Config
	targetEnrichmentRules := make([]discovery_kit_api.DescribingEndpointReference, 0)
	if !cfg.DiscoveryDisabledEc2 {
		targetEnrichmentRules = append(targetEnrichmentRules, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/ec2/instance/discovery/rules/ec2-to-host",
		})
		for _, targetType := range config.Config.EnrichEc2DataForTargetTypes {
			targetEnrichmentRules = append(targetEnrichmentRules, discovery_kit_api.DescribingEndpointReference{
				Method: "GET",
				Path:   fmt.Sprintf("/ec2/instance/discovery/rules/ec2-to-%s", targetType),
			})
		}
	}
	return targetEnrichmentRules
}
