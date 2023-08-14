package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
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
		Icon:            extutil.Ptr(lambdaTargetIcon),
		TargetSelection: &lambdaTargetSelection,
		Category:        extutil.Ptr("application"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
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
				Name:         "statuscode",
				Label:        "Status Code",
				Description:  extutil.Ptr("The status code to return."),
				Type:         action_kit_api.Integer,
				DefaultValue: extutil.Ptr("500"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}
func injectStatusCode(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	return &FailureInjectionConfig{
		FailureMode: "statuscode",
		Rate:        request.Config["rate"].(float64) / 100.0,
		StatusCode:  extutil.Ptr(int(request.Config["statuscode"].(float64))),
		IsEnabled:   true,
	}, nil
}
