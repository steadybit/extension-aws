package utils

import (
	"encoding/json"
	"github.com/mitchellh/mapstructure"
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
				ErrorLogger.Printf("Panic: %v\n %s", err, string(debug.Stack()))
				WriteError(w, "Internal Server Error", nil)
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
			InfoLogger.Printf("%s %s with body %s", r.Method, r.URL, body)
		} else {
			InfoLogger.Printf("%s %s", r.Method, r.URL)
		}

		next(w, r, body)
	}
}

func WriteError(w http.ResponseWriter, title string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(500)
	var response discovery_kit_api.DiscoveryKitError
	if err != nil {
		response = discovery_kit_api.DiscoveryKitError{Title: title, Detail: discovery_kit_api.Ptr(err.Error())}
	} else {
		response = discovery_kit_api.DiscoveryKitError{Title: title}
	}
	json.NewEncoder(w).Encode(response)
}

func WriteBody(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}

func WriteAttackState[T any](w http.ResponseWriter, state T) {
	err, encodedState := EncodeAttackState(state)
	if err != nil {
		WriteError(w, "Failed to encode attack state", err)
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
