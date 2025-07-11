// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package extecs

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extutil"
	"strconv"
	"time"
)

func NewEcsTaskStressMemoryAction() action_kit_sdk.Action[TaskSsmActionState] {
	return newEcsTaskSsmAction(getEcsTaskStressMemoryDescription, ssmCommandInvocation{
		documentName:     "AWSFIS-Run-Memory-Stress",
		documentVersion:  "$DEFAULT",
		stepNameToOutput: "ExecuteStressNg",
		getParameters:    getEcsTaskStressMemoryParameters,
	})
}

func getEcsTaskStressMemoryParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

	return map[string][]string{
		"DurationSeconds": {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"Workers":         {strconv.Itoa(extutil.ToInt(request.Config["workers"]))},
		"Percent":         {strconv.Itoa(extutil.ToInt(request.Config["percent"]))},
	}, nil
}

func getEcsTaskStressMemoryDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stress_mem", ecsTaskTargetId),
		Label:       "Stress Memory",
		Description: "Stresses Memory for the given duration.",
		Hint: &action_kit_api.ActionHint{
			Type:    action_kit_api.HintInfo,
			Content: "This action targets the entire task. Not individual containers inside it.",
		},
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percent",
				Label:        "Memory Percentage",
				Description:  extutil.Ptr("How many the percent of the available virtual memory shall be used?"),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(0),
				MinValue:     extutil.Ptr(1),
				MaxValue:     extutil.Ptr(100),
			},
			{
				Name:         "workers",
				Label:        "Workers",
				Description:  extutil.Ptr("How many workers should stress the memory?"),
				Type:         action_kit_api.ActionParameterTypeStressngWorkers,
				DefaultValue: extutil.Ptr("1"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should the memory be stressed?"),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
	}
}
