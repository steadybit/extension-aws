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

func NewEcsTaskStressIoAction() action_kit_sdk.Action[TaskSsmActionState] {
	return newEcsTaskSsmAction(getEcsTaskStressIoDescription, ssmCommandInvocation{
		documentName:     "AWSFIS-Run-IO-Stress",
		documentVersion:  "$DEFAULT",
		stepNameToOutput: "ExecuteStressNg",
		getParameters:    getEcsTaskStressIoParameters,
	})
}

func getEcsTaskStressIoParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

	return map[string][]string{
		"DurationSeconds": {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"Workers":         {strconv.Itoa(extutil.ToInt(request.Config["workers"]))},
		"Percent":         {strconv.Itoa(extutil.ToInt(request.Config["percent"]))},
	}, nil
}

func getEcsTaskStressIoDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stress_io", ecsTaskTargetId),
		Label:       "Stress IO",
		Description: "Stresses IO on the ephemeral storage for the given duration.",
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percent",
				Label:        "Disk Space Percentage",
				Description:  new("How many the percent of the available file system space shall be used by each worker?"),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: new("100"),
				Required:     new(true),
				Order:        new(0),
				MinValue:     new(1),
				MaxValue:     new(100),
			},
			{
				Name:         "workers",
				Label:        "Workers",
				Description:  new("How many workers should stress the IO?"),
				Type:         action_kit_api.ActionParameterTypeStressngWorkers,
				DefaultValue: new("1"),
				Required:     new(true),
				Order:        new(1),
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("How long should the IO be stressed?"),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("30s"),
				Required:     new(true),
				Order:        new(2),
			},
		},
	}
}
