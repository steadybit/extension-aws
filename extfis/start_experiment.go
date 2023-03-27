// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"sort"
)

func RegisterFisActionHandlers() {
	exthttp.RegisterHttpHandler("/fis/experiment/action", exthttp.GetterAsHandler(getFisExperimentActionDescription))
	exthttp.RegisterHttpHandler("/fis/experiment/action/prepare", prepareExperiment)
	exthttp.RegisterHttpHandler("/fis/experiment/action/start", startExperiment)
	exthttp.RegisterHttpHandler("/fis/experiment/action/status", statusExperiment)
	exthttp.RegisterHttpHandler("/fis/experiment/action/stop", stopExperiment)
}

func getFisExperimentActionDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fisActionId,
		Label:       "AWS FIS Experiment",
		Description: "Start an AWS FIS experiment",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(fisIcon),
		TargetType:  extutil.Ptr(fisTargetId),
		TimeControl: action_kit_api.Internal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Description:  extutil.Ptr("The total duration of your FIS experiment."),
				Name:         "duration",
				Type:         action_kit_api.Duration,
				Advanced:     extutil.Ptr(false),
				DefaultValue: extutil.Ptr("60s"),
			},
		},
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/fis/experiment/action/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/fis/experiment/action/start",
		},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			Method:       "POST",
			Path:         "/fis/experiment/action/status",
			CallInterval: extutil.Ptr("5s"),
		}),
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/fis/experiment/action/stop",
		}),
	}
}

type FisExperimentState struct {
	Account      string
	ExperimentId string
	TemplateId   string
	LastSummary  string
	ExecutionId  uuid.UUID
}

func prepareExperiment(w http.ResponseWriter, _ *http.Request, body []byte) {
	state, extKitErr := PrepareExperiment(body)
	if extKitErr != nil {
		exthttp.WriteError(w, *extKitErr)
		return
	}

	var convertedState action_kit_api.ActionState
	err := extconversion.Convert(*state, &convertedState)
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state", err))
		return
	}

	exthttp.WriteBody(w, extutil.Ptr(action_kit_api.PrepareResult{
		State: convertedState,
	}))
}

func PrepareExperiment(body []byte) (*FisExperimentState, *extension_kit.ExtensionError) {
	var request action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	templateId := request.Target.Attributes["aws.fis.experiment.template.id"]
	if templateId == nil || len(templateId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.fis.experiment.template.id' target attribute.", nil))
	}

	account := request.Target.Attributes["aws.account"]
	if account == nil || len(account) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil))
	}

	return extutil.Ptr(FisExperimentState{
		Account:     account[0],
		TemplateId:  templateId[0],
		ExecutionId: request.ExecutionId,
	}), nil
}

func startExperiment(w http.ResponseWriter, r *http.Request, body []byte) {
	state, err := StartExperiment(r.Context(), body, func(account string) (FisStartExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		var convertedState action_kit_api.ActionState
		err := extconversion.Convert(state, &convertedState)
		if err != nil {
			exthttp.WriteError(w, extension_kit.ToError("Failed to encode action state", err))
		} else {
			exthttp.WriteBody(w, action_kit_api.PrepareResult{
				State: convertedState,
			})
		}
	}
}

type FisStartExperimentClient interface {
	StartExperiment(ctx context.Context, params *fis.StartExperimentInput, optFns ...func(*fis.Options)) (*fis.StartExperimentOutput, error)
}

func StartExperiment(ctx context.Context, body []byte, clientProvider func(account string) (FisStartExperimentClient, error)) (*FisExperimentState, *extension_kit.ExtensionError) {
	var request action_kit_api.StartActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state FisExperimentState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse action state", err))
	}

	client, err := clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize FIS client for AWS account %s", state.Account), err))
	}

	clientToken, err := uuid.NewRandom()
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to generate a random client-token.", err))
	}

	input := fis.StartExperimentInput{
		ExperimentTemplateId: &state.TemplateId,
		ClientToken:          extutil.Ptr(clientToken.String()),
		Tags:                 map[string]string{"steadybit-execution-id": state.ExecutionId.String()},
	}
	response, err := client.StartExperiment(ctx, &input)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to start fis experiment", err))
	}

	state.ExperimentId = *response.Experiment.Id
	return &state, nil
}

