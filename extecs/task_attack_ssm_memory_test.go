// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getEcsTaskStressMemoryParameters(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration": 60000,
			"workers":  2,
			"percent":  25,
		},
	}

	params, err := getEcsTaskStressMemoryParameters(req)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"DurationSeconds": {"60"},
		"Workers":         {"2"},
		"Percent":         {"25"},
	}, params)
}
