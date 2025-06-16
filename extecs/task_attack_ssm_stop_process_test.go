// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_StopProcess_Description(t *testing.T) {
	action := NewEcsTaskStopProcessAction()
	assert.NotNil(t, action)

	describe := action.Describe()
	assert.NotNil(t, describe)
}

func Test_getEcsTaskStopProcessParameters(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"process":  "foo",
			"graceful": true,
		},
	}

	params, err := getEcsTaskStopProcessParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 3)
	assert.Equal(t, "foo", params["ProcessName"][0])
	assert.Equal(t, "SIGTERM", params["Signal"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])
}

func Test_getEcsTaskStopProcessParameters_defaults(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"process":  "foo",
			"graceful": false,
		},
	}

	params, err := getEcsTaskStopProcessParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 3)
	assert.Equal(t, "foo", params["ProcessName"][0])
	assert.Equal(t, "SIGKILL", params["Signal"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])
}

func Test_getEcsTaskStopProcessParameters_invalidProcessName(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"process":  "foo*",
			"graceful": false,
		},
	}

	_, err := getEcsTaskStopProcessParameters(req)
	assert.Error(t, err)
}
