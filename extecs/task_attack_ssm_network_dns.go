// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extutil"
	"strconv"
	"time"
)

func NewEcsTaskNetworkDnsAction() action_kit_sdk.Action[TaskSsmActionState] {
	var heartbeatParameters = updateFisActionStateParameter
	return newEcsTaskSsmAction(getEcsTaskNetworkDnsDescription, ssmCommandInvocation{
		documentName:              "AWSFIS-Run-Network-Blackhole-Port-ECS",
		documentVersion:           "$DEFAULT",
		stepNameToOutput:          "FaultInjection",
		getParameters:             getEcsTaskNetworkDnsParameters,
		updateHeartbeatParameters: &heartbeatParameters,
	})
}

func getEcsTaskNetworkDnsDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_dns", ecsTaskTargetId),
		Label:       "Block DNS",
		Description: "Block access to DNS servers",
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("Duration of the attack."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("30s"),
				Required:     new(true),
				Order:        new(0),
			},
			{
				Name:         "dnsPort",
				Label:        "DNS Port",
				Description:  new("Port number used for DNS queries (typically 53)"),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("53"),
				Required:     new(true),
				Order:        new(1),
				MinValue:     new(1),
				MaxValue:     new(65534),
			},
		},
	}
}

func getEcsTaskNetworkDnsParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
	if duration.Seconds() > 43200 {
		return nil, fmt.Errorf("duration longer than 43200 seconds is not supported")
	}
	fisActionStateParameter := newFisActionStateParameter(request.ExecutionId.String())
	return map[string][]string{
		"DurationSeconds":           {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"Protocol":                  {"udp"},
		"Port":                      {strconv.Itoa(extutil.ToInt(request.Config["dnsPort"]))},
		"TrafficType":               {"egress"},
		"InstallDependencies":       {"True"},
		fisActionStateParameterName: {fisActionStateParameter},
	}, nil
}
