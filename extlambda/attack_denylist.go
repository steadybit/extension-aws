package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewDenylistAction() action_kit_sdk.Action[LambdaActionState] {
	return &lambdaAction{
		description:    getDenylistDescription(),
		configProvider: denyConnection,
		clientProvider: defaultClientProvider,
	}
}

func getDenylistDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.denylist", lambdaTargetID),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Block TCP Connections",
		Description:     "Blocks TCP connection made to listed host(s)",
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
				Name:         "denylist",
				Label:        "Hostname Deny Regex",
				Description:  new("Regular expression to match the hostname to deny traffic."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: new(".*"),
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func denyConnection(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	denylist := make([]string, 1)
	if request.Config["denylist"] == nil {
		denylist[0] = ""
	} else {
		denylist[0] = request.Config["denylist"].(string)
	}

	return &FailureInjectionConfig{
		FailureMode: "denylist",
		Rate:        request.Config["rate"].(float64) / 100.0,
		Denylist:    &denylist,
		IsEnabled:   true,
	}, nil
}
