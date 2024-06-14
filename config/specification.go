// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

type Specification struct {
	AssumeRoles                       []string `json:"assumeRoles" split_words:"true" required:"false"`
	WorkerThreads                     int      `json:"workerThreads" split_words:"true" required:"false" default:"1"`
	AwsEndpointOverride               string   `json:"awsEndpointOverride" split_words:"true" required:"false"`
	DiscoveryDisabledEc2              bool     `json:"discoveryDisabledEc2" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledEcs              bool     `json:"discoveryDisabledEcs" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledFis              bool     `json:"discoveryDisabledFis" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledLambda           bool     `json:"discoveryDisabledLambda" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledRds              bool     `json:"discoveryDisabledRds" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledZone             bool     `json:"discoveryDisabledZone" split_words:"true" required:"false" default:"false"`
	DiscoveryIntervalEc2              int      `json:"discoveryIntervalEc2" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalEcsService       int      `json:"discoveryIntervalEcsService" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalEcsTask          int      `json:"discoveryIntervalEcsTask" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalFis              int      `json:"discoveryIntervalFis" split_words:"true" required:"false" default:"300"`
	DiscoveryIntervalLambda           int      `json:"discoveryIntervalLambda" split_words:"true" required:"false" default:"60"`
	DiscoveryIntervalRds              int      `json:"discoveryIntervalRds" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalZone             int      `json:"discoveryIntervalZone" split_words:"true" required:"false" default:"300"`
	EnrichEc2DataForTargetTypes       []string `json:"EnrichEc2DataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.kubernetes-deployment,com.steadybit.extension_kubernetes.kubernetes-pod,com.steadybit.extension_kubernetes.kubernetes-daemonset,com.steadybit.extension_kubernetes.kubernetes-statefulset"`
	EnrichEc2DataMatcherAttribute     string   `json:"EnrichEc2DataMatcherAttribute" split_words:"true" default:"host.hostname"`
	DiscoveryAttributesExcludesEc2    []string `json:"discoveryAttributesExcludesEc2" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesEcs    []string `json:"discoveryAttributesExcludesEcs" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesFis    []string `json:"discoveryAttributesExcludesFis" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesLambda []string `json:"discoveryAttributesExcludesLambda" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesRds    []string `json:"discoveryAttributesExcludesRds" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesZone   []string `json:"discoveryAttributesExcludesZone" split_words:"true" required:"false"`
}
