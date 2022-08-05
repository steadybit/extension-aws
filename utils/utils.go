// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package utils

import (
	"github.com/mitchellh/mapstructure"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"net/http"
)

func WriteAttackState[T any](w http.ResponseWriter, state T) {
	err, encodedState := EncodeAttackState(state)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode attack state", err))
	} else {
		exthttp.WriteBody(w, attack_kit_api.AttackStateAndMessages{
			State: encodedState,
		})
	}
}

func EncodeAttackState[T any](attackState T) (error, attack_kit_api.AttackState) {
	var result attack_kit_api.AttackState
	err := mapstructure.Decode(attackState, &result)
	return err, result
}

func DecodeAttackState[T any](attackState attack_kit_api.AttackState, result *T) error {
	return mapstructure.Decode(attackState, result)
}
