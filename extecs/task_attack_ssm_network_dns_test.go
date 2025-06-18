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

func Test_NetworkDns_Description(t *testing.T) {
	action := NewEcsTaskNetworkDnsAction()
	assert.NotNil(t, action)

	describe := action.Describe()
	assert.NotNil(t, describe)
}

func Test_getEcsTaskNetworkDnsParameters(t *testing.T) {
	id := uuid.New()
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: id,
		Config: map[string]interface{}{
			"duration": 60000,
			"dnsPort":  "53",
		},
	}

	params, err := getEcsTaskNetworkDnsParameters(req)
	assert.NoError(t, err)
	assert.Len(t, params, 6)
	assert.Equal(t, "60", params["DurationSeconds"][0])
	assert.Equal(t, "udp", params["Protocol"][0])
	assert.Equal(t, "53", params["Port"][0])
	assert.Equal(t, "egress", params["TrafficType"][0])
	assert.Equal(t, "True", params["InstallDependencies"][0])

	var fisActionState fisActionState
	err = json.Unmarshal([]byte(params[fisActionStateParameterName][0]), &fisActionState)
	assert.NoError(t, err)

	assert.Equal(t, id.String(), fisActionState.Id)
	assert.Equal(t, 1, fisActionState.CallCount)
	assert.NotZero(t, fisActionState.CallTime)
}

func Test_getEcsTaskNetworkDnsParameters_invalidDuration(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		ExecutionId: uuid.New(),
		Config: map[string]interface{}{
			"duration": 43201_000,
		},
	}

	_, err := getEcsTaskNetworkDnsParameters(req)
	assert.Error(t, err)
}
