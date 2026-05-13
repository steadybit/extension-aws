// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newThrottleRequest(rate int, burst int, protocol string) action_kit_api.PrepareActionRequestBody {
	apiId := "rest-1"
	if protocol == "HTTP" {
		apiId = "http-1"
	}
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{"rateLimit": rate, "burstLimit": burst},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":                      {"42"},
				"aws.region":                       {"us-east-1"},
				"aws.apigateway.api.id":            {apiId},
				"aws.apigateway.stage.name":        {"prod"},
				"aws.apigateway.api.protocol-type": {protocol},
				"extension-aws.discovered-by-role": {"arn:role"},
			},
		}),
	})
}

func newRestAttack(api *restApiMock) stageThrottleAttack {
	return stageThrottleAttack{
		restClientProvider: func(account string, region string, role *string) (RestApiGatewayApi, error) { return api, nil },
	}
}

func newHttpAttack(api *httpApiMock) stageThrottleAttack {
	return stageThrottleAttack{
		httpClientProvider: func(account string, region string, role *string) (HttpApiGatewayApi, error) { return api, nil },
	}
}

func TestPrepareThrottleCapturesOriginalSettings(t *testing.T) {
	api := new(restApiMock)
	api.On("GetStage", mock.Anything, mock.Anything).Return(&apigateway.GetStageOutput{
		MethodSettings: map[string]apigwtypes.MethodSetting{
			"*/*": {ThrottlingRateLimit: 500, ThrottlingBurstLimit: 1000},
		},
	}, nil)
	attack := newRestAttack(api)
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(1, 1, "REST"))
	require.NoError(t, err)
	assert.Equal(t, "REST", state.ProtocolType)
	assert.Equal(t, "rest-1", state.ApiId)
	assert.Equal(t, "prod", state.StageName)
	assert.Equal(t, float64(1), state.TargetRateLimit)
	assert.Equal(t, int32(1), state.TargetBurstLimit)
	assert.True(t, state.RestStageHadMethodSetting)
	assert.True(t, state.HadOriginalThrottleSettings)
	assert.Equal(t, float64(500), state.OriginalRateLimit)
	assert.Equal(t, int32(1000), state.OriginalBurstLimit)
}

func TestPrepareThrottleNoOriginalSettings(t *testing.T) {
	api := new(restApiMock)
	api.On("GetStage", mock.Anything, mock.Anything).Return(&apigateway.GetStageOutput{
		MethodSettings: map[string]apigwtypes.MethodSetting{}, // no */* override
	}, nil)
	attack := newRestAttack(api)
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(5, 10, "REST"))
	require.NoError(t, err)
	assert.False(t, state.RestStageHadMethodSetting)
	assert.False(t, state.HadOriginalThrottleSettings)
}

func TestPrepareThrottleMethodSettingExistsWithoutThrottle(t *testing.T) {
	// */* MethodSetting present but throttle fields are zero (Go SDK quirk for unset). Should NOT be
	// treated as "had original throttle" — Stop will reset to account default instead of locking the
	// stage to 0 rps.
	api := new(restApiMock)
	api.On("GetStage", mock.Anything, mock.Anything).Return(&apigateway.GetStageOutput{
		MethodSettings: map[string]apigwtypes.MethodSetting{
			"*/*": {CachingEnabled: true, MetricsEnabled: true},
		},
	}, nil)
	attack := newRestAttack(api)
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(1, 1, "REST"))
	require.NoError(t, err)
	assert.True(t, state.RestStageHadMethodSetting)
	assert.False(t, state.HadOriginalThrottleSettings)
}

func TestPrepareThrottleRejectsWebSocket(t *testing.T) {
	attack := newRestAttack(new(restApiMock))
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(1, 1, "WEBSOCKET"))
	require.Error(t, err)
}

