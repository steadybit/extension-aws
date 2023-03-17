// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package main

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/extrds"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
)

func main() {
	extlogging.InitZeroLog()
	extbuild.PrintBuildInformation()

	config.ParseConfiguration()
	utils.InitializeAwsAccountAccess(config.Config)

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
	utils.RegisterCommonDiscoveryHandlers()

	extrds.RegisterRdsDiscoveryHandlers()
	extrds.RegisterRdsAttackHandlers()

	extec2.RegisterEc2AttackHandlers()

	exthttp.Listen(exthttp.ListenOpts{
		Port: 8085,
	})
}

type ExtensionListResponse struct {
	Attacks          []action_kit_api.DescribingEndpointReference    `json:"attacks"`
	Discoveries      []discovery_kit_api.DescribingEndpointReference `json:"discoveries"`
	TargetTypes      []discovery_kit_api.DescribingEndpointReference `json:"targetTypes"`
	TargetAttributes []discovery_kit_api.DescribingEndpointReference `json:"targetAttributes"`
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		Attacks: []action_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/rds/instance/attack/reboot",
			},
			{
				"GET",
				"/ec2/instance/attack/state",
			},
		},
		Discoveries: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/rds/instance/discovery",
			},
		},
		TargetTypes: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/rds/instance/discovery/target-description",
			},
		},
		TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/rds/instance/discovery/attribute-descriptions",
			},
			{
				"GET",
				"/common/discovery/attribute-descriptions",
			},
		},
	}
}
