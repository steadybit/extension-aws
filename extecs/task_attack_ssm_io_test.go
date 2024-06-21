// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getEcsTaskStressIoParameters(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration": 3333,
			"workers":  1,
			"percent":  99,
		},
	}

	params, err := getEcsTaskStressIoParameters(req)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"DurationSeconds": {"3"},
		"Workers":         {"1"},
		"Percent":         {"99"},
	}, params)
}
