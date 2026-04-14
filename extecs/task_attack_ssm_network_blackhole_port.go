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

func NewEcsTaskNetworkBlockholePortAction() action_kit_sdk.Action[TaskSsmActionState] {
	var heartbeatParameters = updateFisActionStateParameter
	return newEcsTaskSsmAction(getEcsTaskNetworkBlackholePortDescription, ssmCommandInvocation{
		documentName:              "AWSFIS-Run-Network-Blackhole-Port-ECS",
		documentVersion:           "$DEFAULT",
		stepNameToOutput:          "FaultInjection",
		getParameters:             getEcsTaskNetworkBlackholePortParameters,
		updateHeartbeatParameters: &heartbeatParameters,
	})
}

func getEcsTaskNetworkBlackholePortDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_blackhole_port", ecsTaskTargetId),
		Label:       "Block Traffic",
		Description: "Drop inbound or outbound traffic for the specified protocol and port.",
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
				Name:         "protocol",
				Label:        "Protocol",
				Description:  new("The affected protocol."),
				Type:         action_kit_api.ActionParameterTypeString,
				Required:     new(true),
				Order:        new(1),
				DefaultValue: new("tcp"),
				Options: new([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "TCP",
						Value: "tcp",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "UDP",
						Value: "udp",
					},
				}),
			},
			{
				Name:         "port",
				Label:        "Port",
				Description:  new("The affected port."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("80"),
				Required:     new(true),
				Order:        new(2),
			},
			{
				Name:         "trafficType",
				Label:        "Traffic Type",
				Description:  new("The affected traffic type."),
				Type:         action_kit_api.ActionParameterTypeString,
				Required:     new(true),
				Order:        new(3),
				DefaultValue: new("ingress"),
				Options: new([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Ingress",
						Value: "ingress",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Egress",
						Value: "egress",
					},
				}),
			},
		},
	}
}

func getEcsTaskNetworkBlackholePortParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
	if duration.Seconds() > 43200 {
		return nil, fmt.Errorf("duration longer than 43200 seconds is not supported")
	}
	fisActionStateParameter := newFisActionStateParameter(request.ExecutionId.String())
	return map[string][]string{
		"DurationSeconds":           {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"Protocol":                  {extutil.ToString(request.Config["protocol"])},
		"Port":                      {strconv.Itoa(extutil.ToInt(request.Config["port"]))},
		"TrafficType":               {extutil.ToString(request.Config["trafficType"])},
		"InstallDependencies":       {"True"},
		fisActionStateParameterName: {fisActionStateParameter},
	}, nil
}
