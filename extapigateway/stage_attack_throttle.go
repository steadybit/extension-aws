// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type stageThrottleAttack struct {
	restClientProvider func(account string, region string, role *string) (RestApiGatewayApi, error)
	httpClientProvider func(account string, region string, role *string) (HttpApiGatewayApi, error)
}

var (
	_ action_kit_sdk.Action[ApiGatewayStageThrottleAttackState]         = (*stageThrottleAttack)(nil)
	_ action_kit_sdk.ActionWithStop[ApiGatewayStageThrottleAttackState] = (*stageThrottleAttack)(nil)
)

func NewStageThrottleAttack() action_kit_sdk.ActionWithStop[ApiGatewayStageThrottleAttackState] {
	return &stageThrottleAttack{
		restClientProvider: defaultRestClientProvider,
		httpClientProvider: defaultHttpClientProvider,
	}
}

func (a *stageThrottleAttack) NewEmptyState() ApiGatewayStageThrottleAttackState {
	return ApiGatewayStageThrottleAttackState{}
}

func (a *stageThrottleAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    fmt.Sprintf("%s.throttle", stageTargetType),
		Label: "Throttle API Gateway stage",
		Description: "Lowers the stage-level throttle rate and burst limits for the duration of the experiment to simulate API throttling. " +
			"Original limits are restored on stop. Supports REST APIs (v1) and HTTP APIs (v2); WebSocket APIs are not supported.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(apiGatewayIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: stageTargetType,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by API id and stage (REST)",
					Description: new("Find REST API stage by API id and stage name"),
					Query:       "aws.apigateway.api.protocol-type=\"REST\" and aws.apigateway.api.id=\"\" and aws.apigateway.stage.name=\"\"",
				},
				{
					Label:       "by API id and stage (HTTP)",
					Description: new("Find HTTP API stage by API id and stage name"),
					Query:       "aws.apigateway.api.protocol-type=\"HTTP\" and aws.apigateway.api.id=\"\" and aws.apigateway.stage.name=\"\"",
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
	if len(protocols) == 0 {
		return nil, extension_kit.ToError("Target is missing aws.apigateway.api.protocol-type attribute.", nil)
	}
	state.ProtocolType = protocols[0]
	if state.ProtocolType != protocolREST && state.ProtocolType != protocolHTTP {
		return nil, extension_kit.ToError(fmt.Sprintf("Throttle attack does not support protocol-type %q. Supported: REST, HTTP.", state.ProtocolType), nil)
	}

	rateLimit := extutil.ToInt(request.Config["rateLimit"])
	burstLimit := extutil.ToInt(request.Config["burstLimit"])
	if rateLimit < 0 || burstLimit < 0 {
		return nil, extension_kit.ToError("rateLimit and burstLimit must not be negative.", nil)
	}
	state.TargetRateLimit = float64(rateLimit)
	state.TargetBurstLimit = int32(burstLimit)

	switch state.ProtocolType {
	case protocolREST:
		return nil, a.prepareRest(ctx, state)
	case protocolHTTP:
		return nil, a.prepareHttp(ctx, state)
	}
	return nil, nil
}

