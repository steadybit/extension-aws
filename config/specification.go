// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

type Specification struct {
	AssumeRoles                 []string `json:"assumeRoles" split_words:"true" required:"false"`
	AwsEndpointOverride         string   `json:"awsEndpointOverride" split_words:"true" required:"false"`
	DiscoveryDisabledEc2        bool     `json:"discoveryDisabledEc2" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledRds        bool     `json:"discoveryDisabledRds" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledZone       bool     `json:"discoveryDisabledZone" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledFis        bool     `json:"discoveryDisabledFis" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledLambda     bool     `json:"discoveryDisabledLambda" split_words:"true" required:"false" default:"false"`
	EnrichEc2DataForTargetTypes []string `json:"EnrichEc2DataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.application,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.kubernetes-deployment"`
}
