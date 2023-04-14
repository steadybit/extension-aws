package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
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
		Id:          fmt.Sprintf("%s.latency", lambdaTargetID),
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Label:       "Inject Latency",
		Description: "Injects latency into the function.",
		Icon:        extutil.Ptr(lambdaTargetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: lambdaTargetID,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by function name",
					Query: "aws.lambda.function-name=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("application"),
		Kind:        action_kit_api.Attack,
		TimeControl: action_kit_api.External,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         "duration",
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
				Type:         action_kit_api.Percentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "minLatency",
				Label:        "Minimum Latency",
				Description:  extutil.Ptr("Minimum latency to inject."),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("500ms"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			{
				Name:         "maxLatency",
				Label:        "Maximum Latency",
				Description:  extutil.Ptr("Maximum latency to inject."),
				Type:         action_kit_api.Duration,
				DefaultValue: extutil.Ptr("500ms"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectLatency(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "latency",
		Rate:        request.Config["rate"].(float64) / 100.0,
		MinLatency:  extutil.Ptr(int(request.Config["minLatency"].(float64))),
		MaxLatency:  extutil.Ptr(int(request.Config["maxLatency"].(float64))),
		IsEnabled:   true,
	}, nil
}
