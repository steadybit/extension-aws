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

// ApiGatewayStageThrottleAttackState captures the throttle config of a REST stage so we can restore it on stop.
type ApiGatewayStageThrottleAttackState struct {
	ApiId                       string
	StageName                   string
	Account                     string
	Region                      string
	DiscoveredByRole            *string
	OriginalRateLimit           float64
	OriginalBurstLimit          int32
	HadOriginalThrottleSettings bool
	TargetRateLimit             float64
	TargetBurstLimit            int32
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
