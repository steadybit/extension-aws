package extlambda

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const denylistBasePath = "/lambda/actions/inject-denylist"

func NewDenylistAction() action_kit_sdk.Action[LambdaActionState] {
	return &LambdaAction{
		description:    getDenylistDescription(),
		configProvider: denyConnection,
		clientProvider: defaultClientProvider,
	}
}

func getDenylistDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.denylist", lambdaTargetID),
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Label:       "Block TCP Connections",
		Description: "Blocks TCP connection made to listed host(s)",
		Icon:        extutil.Ptr(lambdaTargetIcon),
		TargetType:  extutil.Ptr(lambdaTargetID),
		TargetSelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label: "by function name",
				Query: "aws.lambda.function-name=\"\"",
			},
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
				Name:         "denylist",
				Label:        "Deny list",
				Description:  extutil.Ptr("List of regular expressions to match the hosts against"),
				Type:         action_kit_api.String1,
				DefaultValue: nil,
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   denylistBasePath + "/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   denylistBasePath + "/start",
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   denylistBasePath + "/stop",
		}),
	}
}

func denyConnection(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error) {
	denylist := make([]string, len(request.Config["denylist"].([]interface{})))
	for i, v := range request.Config["denylist"].([]interface{}) {
		denylist[i] = v.(string)
	}

	return &FailureInjectionConfig{
		FailureMode: "denylist",
		Rate:        request.Config["rate"].(float64) / 100.0,
		Denylist:    &denylist,
		IsEnabled:   true,
	}, nil
}