func (a *stageThrottleAttack) prepareRest(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.restClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
	}
	stageOut, err := client.GetStage(ctx, &apigateway.GetStageInput{
		RestApiId: aws.String(state.ApiId),
		StageName: aws.String(state.StageName),
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to describe API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	if ms, ok := stageOut.MethodSettings["*/*"]; ok {
		state.RestStageHadMethodSetting = true
		// The AWS Go SDK uses non-pointer fields for ThrottlingRateLimit/BurstLimit, so an unset throttle
		// returns as zero. A literal 0 throttle is pathological (locks the stage to no traffic) and
		// indistinguishable here from "unset"; treat any positive value as set, otherwise fall through to
		// the "reset to account default" branch on Stop. AWS reports the unset sentinel as -1 too.
		if ms.ThrottlingRateLimit > 0 || ms.ThrottlingBurstLimit > 0 {
			state.HadOriginalThrottleSettings = true
			state.OriginalRateLimit = ms.ThrottlingRateLimit
			state.OriginalBurstLimit = ms.ThrottlingBurstLimit
		}
	}
	return nil
}

func (a *stageThrottleAttack) prepareHttp(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.httpClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway v2 client for AWS account %s", state.Account), err)
	}
	stageOut, err := client.GetStage(ctx, &apigatewayv2.GetStageInput{
		ApiId:     aws.String(state.ApiId),
		StageName: aws.String(state.StageName),
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to describe API Gateway v2 stage %s/%s", state.ApiId, state.StageName), err)
	}
	// HTTP $default stage on quick-created APIs is managed by API Gateway and cannot be modified — surface
	// the constraint here instead of letting UpdateStage fail mid-experiment with a less obvious error.
	if stageOut.ApiGatewayManaged != nil && *stageOut.ApiGatewayManaged {
		return extension_kit.ToError(fmt.Sprintf("HTTP API stage %s/%s is managed by API Gateway and cannot be modified.", state.ApiId, state.StageName), nil)
	}
	if stageOut.DefaultRouteSettings != nil {
		// Snapshot DefaultRouteSettings as JSON so we can restore non-throttle fields verbatim on Stop;
		// v2 has no JSON-Patch shape so a Start that only sets throttling would clobber DataTraceEnabled,
		// DetailedMetricsEnabled, LoggingLevel.
		encoded, err := json.Marshal(stageOut.DefaultRouteSettings)
		if err != nil {
			return extension_kit.ToError("Failed to snapshot original DefaultRouteSettings", err)
		}
		state.HttpOrigDefaultRouteSettings = string(encoded)
		if stageOut.DefaultRouteSettings.ThrottlingRateLimit != nil || stageOut.DefaultRouteSettings.ThrottlingBurstLimit != nil {
			state.HadOriginalThrottleSettings = true
			if stageOut.DefaultRouteSettings.ThrottlingRateLimit != nil {
				state.OriginalRateLimit = *stageOut.DefaultRouteSettings.ThrottlingRateLimit
			}
			if stageOut.DefaultRouteSettings.ThrottlingBurstLimit != nil {
				state.OriginalBurstLimit = *stageOut.DefaultRouteSettings.ThrottlingBurstLimit
			}
		}
	}
	return nil
}

func (a *stageThrottleAttack) Start(ctx context.Context, state *ApiGatewayStageThrottleAttackState) (*action_kit_api.StartResult, error) {
	switch state.ProtocolType {
	case protocolREST:
		if err := a.startRest(ctx, state); err != nil {
			return nil, err
		}
	case protocolHTTP:
		if err := a.startHttp(ctx, state); err != nil {
			return nil, err
		}
	}
	msg := fmt.Sprintf("Throttled API Gateway stage %s/%s (%s) to rate=%v, burst=%d. Original: rate=%v, burst=%d (had-settings=%t).",
		state.ApiId, state.StageName, state.ProtocolType, state.TargetRateLimit, state.TargetBurstLimit, state.OriginalRateLimit, state.OriginalBurstLimit, state.HadOriginalThrottleSettings)
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{Level: extutil.Ptr(action_kit_api.Info), Message: msg}}),
	}, nil
}

func (a *stageThrottleAttack) startRest(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.restClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
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
		return extension_kit.ToError(fmt.Sprintf("Failed to throttle API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	return nil
}

func (a *stageThrottleAttack) startHttp(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.httpClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway v2 client for AWS account %s", state.Account), err)
	}
	// Start from the original DefaultRouteSettings snapshot (if any) so non-throttle fields are preserved,
	// then override the two throttle fields. v2 UpdateStage replaces the whole DefaultRouteSettings object.
	settings := decodeRouteSettings(state.HttpOrigDefaultRouteSettings)
	rate := state.TargetRateLimit
	burst := state.TargetBurstLimit
	settings.ThrottlingRateLimit = &rate
	settings.ThrottlingBurstLimit = &burst
	_, err = client.UpdateStage(ctx, &apigatewayv2.UpdateStageInput{
		ApiId:                aws.String(state.ApiId),
		StageName:            aws.String(state.StageName),
		DefaultRouteSettings: &settings,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to throttle API Gateway v2 stage %s/%s", state.ApiId, state.StageName), err)
	}
	return nil
}

func (a *stageThrottleAttack) Stop(ctx context.Context, state *ApiGatewayStageThrottleAttackState) (*action_kit_api.StopResult, error) {
	switch state.ProtocolType {
	case protocolREST:
		if err := a.stopRest(ctx, state); err != nil {
			log.Error().Err(err).Msgf("Failed to restore throttle settings on API Gateway stage %s/%s", state.ApiId, state.StageName)
			return nil, err
		}
	case protocolHTTP:
		if err := a.stopHttp(ctx, state); err != nil {
			log.Error().Err(err).Msgf("Failed to restore throttle settings on API Gateway v2 stage %s/%s", state.ApiId, state.StageName)
			return nil, err
		}
	}
	msg := fmt.Sprintf("Restored throttle settings on API Gateway stage %s/%s (%s, had-settings=%t).", state.ApiId, state.StageName, state.ProtocolType, state.HadOriginalThrottleSettings)
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{Level: extutil.Ptr(action_kit_api.Info), Message: msg}}),
	}, nil
}

