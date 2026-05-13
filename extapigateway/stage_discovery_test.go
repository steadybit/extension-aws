// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type restApiMock struct {
	mock.Mock
}

func (m *restApiMock) GetRestApis(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigateway.GetRestApisOutput), args.Error(1)
}

func (m *restApiMock) GetStages(ctx context.Context, params *apigateway.GetStagesInput, optFns ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigateway.GetStagesOutput), args.Error(1)
}

func (m *restApiMock) GetStage(ctx context.Context, params *apigateway.GetStageInput, optFns ...func(*apigateway.Options)) (*apigateway.GetStageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigateway.GetStageOutput), args.Error(1)
}

func (m *restApiMock) UpdateStage(ctx context.Context, params *apigateway.UpdateStageInput, optFns ...func(*apigateway.Options)) (*apigateway.UpdateStageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigateway.UpdateStageOutput), args.Error(1)
}

type httpApiMock struct {
	mock.Mock
}

func (m *httpApiMock) GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigatewayv2.GetApisOutput), args.Error(1)
}

func (m *httpApiMock) GetStages(ctx context.Context, params *apigatewayv2.GetStagesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigatewayv2.GetStagesOutput), args.Error(1)
}

func (m *httpApiMock) GetStage(ctx context.Context, params *apigatewayv2.GetStageInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigatewayv2.GetStageOutput), args.Error(1)
}

func (m *httpApiMock) UpdateStage(ctx context.Context, params *apigatewayv2.UpdateStageInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.UpdateStageOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apigatewayv2.UpdateStageOutput), args.Error(1)
}

func TestGetAllStagesRestAndHttp(t *testing.T) {
	rest := new(restApiMock)
	httpApi := new(httpApiMock)

	rest.On("GetRestApis", mock.Anything, mock.Anything).Return(&apigateway.GetRestApisOutput{
		Items: []apigwtypes.RestApi{
			{
				Id:   aws.String("rest-1"),
				Name: aws.String("payments"),
				EndpointConfiguration: &apigwtypes.EndpointConfiguration{
					Types: []apigwtypes.EndpointType{apigwtypes.EndpointTypeRegional},
				},
				DisableExecuteApiEndpoint: false,
				Tags:                      map[string]string{"application": "Demo"},
			},
		},
	}, nil)
	rest.On("GetStages", mock.Anything, mock.MatchedBy(func(p *apigateway.GetStagesInput) bool {
		return aws.ToString(p.RestApiId) == "rest-1"
	})).Return(&apigateway.GetStagesOutput{
		Item: []apigwtypes.Stage{
			{
				StageName:           aws.String("prod"),
				CacheClusterEnabled: true,
				CacheClusterSize:    apigwtypes.CacheClusterSizeSize0Point5Gb,
				TracingEnabled:      true,
				MethodSettings: map[string]apigwtypes.MethodSetting{
					"*/*": {
						ThrottlingRateLimit:  500,
						ThrottlingBurstLimit: 1000,
						LoggingLevel:         aws.String("INFO"),
					},
				},
				AccessLogSettings: &apigwtypes.AccessLogSettings{DestinationArn: aws.String("arn:aws:logs:us-east-1:42:log-group:/api/payments")},
				WebAclArn:         aws.String("arn:aws:wafv2:..."),
			},
		},
	}, nil)

	httpApi.On("GetApis", mock.Anything, mock.Anything).Return(&apigatewayv2.GetApisOutput{
		Items: []apigwv2types.Api{
			{
				ApiId:                     aws.String("http-1"),
				Name:                      aws.String("orders"),
				ProtocolType:              apigwv2types.ProtocolTypeHttp,
				DisableExecuteApiEndpoint: aws.Bool(false),
				Tags:                      map[string]string{"application": "Demo"},
			},
		},
	}, nil)
	httpApi.On("GetStages", mock.Anything, mock.MatchedBy(func(p *apigatewayv2.GetStagesInput) bool {
		return aws.ToString(p.ApiId) == "http-1"
	})).Return(&apigatewayv2.GetStagesOutput{
		Items: []apigwv2types.Stage{
			{
				StageName:  aws.String("$default"),
				AutoDeploy: aws.Bool(true),
				DefaultRouteSettings: &apigwv2types.RouteSettings{
					ThrottlingRateLimit:  aws.Float64(2000),
					ThrottlingBurstLimit: aws.Int32(5000),
					LoggingLevel:         apigwv2types.LoggingLevelInfo,
				},
				AccessLogSettings: &apigwv2types.AccessLogSettings{DestinationArn: aws.String("arn:logs:orders")},
			},
		},
	}, nil)

	targets, err := getAllStages(context.Background(), rest, httpApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    aws.String("arn:role"),
		TagFilters:    []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(targets))

	var restTgt, httpTgt *struct{ idx int }
	for i, tgt := range targets {
		idxCopy := i
		if tgt.Attributes["aws.apigateway.api.protocol-type"][0] == "REST" {
			restTgt = &struct{ idx int }{idxCopy}
		}
		if tgt.Attributes["aws.apigateway.api.protocol-type"][0] == "HTTP" {
			httpTgt = &struct{ idx int }{idxCopy}
		}
	}
	assert.NotNil(t, restTgt, "expected REST target present")
	assert.NotNil(t, httpTgt, "expected HTTP target present")

	rt := targets[restTgt.idx]
	assert.Equal(t, stageTargetType, rt.TargetType)
	assert.Equal(t, "payments/prod", rt.Label)
	assert.Equal(t, []string{"REST"}, rt.Attributes["aws.apigateway.api.protocol-type"])
	assert.Equal(t, []string{"REGIONAL"}, rt.Attributes["aws.apigateway.api.endpoint-type"])
	assert.Equal(t, []string{"500"}, rt.Attributes["aws.apigateway.stage.throttle.rate-limit"])
	assert.Equal(t, []string{"1000"}, rt.Attributes["aws.apigateway.stage.throttle.burst-limit"])
	assert.Equal(t, []string{"true"}, rt.Attributes["aws.apigateway.stage.cache.enabled"])
	assert.Equal(t, []string{"0.5"}, rt.Attributes["aws.apigateway.stage.cache.size"])
	assert.Equal(t, []string{"true"}, rt.Attributes["aws.apigateway.stage.tracing-enabled"])
	assert.Equal(t, []string{"INFO"}, rt.Attributes["aws.apigateway.stage.logging-level"])
	assert.Equal(t, []string{"true"}, rt.Attributes["aws.apigateway.stage.access-log.configured"])
	assert.Equal(t, []string{"arn:aws:wafv2:..."}, rt.Attributes["aws.apigateway.stage.waf-arn"])
	assert.Equal(t, []string{"Demo"}, rt.Attributes["aws.apigateway.api.label.application"])
	assert.Equal(t, []string{"arn:role"}, rt.Attributes["extension-aws.discovered-by-role"])

	ht := targets[httpTgt.idx]
	assert.Equal(t, "orders/$default", ht.Label)
	assert.Equal(t, []string{"HTTP"}, ht.Attributes["aws.apigateway.api.protocol-type"])
	assert.Equal(t, []string{"2000"}, ht.Attributes["aws.apigateway.stage.throttle.rate-limit"])
	assert.Equal(t, []string{"5000"}, ht.Attributes["aws.apigateway.stage.throttle.burst-limit"])
	assert.Equal(t, []string{"INFO"}, ht.Attributes["aws.apigateway.stage.logging-level"])
	assert.Equal(t, []string{"true"}, ht.Attributes["aws.apigateway.stage.access-log.configured"])
	assert.Equal(t, []string{"true"}, ht.Attributes["aws.apigateway.stage.auto-deploy"])
}

