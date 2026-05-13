// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	apiGatewayIcon  = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djE2SDRWNHptMiAyaDR2NEg2VjZ6bTYgMGg2djRoLTZWNnptLTYgNmg2djZINnYtNnptOCAwaDR2Mmg0djRoLTRoLTRoLTRWMTJ6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
	stageTargetType = "com.steadybit.extension_aws.apigateway.stage"
)

// ApiGatewayStageThrottleAttackState captures the throttle config of an API Gateway stage so we can restore
// it on stop. The same state struct serves both REST (v1) and HTTP (v2) stages — ProtocolType drives the
// branching in Prepare/Start/Stop.
type ApiGatewayStageThrottleAttackState struct {
	ApiId            string
	StageName        string
	Account          string
	Region           string
	ProtocolType     string // "REST" or "HTTP"
	DiscoveredByRole *string

	// Target values (apply to both REST and HTTP).
	TargetRateLimit  float64
	TargetBurstLimit int32

	// REST snapshot pulled from Stage.MethodSettings["*/*"]. Three cases drive the Stop restore:
	//   - HadOriginalThrottleSettings=true → replace throttle fields with the captured values.
	//   - HadOriginalThrottleSettings=false && RestStageHadMethodSetting=true → reset throttle to
	//     account defaults via op=replace value=-1, preserving other MethodSetting fields (caching,
	//     metrics, logging).
	//   - RestStageHadMethodSetting=false → op=remove path=/*/* to delete the MethodSetting our Start
	//     implicitly created. AWS does not support op=remove on individual property paths under a
	//     MethodSetting, so the per-field remove that was here previously erroneously failed.
	OriginalRateLimit           float64
	OriginalBurstLimit          int32
	HadOriginalThrottleSettings bool
	RestStageHadMethodSetting   bool

	// HTTP snapshot: full Stage.DefaultRouteSettings serialised as JSON so we can restore non-throttle
	// fields (DataTraceEnabled, DetailedMetricsEnabled, LoggingLevel) verbatim. Empty string means
	// DefaultRouteSettings was nil; on Stop we send an empty RouteSettings to clear what we set.
	HttpOrigDefaultRouteSettings string
}

type RestApiGatewayApi interface {
	GetRestApis(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error)
	GetStages(ctx context.Context, params *apigateway.GetStagesInput, optFns ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error)
	GetStage(ctx context.Context, params *apigateway.GetStageInput, optFns ...func(*apigateway.Options)) (*apigateway.GetStageOutput, error)
	UpdateStage(ctx context.Context, params *apigateway.UpdateStageInput, optFns ...func(*apigateway.Options)) (*apigateway.UpdateStageOutput, error)
}

type HttpApiGatewayApi interface {
	GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
	GetStages(ctx context.Context, params *apigatewayv2.GetStagesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error)
	GetStage(ctx context.Context, params *apigatewayv2.GetStageInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStageOutput, error)
	UpdateStage(ctx context.Context, params *apigatewayv2.UpdateStageInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.UpdateStageOutput, error)
}

func defaultRestClientProvider(account string, region string, role *string) (RestApiGatewayApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return apigateway.NewFromConfig(awsAccess.AwsConfig), nil
}

func defaultHttpClientProvider(account string, region string, role *string) (HttpApiGatewayApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return apigatewayv2.NewFromConfig(awsAccess.AwsConfig), nil
}
