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

func NewEcsTaskFillDiskAction() action_kit_sdk.Action[TaskSsmActionState] {
	return newEcsTaskSsmAction(getEcsTaskFillDiskDescription, ssmCommandInvocation{
		documentName:     "AWSFIS-Run-Disk-Fill",
		documentVersion:  "8",
		stepNameToOutput: "ExecuteDiskFill",
		getParameters:    getEcsTaskFillDiskParameters,
	})
}

func getEcsTaskFillDiskParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	duration := time.Duration(extutil.ToInt64(request.Config["duration"])) * time.Millisecond

	return map[string][]string{
		"DurationSeconds": {fmt.Sprintf("%d", int64(duration.Seconds()))},
		"Percent":         {strconv.Itoa(extutil.ToInt(request.Config["percent"]))},
	}, nil
}

func getEcsTaskFillDiskDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.fill_disk", ecsTaskTargetId),
		Label:       "Fill Disk",
		Description: "Fill ephemeral storage for the given duration.",
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percent",
				Label:        "Fill Percentage",
				Description:  extutil.Ptr("How many the percent of the allocated disk space dependent on the total available size shall be filled?"),
				Type:         action_kit_api.Percentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(0),
				MinValue:     extutil.Ptr(1),
				MaxValue:     extutil.Ptr(100),
			},
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long should the disk be filled?"),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("30s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
	}
}
