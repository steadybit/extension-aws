package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewInjectStatusCodeAction() action_kit_sdk.Action[LambdaActionState] {
	return &lambdaAction{
		description:    getInjectStatusCodeDescription(),
		configProvider: injectStatusCode,
		clientProvider: defaultClientProvider,
	}
}

func getInjectStatusCodeDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.statusCode", lambdaTargetID),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Status Code",
		Description:     "Returns a fixed status code.",
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
				Name:         "statuscode",
				Label:        "Status Code",
				Description:  new("The status code to return."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("500"),
				MinValue:     new(100),
				MaxValue:     new(599),
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}
func injectStatusCode(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "statuscode",
		Rate:        request.Config["rate"].(float64) / 100.0,
		StatusCode:  new(int(request.Config["statuscode"].(float64))),
		IsEnabled:   true,
	}, nil
}
