// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/extrds"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func main() {
	utils.InitializeAwsAccountAccess()

	utils.RegisterHttpHandler("/", utils.GetterAsHandler(getExtensionList))
	utils.RegisterCommonDiscoveryHandlers()

	extrds.RegisterRdsDiscoveryHandlers()
	extrds.RegisterRdsAttackHandlers()

	port := 8085
	InfoLogger.Printf("Starting extension-aws server on port %d. Get started via /\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
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
