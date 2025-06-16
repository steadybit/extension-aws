// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
	"time"
)

func NewEcsTaskNetworkDelayAction() action_kit_sdk.Action[TaskSsmActionState] {
	var heartbeatParameters = updateFisActionStateParameter
	return newEcsTaskSsmAction(getEcsTaskNetworkDelayDescription, ssmCommandInvocation{
		documentName:              "AWSFIS-Run-Network-Latency-ECS",
		documentVersion:           "$DEFAULT",
		stepNameToOutput:          "FaultInjection",
		getParameters:             getEcsTaskNetworkDelayParameters,
		updateHeartbeatParameters: &heartbeatParameters,
	})
}

func getEcsTaskNetworkDelayDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_delay", ecsTaskTargetId),
		Label:       "Delay Outgoing Traffic",
		Description: "Inject latency into egress network traffic.",
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("Duration of the attack."),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(0),
			},
			{
				Name:         "networkDelay",
				Label:        "Network Delay",
				Description:  extutil.Ptr("How much should the traffic be delayed?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("500ms"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "networkDelayJitter",
				Label:        "Jitter",
				Description:  extutil.Ptr("Add random +/-30% jitter to network delay?"),
				Type:         action_kit_api.Boolean,
				DefaultValue: extutil.Ptr("false"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			{
				Name:         "ip",
				Label:        "IP Address/CIDR",
				Description:  extutil.Ptr("Restrict to which IP addresses, CIDR blocks or domain names the traffic is affected."),
				Type:         action_kit_api.StringArray,
				DefaultValue: extutil.Ptr(""),
				Advanced:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
			},
		},
	}
}

func getEcsTaskNetworkDelayParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	fisActionStateParameter := newFisActionStateParameter(request.ExecutionId.String())
	delay := time.Duration(extutil.ToInt64(request.Config["networkDelay"])) * time.Millisecond
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
	if duration.Seconds() > 43200 {
		return nil, fmt.Errorf("duration longer than 43200 seconds is not supported")
	}
	sources := strings.Join(extutil.ToStringArray(request.Config["ip"]), ",")
	if sources == "" {
		sources = "0.0.0.0/0"
	}
	if err := validateSourcesPattern(sources); err != nil {
		return nil, err
	}
	jitter := 0 * time.Millisecond
	if extutil.ToBool(request.Config["networkDelayJitter"]) {
		jitter = delay * 30 / 100
	}
	//goland:noinspection GoDfaNilDereference
	return map[string][]string{
		"DurationSeconds":           {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"DelayMilliseconds":         {fmt.Sprintf("%d", delay.Milliseconds())},
		"JitterMilliseconds":        {fmt.Sprintf("%d", jitter.Milliseconds())},
		"Sources":                   {sources},
		"InstallDependencies":       {"True"},
		fisActionStateParameterName: {fisActionStateParameter},
	}, nil
}
