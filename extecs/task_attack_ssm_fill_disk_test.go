// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extecs

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_getEcsTaskFillDiskParameters(t *testing.T) {
	req := action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{
			"duration": 1000,
			"percent":  77,
		},
	}

	params, err := getEcsTaskFillDiskParameters(req)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"DurationSeconds": {"1"},
		"Percent":         {"77"},
	}, params)
}
