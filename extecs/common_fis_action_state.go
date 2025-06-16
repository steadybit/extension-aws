// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extecs

import (
	"encoding/json"
	"fmt"
	"time"
)

const fisActionStateParameterName = "FISActionState"

// Field order is important. Do not change!
type fisActionState struct {
	Id        string `json:"id"`
	CallTime  int64  `json:"callTime"`
	CallCount int    `json:"callCount"`
}

func newFisActionStateParameter(id string) string {
	fisActionState, _ := json.Marshal(fisActionState{
		Id:        id,
		CallCount: 1,
		CallTime:  time.Now().Unix(),
	})
	return string(fisActionState)
}

func updateFisActionStateParameter(parameters map[string][]string) error {
	if len(parameters[fisActionStateParameterName]) == 0 || len(parameters[fisActionStateParameterName][0]) == 0 {
		return fmt.Errorf("missing FISActionState Parameter")
	}

	var fisActionState fisActionState
	err := json.Unmarshal([]byte(parameters[fisActionStateParameterName][0]), &fisActionState)
	if err != nil {
		return err
	}

	fisActionState.CallCount = fisActionState.CallCount + 1
	fisActionState.CallTime = time.Now().Unix()
	f, err := json.Marshal(fisActionState)
	if err != nil {
		return err
	}

	parameters[fisActionStateParameterName][0] = string(f)
	return nil
}
