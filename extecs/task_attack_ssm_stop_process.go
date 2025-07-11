// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extutil"
	"regexp"
)

func NewEcsTaskStopProcessAction() action_kit_sdk.Action[TaskSsmActionState] {
	return newEcsTaskSsmAction(getEcsTaskStopProcessDescription, ssmCommandInvocation{
		documentName:     "AWSFIS-Run-Kill-Process",
		documentVersion:  "$DEFAULT",
		stepNameToOutput: "KillProcess",
		getParameters:    getEcsTaskStopProcessParameters,
	})
}

func getEcsTaskStopProcessDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.stop-process", ecsTaskTargetId),
		Label:       "Stop Process",
		Description: "Stop a particular process by name in an instance. ",
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:        "process",
				Label:       "Process",
				Description: extutil.Ptr("Name of the process to stop."),
				Type:        action_kit_api.ActionParameterTypeString,
				Required:    extutil.Ptr(true),
				Order:       extutil.Ptr(0),
			},
			{
				Name:         "graceful",
				Label:        "Graceful",
				Description:  extutil.Ptr("If true a TERM signal is sent, otherwise the KILL signal."),
				Type:         action_kit_api.ActionParameterTypeBoolean,
				DefaultValue: extutil.Ptr("true"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
		},
	}
}

func getEcsTaskStopProcessParameters(request action_kit_api.PrepareActionRequestBody) (map[string][]string, error) {
	process := extutil.ToString(request.Config["process"])
	pattern := `^[0-9a-zA-Z.\-=_]{1,128}$`
	matched, err := regexp.MatchString(pattern, process)
	if err != nil {
		return nil, fmt.Errorf("failed to validate process name: %w", err)
	}
	if !matched {
		return nil, fmt.Errorf("invalid process name: must match %q", pattern)
	}
	var signal = "SIGKILL"
	if extutil.ToBool(request.Config["graceful"]) {
		signal = "SIGTERM"
	}
	return map[string][]string{
		"ProcessName":         {process},
		"Signal":              {signal},
		"InstallDependencies": {"True"},
	}, nil
}
