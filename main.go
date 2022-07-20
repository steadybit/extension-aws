// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package main

import (
	"fmt"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/discovery"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func main() {
	utils.RegisterHttpHandler("/", utils.GetterAsHandler(getExtensionList))

	discovery.RegisterCommonDiscoveryHandlers()
	discovery.RegisterRdsDiscoveryHandlers()

	port := 8085
	InfoLogger.Printf("Starting extension-aws server on port %d. Get started via /\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

type ExtensionListResponse struct {
	Discoveries      []discovery_kit_api.DescribingEndpointReference `json:"discoveries"`
	TargetTypes      []discovery_kit_api.DescribingEndpointReference `json:"targetTypes"`
	TargetAttributes []discovery_kit_api.DescribingEndpointReference `json:"targetAttributes"`
}

func getExtensionList() ExtensionListResponse {
	return ExtensionListResponse{
		Discoveries: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/discovery/rds",
			},
		},
		TargetTypes: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/discovery/rds/target-description",
			},
		},
		TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
			{
				"GET",
				"/discovery/rds/attribute-descriptions",
			},
			{
				"GET",
				"/discovery/common/attribute-descriptions",
			},
		},
	}
}
