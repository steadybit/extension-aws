// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
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
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()

	config.ParseConfiguration()
	utils.InitializeAwsAccountAccess(config.Config)

	utils.RegisterCommonDiscoveryHandlers()

	extrds.RegisterRdsDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extrds.NewRdsInstanceAttack())

	extaz.RegisterAZDiscoveryHandlers()
	extaz.RegisterAZAttackHandlers()

	extec2.RegisterEc2InstanceDiscoveryHandlers()
	extec2.RegisterEc2AttackHandlers()

	extfis.RegisterFisInstanceDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extfis.NewFisExperimentAction())

	extlambda.RegisterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extlambda.NewInjectStatusCodeAction())
	action_kit_sdk.RegisterAction(extlambda.NewInjectExceptionAction())
	action_kit_sdk.RegisterAction(extlambda.NewInjectLatencyAction())
	action_kit_sdk.RegisterAction(extlambda.NewFillDiskspaceAction())
	action_kit_sdk.RegisterAction(extlambda.NewDenylistAction())

	exthealth.StartProbes(8086)

	stop := action_kit_sdk.Start()
	defer stop()

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
	exthttp.Listen(exthttp.ListenOpts{
		Port: 8085,
	})
}

type ExtensionListResponse struct {
	action_kit_api.ActionList
	discovery_kit_api.DiscoveryList
}

func getExtensionList() ExtensionListResponse {
	cfg := config.Config
	discoveries := make([]discovery_kit_api.DescribingEndpointReference, 0)
	if !cfg.DiscoveryDisabledRds {
		discoveries = append(discoveries, discovery_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/rds/instance/discovery",
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
	actionList := action_kit_sdk.GetActionList()
	actionList.Actions = append(actionList.Actions,
		action_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/ec2/instance/attack/state",
		},
		action_kit_api.DescribingEndpointReference{
			Method: "GET",
			Path:   "/az/attack/blackhole",
		},
	)

	return ExtensionListResponse{
		ActionList: actionList,
		DiscoveryList: discovery_kit_api.DiscoveryList{
			Discoveries: discoveries,
			TargetTypes: []discovery_kit_api.DescribingEndpointReference{
				{
					Method: "GET",
					Path:   "/rds/instance/discovery/target-description",
				}, {
					Method: "GET",
					Path:   "/az/discovery/target-description",
				},
				{
					Method: "GET",
					Path:   "/ec2/instance/discovery/target-description",
				},
				{
					Method: "GET",
					Path:   "/fis/template/discovery/target-description",
				},
				{
					Method: "GET",
					Path:   "/lambda/discovery/target-description",
				},
			},
			TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
				{
					Method: "GET",
					Path:   "/rds/instance/discovery/attribute-descriptions",
				},
				{
					Method: "GET",
					Path:   "/ec2/instance/discovery/attribute-descriptions",
				},
				{
					Method: "GET",
					Path:   "/fis/template/discovery/attribute-descriptions",
				},
				{
					Method: "GET",
					Path:   "/lambda/discovery/attribute-descriptions",
				},
				{
					Method: "GET",
					Path:   "/common/discovery/attribute-descriptions",
				},
			},
		},
	}
}