func TestPrivateRestApiSurfacesEndpointType(t *testing.T) {
	rest := new(restApiMock)
	httpApi := new(httpApiMock)
	rest.On("GetRestApis", mock.Anything, mock.Anything).Return(&apigateway.GetRestApisOutput{
		Items: []apigwtypes.RestApi{
			{
				Id:   aws.String("rest-priv"),
				Name: aws.String("internal"),
				EndpointConfiguration: &apigwtypes.EndpointConfiguration{
					Types: []apigwtypes.EndpointType{apigwtypes.EndpointTypePrivate},
				},
			},
		},
	}, nil)
	rest.On("GetStages", mock.Anything, mock.Anything).Return(&apigateway.GetStagesOutput{
		Item: []apigwtypes.Stage{{StageName: aws.String("v1")}},
	}, nil)
	httpApi.On("GetApis", mock.Anything, mock.Anything).Return(&apigatewayv2.GetApisOutput{}, nil)

	targets, err := getAllStages(context.Background(), rest, httpApi, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	assert.Equal(t, []string{"PRIVATE"}, targets[0].Attributes["aws.apigateway.api.endpoint-type"])
}

func TestHttpApiFailureDoesNotKillRestDiscovery(t *testing.T) {
	rest := new(restApiMock)
	httpApi := new(httpApiMock)
	rest.On("GetRestApis", mock.Anything, mock.Anything).Return(&apigateway.GetRestApisOutput{
		Items: []apigwtypes.RestApi{{Id: aws.String("rest-1"), Name: aws.String("only-rest")}},
	}, nil)
	rest.On("GetStages", mock.Anything, mock.Anything).Return(&apigateway.GetStagesOutput{
		Item: []apigwtypes.Stage{{StageName: aws.String("v1")}},
	}, nil)
	// HTTP API call fails
	httpApi.On("GetApis", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	targets, err := getAllStages(context.Background(), rest, httpApi, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err, "v1 discovery must not error when v2 fails")
	assert.Equal(t, 1, len(targets))
	assert.Equal(t, []string{"REST"}, targets[0].Attributes["aws.apigateway.api.protocol-type"])
}
