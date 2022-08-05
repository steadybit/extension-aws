// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/extec2"
	"github.com/steadybit/extension-aws/extrds"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extlogging"
	"net/http"
)

func main() {
	extlogging.InitZeroLog()

	utils.InitializeAwsAccountAccess()

	exthttp.RegisterHttpHandler("/", exthttp.GetterAsHandler(getExtensionList))
	utils.RegisterCommonDiscoveryHandlers()

	extrds.RegisterRdsDiscoveryHandlers()
	extrds.RegisterRdsAttackHandlers()

	extec2.RegisterEc2AttackHandlers()

	port := 8085
	log.Info().Msgf("Starting extension-aws server on port %d. Get started via /", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to start extension-aws server on port %d", port)
	}
}

type ExtensionListResponse struct {
	Attacks          []attack_kit_api.DescribingEndpointReference    `json:"attacks"`
	Discoveries      []discovery_kit_api.DescribingEndpointReference `json:"discoveries"`
	TargetTypes      []discovery_kit_api.DescribingEndpointReference `json:"targetTypes"`
	TargetAttributes []discovery_kit_api.DescribingEndpointReference `json:"targetAttributes"`
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		Attacks: []attack_kit_api.DescribingEndpointReference{
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
