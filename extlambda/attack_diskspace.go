package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
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
		Icon:            extutil.Ptr(lambdaTargetIcon),
		TargetSelection: &lambdaTargetSelection,
		Technology:      extutil.Ptr("AWS"),
		Category:        extutil.Ptr("Lambda"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  extutil.Ptr("The duration of the attack."),
				Advanced:     extutil.Ptr(false),
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  extutil.Ptr("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "diskSpace",
				Label:        "Megabytes",
				Description:  extutil.Ptr("Size in MB of the file created in tmp."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: nil,
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func fillDiskspace(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "diskspace",
		Rate:        request.Config["rate"].(float64) / 100.0,
		DiskSpace:   extutil.Ptr(int(request.Config["diskSpace"].(float64))),
		IsEnabled:   true,
	}, nil
}
