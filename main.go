// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
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
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()
	extruntime.LogRuntimeInformation(zerolog.DebugLevel)
	exthealth.StartProbes(8086)
	exthealth.SetReady(false)

	config.ParseConfiguration()
	utils.InitializeAwsAccountAccess(config.Config)

	utils.RegisterCommonDiscoveryHandlers()

	extrds.RegisterInstanceDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extrds.NewRdsInstanceRebootAttack())
	action_kit_sdk.RegisterAction(extrds.NewRdsInstanceStopAttack())

	extrds.RegisterClusterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extrds.NewRdsClusterFailoverAttack())

	extaz.RegisterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extaz.NewAzBlackholeAction())

	extec2.RegisterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extec2.NewEc2InstanceStateAction())

	extfis.RegisterFisInstanceDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extfis.NewFisExperimentAction())

	extlambda.RegisterDiscoveryHandlers()
	action_kit_sdk.RegisterAction(extlambda.NewInjectStatusCodeAction())
	action_kit_sdk.RegisterAction(extlambda.NewInjectExceptionAction())
	action_kit_sdk.RegisterAction(extlambda.NewInjectLatencyAction())
	action_kit_sdk.RegisterAction(extlambda.NewFillDiskspaceAction())
	action_kit_sdk.RegisterAction(extlambda.NewDenylistAction())

	action_kit_sdk.InstallSignalHandler()
	exthealth.SetReady(true)

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

	return ExtensionListResponse{
		ActionList: action_kit_sdk.GetActionList(),
		DiscoveryList: discovery_kit_api.DiscoveryList{
			Discoveries: discoveries,
			TargetTypes: []discovery_kit_api.DescribingEndpointReference{
				{
					Method: "GET",
					Path:   "/rds/instance/discovery/target-description",
				},
				{
					Method: "GET",
					Path:   "/rds/cluster/discovery/target-description",
				},
				{
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
					Path:   "/rds/cluster/discovery/attribute-descriptions",
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
