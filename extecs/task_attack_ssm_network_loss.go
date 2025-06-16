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

func NewEcsTaskNetworkLossAction() action_kit_sdk.Action[TaskSsmActionState] {
	var heartbeatParameters = updateFisActionStateParameter
	return newEcsTaskSsmAction(getEcsTaskNetworkLossDescription, ssmCommandInvocation{
		documentName:              "AWSFIS-Run-Network-Packet-Loss-ECS",
		documentVersion:           "$DEFAULT",
		stepNameToOutput:          "FaultInjection",
		getParameters:             getEcsTaskNetworkLossParameters,
		updateHeartbeatParameters: &heartbeatParameters,
	})
}

func getEcsTaskNetworkLossDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.network_loss", ecsTaskTargetId),
		Label:       "Drop Outgoing Traffic",
		Description: "Cause packet loss for outgoing network traffic (egress).",
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
				Name:         "percentage",
				Label:        "Network Loss",
				Description:  extutil.Ptr("How much of the traffic should be lost?"),
				Type:         action_kit_api.Percentage,
				DefaultValue: extutil.Ptr("70"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "ip",
				Label:        "IP Address/CIDR",
				Description:  extutil.Ptr("Restrict to which IP addresses or blocks the traffic is affected."),
				Type:         action_kit_api.StringArray,
				DefaultValue: extutil.Ptr(""),
				Advanced:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
	}
}

func getEcsTaskNetworkLossParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	fisActionStateParameter := newFisActionStateParameter(request.ExecutionId.String())
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond
	if duration.Seconds() > 46800 {
		return nil, fmt.Errorf("duration longer than 46800 seconds is not supported")
	}
	sources := strings.Join(extutil.ToStringArray(request.Config["ip"]), ",")
	if sources == "" {
		sources = "0.0.0.0/0"
	}
	if err := validateSourcesPattern(sources); err != nil {
		return nil, err
	}
	return map[string][]string{
		fisActionStateParameterName: {fisActionStateParameter},
		"LossPercent":               {fmt.Sprintf("%d", extutil.ToInt(request.Config["percentage"]))},
		"Sources":                   {sources},
		"DurationSeconds":           {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"InstallDependencies":       {"True"},
	}, nil
}
