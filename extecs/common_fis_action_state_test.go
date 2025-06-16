// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUpdateFisActionState_initial(t *testing.T) {
	id := uuid.NewString()
	initialFisActionStateParameter := newFisActionStateParameter(id)
	var initialFisActionState fisActionState
	err := json.Unmarshal([]byte(initialFisActionStateParameter), &initialFisActionState)
	assert.NoError(t, err)
	assert.Equal(t, id, initialFisActionState.Id)
	assert.Equal(t, 1, initialFisActionState.CallCount)
	assert.NotZero(t, initialFisActionState.CallTime)

	parametersToUpdate := map[string][]string{
		fisActionStateParameterName: {initialFisActionStateParameter},
	}
	err = updateFisActionStateParameter(parametersToUpdate)
	assert.NoError(t, err)

	var updatedFisActionState fisActionState
	err = json.Unmarshal([]byte(parametersToUpdate[fisActionStateParameterName][0]), &updatedFisActionState)
	assert.NoError(t, err)
	assert.Equal(t, initialFisActionState.Id, updatedFisActionState.Id)
	assert.Equal(t, 2, updatedFisActionState.CallCount)
	assert.LessOrEqual(t, initialFisActionState.CallTime, updatedFisActionState.CallTime)
}