func TestStartThrottlePatchesStage(t *testing.T) {
	api := new(restApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigateway.UpdateStageInput) bool {
		require.Equal(t, "rest-1", aws.ToString(p.RestApiId))
		require.Equal(t, "prod", aws.ToString(p.StageName))
		require.Equal(t, 2, len(p.PatchOperations))
		require.Equal(t, apigwtypes.OpReplace, p.PatchOperations[0].Op)
		require.Equal(t, "/*/*/throttling/rateLimit", aws.ToString(p.PatchOperations[0].Path))
		require.Equal(t, "1", aws.ToString(p.PatchOperations[0].Value))
		require.Equal(t, "/*/*/throttling/burstLimit", aws.ToString(p.PatchOperations[1].Path))
		require.Equal(t, "1", aws.ToString(p.PatchOperations[1].Value))
		return true
	})).Return(&apigateway.UpdateStageOutput{}, nil)
	attack := newRestAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "rest-1", StageName: "prod", Account: "42", Region: "us-east-1", ProtocolType: "REST",
		TargetRateLimit: 1, TargetBurstLimit: 1,
	}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopRestoresOriginalSettings(t *testing.T) {
	api := new(restApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigateway.UpdateStageInput) bool {
		require.Equal(t, 2, len(p.PatchOperations))
		require.Equal(t, "500", aws.ToString(p.PatchOperations[0].Value))
		require.Equal(t, "1000", aws.ToString(p.PatchOperations[1].Value))
		return true
	})).Return(&apigateway.UpdateStageOutput{}, nil)
	attack := newRestAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "rest-1", StageName: "prod", Account: "42", Region: "us-east-1", ProtocolType: "REST",
		HadOriginalThrottleSettings: true, OriginalRateLimit: 500, OriginalBurstLimit: 1000,
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopRemovesMethodSettingWhenItDidNotExist(t *testing.T) {
	// */* MethodSetting did not exist before our Start; Stop must remove the one Start implicitly
	// created via a single op=remove path=/*/*. Per-property op=remove is not supported by AWS.
	api := new(restApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigateway.UpdateStageInput) bool {
		require.Equal(t, 1, len(p.PatchOperations))
		require.Equal(t, apigwtypes.OpRemove, p.PatchOperations[0].Op)
		require.Equal(t, "/*/*", aws.ToString(p.PatchOperations[0].Path))
		return true
	})).Return(&apigateway.UpdateStageOutput{}, nil)
	attack := newRestAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "rest-1", StageName: "prod", ProtocolType: "REST",
		HadOriginalThrottleSettings: false,
		RestStageHadMethodSetting:   false,
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopResetsToAccountDefaultWhenMethodSettingExistedWithoutThrottle(t *testing.T) {
	// */* MethodSetting existed with caching/metrics but no throttle; Stop must reset throttle to
	// account default (-1) without wiping the MethodSetting.
	api := new(restApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigateway.UpdateStageInput) bool {
		require.Equal(t, 2, len(p.PatchOperations))
		require.Equal(t, apigwtypes.OpReplace, p.PatchOperations[0].Op)
		require.Equal(t, "/*/*/throttling/rateLimit", aws.ToString(p.PatchOperations[0].Path))
		require.Equal(t, "-1", aws.ToString(p.PatchOperations[0].Value))
		require.Equal(t, apigwtypes.OpReplace, p.PatchOperations[1].Op)
		require.Equal(t, "/*/*/throttling/burstLimit", aws.ToString(p.PatchOperations[1].Path))
		require.Equal(t, "-1", aws.ToString(p.PatchOperations[1].Value))
		return true
	})).Return(&apigateway.UpdateStageOutput{}, nil)
	attack := newRestAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "rest-1", StageName: "prod", ProtocolType: "REST",
		HadOriginalThrottleSettings: false,
		RestStageHadMethodSetting:   true,
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartThrottleForwardsError(t *testing.T) {
	api := new(restApiMock)
	api.On("UpdateStage", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	attack := newRestAttack(api)
	state := ApiGatewayStageThrottleAttackState{ApiId: "rest-1", StageName: "prod", ProtocolType: "REST"}
	_, err := attack.Start(context.Background(), &state)
	assert.Error(t, err)
}

// --- HTTP (v2) coverage --------------------------------------------------------

func TestPrepareThrottleHttpCapturesOriginalSettings(t *testing.T) {
	api := new(httpApiMock)
	origRate := float64(500)
	origBurst := int32(1000)
	dataTrace := true
	api.On("GetStage", mock.Anything, mock.Anything).Return(&apigatewayv2.GetStageOutput{
		StageName: aws.String("prod"),
		DefaultRouteSettings: &apigwv2types.RouteSettings{
			ThrottlingRateLimit:  &origRate,
			ThrottlingBurstLimit: &origBurst,
			DataTraceEnabled:     &dataTrace,
		},
	}, nil)
	attack := newHttpAttack(api)
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(1, 1, "HTTP"))
	require.NoError(t, err)
	assert.Equal(t, "HTTP", state.ProtocolType)
	assert.True(t, state.HadOriginalThrottleSettings)
	assert.Equal(t, float64(500), state.OriginalRateLimit)
	assert.Equal(t, int32(1000), state.OriginalBurstLimit)
	assert.NotEmpty(t, state.HttpOrigDefaultRouteSettings) // JSON snapshot preserves DataTraceEnabled
}

