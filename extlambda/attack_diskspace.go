package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewFillDiskspaceAction() action_kit_sdk.Action[LambdaActionState] {
	return &lambdaAction{
		description:    getDiskspaceDescription(),
		configProvider: fillDiskspace,
		clientProvider: defaultClientProvider,
	}
}

func getDiskspaceDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.diskspace", lambdaTargetID),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Fill Diskspace",
		Description:     "Fills tmp diskspace of the function.",
		Icon:            new(lambdaTargetIcon),
		TargetSelection: &lambdaTargetSelection,
		Technology:      new("AWS"),
		Category:        new("Lambda"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  new("The duration of the attack."),
				Advanced:     new(false),
				Required:     new(true),
				DefaultValue: new("30s"),
				Order:        new(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  new("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: new("100"),
				Required:     new(true),
				Order:        new(1),
			},
			{
				Name:         "diskSpace",
				Label:        "Megabytes",
				Description:  new("Size in MB of the file created in tmp."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: nil,
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func fillDiskspace(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "diskspace",
		Rate:        request.Config["rate"].(float64) / 100.0,
		DiskSpace:   new(int(request.Config["diskSpace"].(float64))),
		IsEnabled:   true,
	}, nil
}