func (a *stageThrottleAttack) stopRest(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.restClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway client for AWS account %s", state.Account), err)
	}
	var patches []apigwtypes.PatchOperation
	switch {
	case state.HadOriginalThrottleSettings:
		// Throttle was set on */* before our attack — restore the captured values.
		patches = []apigwtypes.PatchOperation{
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/rateLimit"), Value: aws.String(strconv.FormatFloat(state.OriginalRateLimit, 'f', -1, 64))},
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/burstLimit"), Value: aws.String(strconv.Itoa(int(state.OriginalBurstLimit)))},
		}
	case state.RestStageHadMethodSetting:
		// */* MethodSetting existed (caching, metrics, etc.) but throttle was not set; reset throttle
		// to account-default via value=-1 while preserving the rest of the MethodSetting fields.
		patches = []apigwtypes.PatchOperation{
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/rateLimit"), Value: aws.String("-1")},
			{Op: apigwtypes.OpReplace, Path: aws.String("/*/*/throttling/burstLimit"), Value: aws.String("-1")},
		}
	default:
		// */* MethodSetting did not exist before our Start; remove the one our Start implicitly created.
		// AWS only accepts remove at the MethodSetting level — op=remove on /*/*/throttling/rateLimit
		// errors with "Cannot remove method setting ... because there is no method setting for this method".
		patches = []apigwtypes.PatchOperation{
			{Op: apigwtypes.OpRemove, Path: aws.String("/*/*")},
		}
	}
	_, err = client.UpdateStage(ctx, &apigateway.UpdateStageInput{
		RestApiId:       aws.String(state.ApiId),
		StageName:       aws.String(state.StageName),
		PatchOperations: patches,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to restore throttle settings on API Gateway stage %s/%s", state.ApiId, state.StageName), err)
	}
	return nil
}

func (a *stageThrottleAttack) stopHttp(ctx context.Context, state *ApiGatewayStageThrottleAttackState) error {
	client, err := a.httpClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to initialize API Gateway v2 client for AWS account %s", state.Account), err)
	}
	// Send the original DefaultRouteSettings verbatim. If there was no original, we send an empty
	// RouteSettings{} — that clears the throttling fields we set without affecting other stage config
	// (v2 UpdateStage only touches fields present in the request body).
	settings := decodeRouteSettings(state.HttpOrigDefaultRouteSettings)
	_, err = client.UpdateStage(ctx, &apigatewayv2.UpdateStageInput{
		ApiId:                aws.String(state.ApiId),
		StageName:            aws.String(state.StageName),
		DefaultRouteSettings: &settings,
	})
	if err != nil {
		return extension_kit.ToError(fmt.Sprintf("Failed to restore throttle settings on API Gateway v2 stage %s/%s", state.ApiId, state.StageName), err)
	}
	return nil
}

func decodeRouteSettings(encoded string) apigwv2types.RouteSettings {
	if encoded == "" {
		return apigwv2types.RouteSettings{}
	}
	var rs apigwv2types.RouteSettings
	if err := json.Unmarshal([]byte(encoded), &rs); err != nil {
		log.Warn().Err(err).Msg("Failed to decode DefaultRouteSettings snapshot; restoring with empty settings")
		return apigwv2types.RouteSettings{}
	}
	return rs
}
