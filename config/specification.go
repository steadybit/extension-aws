// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import (
	"encoding/json"
)

type Specification struct {
	AssumeRoles                                  []string    `json:"assumeRoles" split_words:"true" required:"false"`
	Regions                                      []string    `json:"regions" split_words:"true" required:"false"`             // If specified `Regions` will be applied for each role specified by `AssumeRoles`
	TagFilters                                   TagFilters  `json:"tagFilters" split_words:"true" required:"false"`          // If specified `TagFilters` will be applied for each role specified by `AssumeRoles`
	AssumeRolesAdvanced                          AssumeRoles `json:"assumeRolesAdvanced" split_words:"true" required:"false"` // If you need a more fine-grained approach and want to specify Regions/TagFilters per role.
	WorkerThreads                                int         `json:"workerThreads" split_words:"true" required:"false" default:"1"`
	AwsEndpointOverride                          string      `json:"awsEndpointOverride" split_words:"true" required:"false"`
	DiscoveryDisabledEc2                         bool        `json:"discoveryDisabledEc2" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledEcs                         bool        `json:"discoveryDisabledEcs" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledElasticache                 bool        `json:"discoveryDisabledElasticache" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledElb                         bool        `json:"discoveryDisabledElb" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledFis                         bool        `json:"discoveryDisabledFis" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledMsk                         bool        `json:"discoveryDisabledMsk" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledLambda                      bool        `json:"discoveryDisabledLambda" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledRds                         bool        `json:"discoveryDisabledRds" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledSubnet                      bool        `json:"discoveryDisabledSubnet" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledZone                        bool        `json:"discoveryDisabledZone" split_words:"true" required:"false" default:"false"`
	DiscoveryDisabledVpc                         bool        `json:"discoveryDisabledVpc" split_words:"true" required:"false" default:"false"`
	DiscoveryIntervalEc2                         int         `json:"discoveryIntervalEc2" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalEcsService                  int         `json:"discoveryIntervalEcsService" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalEcsTask                     int         `json:"discoveryIntervalEcsTask" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalElasticacheReplicationGroup int         `json:"discoveryIntervalElasticacheReplicationGroup" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalElbAlb                      int         `json:"discoveryIntervalElbAlb" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalMsk                         int         `json:"discoveryIntervalMsk" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalFis                         int         `json:"discoveryIntervalFis" split_words:"true" required:"false" default:"300"`
	DiscoveryIntervalLambda                      int         `json:"discoveryIntervalLambda" split_words:"true" required:"false" default:"60"`
	DiscoveryIntervalRds                         int         `json:"discoveryIntervalRds" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalSubnet                      int         `json:"discoveryIntervalSubnet" split_words:"true" required:"false" default:"30"`
	DiscoveryIntervalZone                        int         `json:"discoveryIntervalZone" split_words:"true" required:"false" default:"300"`
	EnrichEc2DataForTargetTypes                  []string    `json:"EnrichEc2DataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.kubernetes-deployment,com.steadybit.extension_kubernetes.kubernetes-pod,com.steadybit.extension_kubernetes.kubernetes-daemonset,com.steadybit.extension_kubernetes.kubernetes-statefulset,com.steadybit.extension_http.client-location,com.steadybit.extension_jmeter.location,com.steadybit.extension_k6.location,com.steadybit.extension_gatling.location"`
	EnrichEc2DataMatcherAttribute                string      `json:"EnrichEc2DataMatcherAttribute" split_words:"true" default:"host.hostname"`
	DiscoveryAttributesExcludesElb               []string    `json:"discoveryAttributesExcludesElb" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesEc2               []string    `json:"discoveryAttributesExcludesEc2" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesEcs               []string    `json:"discoveryAttributesExcludesEcs" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesElasticache       []string    `json:"discoveryAttributesExcludesElasticache" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesFis               []string    `json:"discoveryAttributesExcludesFis" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesMsk               []string    `json:"discoveryAttributesExcludesMsk" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesLambda            []string    `json:"discoveryAttributesExcludesLambda" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesRds               []string    `json:"discoveryAttributesExcludesRds" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesSubnet            []string    `json:"discoveryAttributesExcludesSubnet" split_words:"true" required:"false"`
	DiscoveryAttributesExcludesZone              []string    `json:"discoveryAttributesExcludesZone" split_words:"true" required:"false"`
	DisableDiscoveryExcludes                     bool        `required:"false" split_words:"true" default:"false"`
}

type AssumeRoles []AssumeRole
type AssumeRole struct {
	RoleArn    string      `json:"roleArn"`
	Regions    []string    `json:"regions"`
	TagFilters []TagFilter `json:"tagFilters"`
}

func (j *AssumeRoles) UnmarshalText(text []byte) error {
	if len(text) == 0 || string(text) == "[]" {
		*j = AssumeRoles{}
		return nil
	}
	return json.Unmarshal(text, (*[]AssumeRole)(j))
}

type TagFilters []TagFilter
type TagFilter struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (j *TagFilters) UnmarshalText(text []byte) error {
	if len(text) == 0 || string(text) == "[]" {
		*j = TagFilters{}
		return nil
	}
	return json.Unmarshal(text, (*[]TagFilter)(j))
}
