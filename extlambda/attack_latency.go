package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewInjectLatencyAction() action_kit_sdk.Action[LambdaActionState] {
	return &lambdaAction{
		description:    getInjectLatencyDescription(),
		configProvider: injectLatency,
		clientProvider: defaultClientProvider,
	}
}

func getInjectLatencyDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.latency", lambdaTargetID),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Latency",
		Description:     "Injects latency into the function.",
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
				Name:         "minLatency",
				Label:        "Minimum Latency",
				Description:  new("Minimum latency to inject."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("500ms"),
				Required:     new(true),
				Order:        new(2),
			},
			{
				Name:         "maxLatency",
				Label:        "Maximum Latency",
				Description:  new("Maximum latency to inject."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("500ms"),
				Required:     new(true),
				Order:        new(3),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectLatency(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "latency",
		Rate:        request.Config["rate"].(float64) / 100.0,
		MinLatency:  new(int(request.Config["minLatency"].(float64))),
		MaxLatency:  new(int(request.Config["maxLatency"].(float64))),
		IsEnabled:   true,
	}, nil
}
