// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getEcsTaskStressCpuParameters(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration": 1000,
			"workers":  0,
			"cpuLoad":  100,
		},
	}

	params, err := getEcsTaskStressCpuParameters(req)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"DurationSeconds": {"1"},
		"CPU":             {"0"},
		"LoadPercent":     {"100"},
	}, params)
}
