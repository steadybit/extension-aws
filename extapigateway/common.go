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
	apiGatewayIcon  = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik03LjkyOTU1IDIuMDM2ODVDOC4wNjYyNyAxLjk3NzM2IDguMjI1ODYgMS45OTA3MyA4LjM1MDUzIDIuMDcyNjVMOC4zNDk1NiAyLjA3MzYyQzguNDc0OTIgMi4xNTU0OCA4LjU1MDg2IDIuMjk1MTUgOC41NTA4NiAyLjQ0NDI4VjYuMjQ3NjNIOS45NjY3MVY3LjEzNTA4SDguNTUwODZWMTYuODY1MUg5Ljk2NjcxVjE3Ljc1MzVIOC41NTA4NlYyMS41NTU5QzguNTUwODQgMjEuNjYzOCA4LjUxMTY2IDIxLjc2NzIgOC40NDI0NyAyMS44NDcyTDguMzY0MDggMjEuOTE4OEM4LjI4ODM1IDIxLjk3MTYgOC4xOTgzIDIyLjAwMDEgOC4xMDY2NSAyMi4wMDAxQzguMDU3MzMgMjIuMDAwMSA4LjAwODIzIDIxLjk5MiA3Ljk2MDUxIDIxLjk3NDlMMi4yOTgwNyAyMC4wMDI2QzIuMTE5OTEgMTkuOTQwNSAyLjAwMDExIDE5Ljc3MjcgMiAxOS41ODM1VjQuOTAzMzlDMiA0LjcyNjcgMi4xMDQ1NiA0LjU2NjE5IDIuMjY3MTEgNC40OTU5Nkw3LjkyOTU1IDIuMDM2ODVaTTIuODg4NDIgNS4xOTM3MlYxOS4yNjYxTDcuNjYyNDQgMjAuOTI5N1YzLjEyMDc1TDIuODg4NDIgNS4xOTM3MloiIGZpbGw9IiM0MjRFNUMiLz4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0xNS42NDk1IDIuMDcyNjVDMTUuNzc0MiAxLjk5MDQxIDE1LjkzMjMgMS45Nzc5OCAxNi4wNjg1IDIuMDM3ODFMMTYuMDY5NSAyLjAzNjg1TDIxLjczMjkgNC40OTU5NkMyMS44OTU0IDQuNTY2MTkgMjIgNC43MjY3IDIyIDQuOTAzMzlWMTkuNTgzNUMyMS45OTk5IDE5Ljc3MjcgMjEuODgwMSAxOS45NDA1IDIxLjcwMTkgMjAuMDAyNkwxNi4wMzg1IDIxLjk3MzlMMTYuMDM5NSAyMS45NzQ5QzE1Ljk5MTggMjEuOTkyIDE1Ljk0MjcgMjIuMDAwMSAxNS44OTM0IDIyLjAwMDFDMTUuODI0NyAyMi4wMDAxIDE1Ljc1NzMgMjEuOTgzOSAxNS42OTU5IDIxLjk1MzZMMTUuNjM1OSAyMS45MTg4QzE1LjUxODcgMjEuODM1MyAxNS40NDkyIDIxLjY5OTUgMTUuNDQ5MSAyMS41NTU5VjE3Ljc1MzVIMTQuMDMzM1YxNi44NjUxSDE1LjQ0OTFWNy4xMzUwOEgxNC4wMzMzVjYuMjQ3NjNIMTUuNDQ5MVYyLjQ0NDI4QzE1LjQ0OTEgMi4yOTU0MSAxNS41MjQ1IDIuMTU0NTggMTUuNjQ5NSAyLjA3MjY1Wk0xNi4zMzc2IDIwLjkyOTdMMjEuMTExNiAxOS4yNjYxVjUuMTkzNzJMMTYuMzM3NiAzLjEyMDc1VjIwLjkyOTdaIiBmaWxsPSIjNDI0RTVDIi8+CjxwYXRoIGQ9Ik0xMi43OTg0IDE3Ljc1MzVIMTEuMjAxNlYxNi44NjUxSDEyLjc5ODRWMTcuNzUzNVoiIGZpbGw9IiM0MjRFNUMiLz4KPHBhdGggZD0iTTE0LjE2OTcgOC42MTc3TDE0Ljg0MjMgOC44NDEyNkwxNC45Mjc1IDguODY5MzJMMTQuODk5NCA4Ljk1NTQ2TDEyLjc3NTIgMTUuMzI1M0wxMi43NDcxIDE1LjQxMTVMMTIuNjYyIDE1LjM4MjRMMTEuOTg5NCAxNS4xNTg5TDExLjkwNDIgMTUuMTMwOEwxMS45MzIzIDE1LjA0NDdMMTQuMDU1NiA4LjY3NDhMMTQuMDg0NiA4LjU4ODY3TDE0LjE2OTcgOC42MTc3WiIgZmlsbD0iIzQyNEU1QyIvPgo8cGF0aCBkPSJNMTIuMDI0MiA5Ljc3MzIyTDEwLjE0OTYgMTEuNjQ1OUwxMi4wMjQyIDEzLjUxOTVMMTEuMzk1MSAxNC4xNDc2TDExLjMzMjIgMTQuMDgzN0w5LjIwNzk3IDExLjk2MDRDOS4wMzUwNCAxMS43ODY2IDkuMDM1MTUgMTEuNTA2MiA5LjIwNzk3IDExLjMzMjNMMTEuMzMyMiA5LjIwODA1TDExLjM5NTEgOS4xNDQxN0wxMi4wMjQyIDkuNzczMjJaIiBmaWxsPSIjNDI0RTVDIi8+CjxwYXRoIGQ9Ik0xMi43OTg0IDcuMTM1MDhIMTEuMjAxNlY2LjI0NzYzSDEyLjc5ODRWNy4xMzUwOFoiIGZpbGw9IiM0MjRFNUMiLz4KPC9zdmc+Cg=="
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
