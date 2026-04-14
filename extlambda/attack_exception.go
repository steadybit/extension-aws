package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewInjectExceptionAction() action_kit_sdk.Action[LambdaActionState] {
	return &lambdaAction{
		description:    getInjectExceptionDescription(),
		configProvider: injectException,
		clientProvider: defaultClientProvider,
	}
}

func getInjectExceptionDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.exception", lambdaTargetID),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Exception",
		Description:     "Injects exception into the function.",
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
				Name:         "exceptionMsg",
				Label:        "Message",
				Description:  new("Message of the thrown exception."),
				Type:         action_kit_api.ActionParameterTypeString,
				DefaultValue: new("Injected exception"),
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectException(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode:  "exception",
		Rate:         request.Config["rate"].(float64) / 100.0,
		ExceptionMsg: new(request.Config["exceptionMsg"].(string)),
		IsEnabled:    true,
	}, nil
}