func statusExperiment(w http.ResponseWriter, r *http.Request, body []byte) {
	result, err := StatusExperiment(r.Context(), body, func(account string) (FisStatusExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		exthttp.WriteBody(w, result)
	}
}

type FisStatusExperimentClient interface {
	GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
}

func StatusExperiment(ctx context.Context, body []byte, clientProvider func(account string) (FisStatusExperimentClient, error)) (*action_kit_api.StatusResult, *extension_kit.ExtensionError) {
	var request action_kit_api.ActionStatusRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state FisExperimentState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to decode action state", err))
	}

	client, err := clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to initialize FIS client for AWS account %s", err))
	}

	experiment, err := client.GetExperiment(ctx, extutil.Ptr(fis.GetExperimentInput{
		Id: &state.ExperimentId,
	}))
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to fetch experiment", err))
	}

	result := action_kit_api.StatusResult{}
	summary := actionSummary(experiment.Experiment)
	if summary != state.LastSummary {
		state.LastSummary = summary
		result.Messages = &action_kit_api.Messages{
			action_kit_api.Message{Level: extutil.Ptr(action_kit_api.Info), Message: summary},
		}
	}

	result.Completed = false
	if experiment.Experiment.State != nil {
		if experiment.Experiment.State.Status == types.ExperimentStatusCompleted || experiment.Experiment.State.Status == types.ExperimentStatusStopped {
			result.Completed = true
		} else if experiment.Experiment.State.Status == types.ExperimentStatusFailed {
			result.Error = extutil.Ptr(action_kit_api.ActionKitError{
				Title:  "FIS Experiment failed",
				Detail: experiment.Experiment.State.Reason,
				Status: extutil.Ptr(action_kit_api.Failed),
			})
			result.Completed = true
		}
	}

	var convertedState action_kit_api.ActionState
	err = extconversion.Convert(state, &convertedState)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to encode action state", err))
	}
	result.State = &convertedState

	return &result, nil
}

func actionSummary(experiment *types.Experiment) string {
	actionNames := make([]string, 0, len(experiment.Actions))
	for actionName := range experiment.Actions {
		actionNames = append(actionNames, actionName)
	}
	sort.Strings(actionNames)

	summary := ""
	for _, actionName := range actionNames {
		action := experiment.Actions[actionName]
		status := "unknown"
		statusReason := ""
		if action.State != nil {
			status = string(action.State.Status)
			if action.State.Reason != nil {
				statusReason = *action.State.Reason
			}
		}
		summary = summary + fmt.Sprintf("%s: %s%s\n", actionName, status, statusReason)
	}
	return summary
}

type FisStopExperimentClient interface {
	GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
	StopExperiment(ctx context.Context, params *fis.StopExperimentInput, optFns ...func(*fis.Options)) (*fis.StopExperimentOutput, error)
}

func stopExperiment(w http.ResponseWriter, r *http.Request, body []byte) {
	result, err := StopExperiment(r.Context(), body, func(account string) (FisStopExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		exthttp.WriteBody(w, result)
	}
}
func StopExperiment(ctx context.Context, body []byte, clientProvider func(account string) (FisStopExperimentClient, error)) (*action_kit_api.StopResult, *extension_kit.ExtensionError) {
	var request action_kit_api.ActionStatusRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state FisExperimentState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to decode action state", err))
	}

	client, err := clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to initialize FIS client for AWS account %s", err))
	}

	experiment, err := client.GetExperiment(ctx, extutil.Ptr(fis.GetExperimentInput{
		Id: &state.ExperimentId,
	}))
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to fetch experiment", err))
	}

	status := experiment.Experiment.State.Status
	if status == types.ExperimentStatusPending || status == types.ExperimentStatusInitiating || status == types.ExperimentStatusRunning {
		_, err := client.StopExperiment(ctx, extutil.Ptr(fis.StopExperimentInput{Id: &state.ExperimentId}))
		if err != nil {
			return nil, extutil.Ptr(extension_kit.ToError("Failed to stop experiment", err))
		}
		log.Debug().Msgf("Stopped Experiment.")
	} else {
		log.Debug().Msgf("Experiment already in state %s.", status)
	}
	return extutil.Ptr(action_kit_api.StopResult{}), nil

}
