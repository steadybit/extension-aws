// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package utils

import (
	"encoding/json"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"io/ioutil"
	"net/http"
	"runtime/debug"
)

func RegisterHttpHandler(path string, handler func(w http.ResponseWriter, r *http.Request, body []byte)) {
	http.Handle(path, PanicRecovery(LogRequest(handler)))
}

func GetterAsHandler[T any](handler func() T) func(w http.ResponseWriter, r *http.Request, body []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		WriteBody(w, handler())
	}
}

func PanicRecovery(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("Panic: %v\n %s", err, string(debug.Stack()))
				WriteError(w, ToError("Internal Server Error", nil))
			}
		}()
		next(w, r)
	}
}

func LogRequest(next func(w http.ResponseWriter, r *http.Request, body []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, bodyReadErr := ioutil.ReadAll(r.Body)
		if bodyReadErr != nil {
			http.Error(w, bodyReadErr.Error(), http.StatusBadRequest)
			return
		}

		if len(body) > 0 {
			log.Info().Msgf("%s %s with body %s", r.Method, r.URL, body)
		} else {
			log.Info().Msgf("%s %s", r.Method, r.URL)
		}

		next(w, r, body)
	}
}

func WriteError(w http.ResponseWriter, err attack_kit_api.AttackKitError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	json.NewEncoder(w).Encode(err)
}

func ToError(title string, err error) attack_kit_api.AttackKitError {
	var response attack_kit_api.AttackKitError
	if err != nil {
		response = attack_kit_api.AttackKitError{Title: title, Detail: discovery_kit_api.Ptr(err.Error())}
	} else {
		response = attack_kit_api.AttackKitError{Title: title}
	}
	return response
}

func WriteBody(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}

func WriteAttackState[T any](w http.ResponseWriter, state T) {
	err, encodedState := EncodeAttackState(state)
	if err != nil {
		WriteError(w, ToError("Failed to encode attack state", err))
	} else {
		WriteBody(w, attack_kit_api.AttackStateAndMessages{
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
