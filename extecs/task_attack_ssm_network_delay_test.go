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

func Test_NetworkLatency_Description(t *testing.T) {
	action := NewEcsTaskNetworkDelayAction()
	assert.NotNil(t, action)

	describe := action.Describe()
	assert.NotNil(t, describe)
}

func Test_getEcsTaskNetworkDelayParameters(t *testing.T) {
	id := uuid.New()
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: id,
		Config: map[string]interface{}{
			"duration":           60000,
			"networkDelay":       100,
			"networkDelayJitter": true,
			"ip":                 []interface{}{"1.1.1.1", "1.1.1.2/32"},
		},
	}

	params, err := getEcsTaskNetworkDelayParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 6)
	assert.Equal(t, "60", params["DurationSeconds"][0])
	assert.Equal(t, "100", params["DelayMilliseconds"][0])
	assert.Equal(t, "30", params["JitterMilliseconds"][0])
	assert.Equal(t, "1.1.1.1,1.1.1.2/32", params["Sources"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])

	var fisActionState fisActionState
	err = json.Unmarshal([]byte(params[fisActionStateParameterName][0]), &fisActionState)
	assert.NoError(t, err)

	assert.Equal(t, id.String(), fisActionState.Id)
	assert.Equal(t, 1, fisActionState.CallCount)
	assert.NotZero(t, fisActionState.CallTime)
}

func Test_getEcsTaskNetworkDelayParameters_defaults(t *testing.T) {
	id := uuid.New()
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: id,
		Config: map[string]interface{}{
			"duration":           10000,
			"networkDelay":       500,
			"networkDelayJitter": false,
		},
	}

	params, err := getEcsTaskNetworkDelayParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 6)
	assert.Equal(t, "10", params["DurationSeconds"][0])
	assert.Equal(t, "500", params["DelayMilliseconds"][0])
	assert.Equal(t, "0", params["JitterMilliseconds"][0])
	assert.Equal(t, "0.0.0.0/0", params["Sources"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])

	var fisActionState fisActionState
	err = json.Unmarshal([]byte(params[fisActionStateParameterName][0]), &fisActionState)
	assert.NoError(t, err)

	assert.Equal(t, id.String(), fisActionState.Id)
	assert.Equal(t, 1, fisActionState.CallCount)
	assert.NotZero(t, fisActionState.CallTime)
}

func Test_getEcsTaskNetworkDelayParameters_invalidSources(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"duration":           10000,
			"networkDelay":       500,
			"networkDelayJitter": false,
			"ip":                 []interface{}{"foo*"},
		},
	}

	_, err := getEcsTaskNetworkDelayParameters(req)
	assert.Error(t, err)
}

func Test_getEcsTaskNetworkDelayParameters_invalidDuration(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"duration": 43201_000,
		},
	}

	_, err := getEcsTaskNetworkDelayParameters(req)
	assert.Error(t, err)
}