func TestPrepareThrottleHttpRejectsManagedStage(t *testing.T) {
	api := new(httpApiMock)
	managed := true
	api.On("GetStage", mock.Anything, mock.Anything).Return(&apigatewayv2.GetStageOutput{
		StageName:         aws.String("$default"),
		ApiGatewayManaged: &managed,
	}, nil)
	attack := newHttpAttack(api)
	state := attack.NewEmptyState()
	_, err := attack.Prepare(context.Background(), &state, newThrottleRequest(1, 1, "HTTP"))
	require.Error(t, err)
}

func TestStartThrottleHttpPreservesOtherDefaultRouteSettings(t *testing.T) {
	api := new(httpApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigatewayv2.UpdateStageInput) bool {
		require.Equal(t, "http-1", aws.ToString(p.ApiId))
		require.NotNil(t, p.DefaultRouteSettings)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingRateLimit)
		require.Equal(t, float64(1), *p.DefaultRouteSettings.ThrottlingRateLimit)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingBurstLimit)
		require.Equal(t, int32(1), *p.DefaultRouteSettings.ThrottlingBurstLimit)
		// DataTraceEnabled from the snapshot must survive Start.
		require.NotNil(t, p.DefaultRouteSettings.DataTraceEnabled)
		require.True(t, *p.DefaultRouteSettings.DataTraceEnabled)
		return true
	})).Return(&apigatewayv2.UpdateStageOutput{}, nil)
	attack := newHttpAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "http-1", StageName: "prod", ProtocolType: "HTTP",
		TargetRateLimit: 1, TargetBurstLimit: 1,
		HttpOrigDefaultRouteSettings: `{"DataTraceEnabled":true,"ThrottlingRateLimit":500,"ThrottlingBurstLimit":1000}`,
	}
	_, err := attack.Start(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopThrottleHttpRestoresSnapshot(t *testing.T) {
	api := new(httpApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigatewayv2.UpdateStageInput) bool {
		require.NotNil(t, p.DefaultRouteSettings)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingRateLimit)
		require.Equal(t, float64(500), *p.DefaultRouteSettings.ThrottlingRateLimit)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingBurstLimit)
		require.Equal(t, int32(1000), *p.DefaultRouteSettings.ThrottlingBurstLimit)
		return true
	})).Return(&apigatewayv2.UpdateStageOutput{}, nil)
	attack := newHttpAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "http-1", StageName: "prod", ProtocolType: "HTTP",
		HadOriginalThrottleSettings:  true,
		OriginalRateLimit:            500,
		OriginalBurstLimit:           1000,
		HttpOrigDefaultRouteSettings: `{"ThrottlingRateLimit":500,"ThrottlingBurstLimit":1000}`,
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopThrottleHttpResetsToAccountDefaultWhenNoOriginalSettings(t *testing.T) {
	// AWS v2 has no documented way to clear throttle fields once set
	// (https://github.com/hashicorp/terraform-provider-aws/issues/30373). Workaround: when there was no
	// original throttle, restore to account-default values (10000 rps / 5000 burst) so traffic is
	// effectively un-throttled, even though the stage's DefaultRouteSettings now carries explicit values.
	api := new(httpApiMock)
	api.On("UpdateStage", mock.Anything, mock.MatchedBy(func(p *apigatewayv2.UpdateStageInput) bool {
		require.NotNil(t, p.DefaultRouteSettings)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingRateLimit)
		require.Equal(t, float64(10000), *p.DefaultRouteSettings.ThrottlingRateLimit)
		require.NotNil(t, p.DefaultRouteSettings.ThrottlingBurstLimit)
		require.Equal(t, int32(5000), *p.DefaultRouteSettings.ThrottlingBurstLimit)
		return true
	})).Return(&apigatewayv2.UpdateStageOutput{}, nil)
	attack := newHttpAttack(api)
	state := ApiGatewayStageThrottleAttackState{
		ApiId: "http-1", StageName: "prod", ProtocolType: "HTTP",
		HadOriginalThrottleSettings:  false,
		HttpOrigDefaultRouteSettings: "", // stage originally had no DefaultRouteSettings
	}
	_, err := attack.Stop(context.Background(), &state)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}
