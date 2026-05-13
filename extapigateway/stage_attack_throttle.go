// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type stageThrottleAttack struct {
	clientProvider func(account string, region string, role *string) (RestApiGatewayApi, error)
}

var (
	_ action_kit_sdk.Action[ApiGatewayStageThrottleAttackState]         = (*stageThrottleAttack)(nil)
	_ action_kit_sdk.ActionWithStop[ApiGatewayStageThrottleAttackState] = (*stageThrottleAttack)(nil)
)

func NewStageThrottleAttack() action_kit_sdk.ActionWithStop[ApiGatewayStageThrottleAttackState] {
	return &stageThrottleAttack{clientProvider: defaultRestClientProvider}
}

func (a *stageThrottleAttack) NewEmptyState() ApiGatewayStageThrottleAttackState {
	return ApiGatewayStageThrottleAttackState{}
}

func (a *stageThrottleAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.throttle", stageTargetType),
		Label:       "Throttle API Gateway stage (REST)",
		Description: "Lowers the stage-level throttle rate and burst limits for the duration of the experiment to simulate API throttling. Original limits are restored on stop. Supported on REST APIs only.",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        new(apiGatewayIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: stageTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by API id and stage (REST)",
					Description: new("Find REST API stage by API id and stage name"),
					Query:       "aws.apigateway.api.protocol-type=\"REST\" and aws.apigateway.api.id=\"\" and aws.apigateway.stage.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("API Gateway"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  new("Duration of the throttle. Original limits are restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: new("60s"),
				Order:        new(1),
				Required:     new(true),
			},
			{
				Name:         "rateLimit",
				Label:        "Throttle rate limit (req/s)",
				Description:  new("New throttle rate limit (requests per second). Use a small value to simulate aggressive throttling."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("1"),
				Order:        new(2),
				Required:     new(true),
			},
			{
				Name:         "burstLimit",
				Label:        "Throttle burst limit",
				Description:  new("New throttle burst limit (requests). Set lower than the original to constrain bursts."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("1"),
				Order:        new(3),
				Required:     new(true),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *stageThrottleAttack) Prepare(ctx context.Context, state *ApiGatewayStageThrottleAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.ApiId = extutil.MustHaveValue(request.Target.Attributes, "aws.apigateway.api.id")[0]
	state.StageName = extutil.MustHaveValue(request.Target.Attributes, "aws.apigateway.stage.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	protocols := request.Target.Attributes["aws.apigateway.api.protocol-type"]
	if len(protocols) == 0 || protocols[0] != protocolREST {
		return nil, extension_kit.ToError("Throttle attack is supported on REST API stages only.", nil)
	}

	rateLimit := extutil.ToInt(request.Config["rateLimit"])
	burstLimit := extutil.ToInt(request.Config["burstLimit"])
	if rateLimit < 0 || burstLimit < 0 {
		return nil, extension_kit.ToError("rateLimit and burstLimit must not be negative.", nil)
	}
	state.TargetRateLimit = float64(rateLimit)
	state.TargetBurstLimit = int32(burstLimit)

	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
	}
	stageOut, err := client.GetStage(ctx, &apigateway.GetStageInput{
		RestApiId: aws.String(state.ApiId),
		StageName: aws.String(state.StageName),
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	if ms, ok := stageOut.MethodSettings["*/*"]; ok {
		state.HadOriginalThrottleSettings = true
		state.OriginalRateLimit = ms.ThrottlingRateLimit
		state.OriginalBurstLimit = ms.ThrottlingBurstLimit
	}
	return nil, nil
}

func (a *stageThrottleAttack) Start(ctx context.Context, state *ApiGatewayStageThrottleAttackState) (*action_kit_api.StartResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
	}
	_, err = client.UpdateStage(ctx, &apigateway.UpdateStageInput{
		RestApiId: aws.String(state.ApiId),
		StageName: aws.String(state.StageName),
		PatchOperations: []apigwtypes.PatchOperation{
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/rateLimit"), Value: aws.String(strconv.FormatFloat(state.TargetRateLimit, 'f', -1, 64))},
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/burstLimit"), Value: aws.String(strconv.Itoa(int(state.TargetBurstLimit)))},
		},
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to throttle API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	msg := fmt.Sprintf("Throttled API Gateway stage %s/%s to rate=%v, burst=%d. Original: rate=%v, burst=%d (had-settings=%t).",
		state.ApiId, state.StageName, state.TargetRateLimit, state.TargetBurstLimit, state.OriginalRateLimit, state.OriginalBurstLimit, state.HadOriginalThrottleSettings)
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{Level: extutil.Ptr(action_kit_api.Info), Message: msg}}),
	}, nil
}

func (a *stageThrottleAttack) Stop(ctx context.Context, state *ApiGatewayStageThrottleAttackState) (*action_kit_api.StopResult, error) {
	client, err := a.clientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
	}

	patches := make([]apigwtypes.PatchOperation, 0, 2)
	if state.HadOriginalThrottleSettings {
		patches = append(patches,
			apigwtypes.PatchOperation{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/rateLimit"), Value: aws.String(strconv.FormatFloat(state.OriginalRateLimit, 'f', -1, 64))},
			apigwtypes.PatchOperation{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/burstLimit"), Value: aws.String(strconv.Itoa(int(state.OriginalBurstLimit)))},
		)
	} else {
		// Stage had no */* throttle override before; remove the ones we added so the stage falls back to account-level defaults.
		patches = append(patches,
			apigwtypes.PatchOperation{Op: apigwtypes.OpRemove, Path: aws.String("/*/*/throttling/rateLimit")},
			apigwtypes.PatchOperation{Op: apigwtypes.OpRemove, Path: aws.String("/*/*/throttling/burstLimit")},
		)
	}

	_, err = client.UpdateStage(ctx, &apigateway.UpdateStageInput{
		RestApiId:       aws.String(state.ApiId),
		StageName:       aws.String(state.StageName),
		PatchOperations: patches,
	})
	if err != nil {
		log.Error().Err(err).Msgf("Failed to restore throttle settings on API Gateway stage %s/%s", state.ApiId, state.StageName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore throttle settings on API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	msg := fmt.Sprintf("Restored throttle settings on API Gateway stage %s/%s (had-settings=%t).", state.ApiId, state.StageName, state.HadOriginalThrottleSettings)
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{Level: extutil.Ptr(action_kit_api.Info), Message: msg}}),
	}, nil
}
