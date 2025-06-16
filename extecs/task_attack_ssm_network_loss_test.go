// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_NetworkLoss_Description(t *testing.T) {
	action := NewEcsTaskNetworkLossAction()
	assert.NotNil(t, action)

	describe := action.Describe()
	assert.NotNil(t, describe)
}

func Test_getEcsTaskNetworkLossParameters(t *testing.T) {
	id := uuid.New()
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: id,
		Config: map[string]interface{}{
			"duration":   60000,
			"percentage": 90,
			"ip":         []interface{}{"1.1.1.1", "1.1.1.2/32"},
		},
	}

	params, err := getEcsTaskNetworkLossParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 5)
	assert.Equal(t, "60", params["DurationSeconds"][0])
	assert.Equal(t, "90", params["LossPercent"][0])
	assert.Equal(t, "1.1.1.1,1.1.1.2/32", params["Sources"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])

	var fisActionState fisActionState
	err = json.Unmarshal([]byte(params[fisActionStateParameterName][0]), &fisActionState)
	assert.NoError(t, err)

	assert.Equal(t, id.String(), fisActionState.Id)
	assert.Equal(t, 1, fisActionState.CallCount)
	assert.NotZero(t, fisActionState.CallTime)
}

func Test_getEcsTaskNetworkLatencyParameters_defaults(t *testing.T) {
	id := uuid.New()
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: id,
		Config: map[string]interface{}{
			"duration":   10000,
			"percentage": 10,
		},
	}

	params, err := getEcsTaskNetworkLossParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 5)
	assert.Equal(t, "10", params["DurationSeconds"][0])
	assert.Equal(t, "10", params["LossPercent"][0])
	assert.Equal(t, "0.0.0.0/0", params["Sources"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])

	var fisActionState fisActionState
	err = json.Unmarshal([]byte(params[fisActionStateParameterName][0]), &fisActionState)
	assert.NoError(t, err)

	assert.Equal(t, id.String(), fisActionState.Id)
	assert.Equal(t, 1, fisActionState.CallCount)
	assert.NotZero(t, fisActionState.CallTime)
}

func Test_getEcsTaskNetworkLatencyParameters_invalidSources(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"duration":   10000,
			"percentage": 10,
			"ip":         []interface{}{"foo*"},
		},
	}

	_, err := getEcsTaskNetworkLossParameters(req)
	assert.Error(t, err)
}

func Test_getEcsTaskNetworkLatencyParameters_invalidDuration(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"duration": 46801_000,
		},
	}

	_, err := getEcsTaskNetworkLossParameters(req)
	assert.Error(t, err)
}
