// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"sort"
)

type FisExperimentAction struct {
}

type FisExperimentState struct {
	Account      string
	ExperimentId string
	TemplateId   string
	LastSummary  string
	ExecutionId  uuid.UUID
}

func NewFisExperimentAction() action_kit_sdk.Action[FisExperimentState] {
	return FisExperimentAction{}
}

// Make sure FisExperimentAction implements all required interfaces
var _ action_kit_sdk.Action[FisExperimentState] = (*FisExperimentAction)(nil)
var _ action_kit_sdk.ActionWithStatus[FisExperimentState] = (*FisExperimentAction)(nil)
var _ action_kit_sdk.ActionWithStop[FisExperimentState] = (*FisExperimentAction)(nil)

func (f FisExperimentAction) NewEmptyState() FisExperimentState {
	return FisExperimentState{}
}

func (f FisExperimentAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          FisActionId,
		Label:       "AWS FIS Experiment",
		Description: "Start an AWS FIS experiment",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(fisIcon),
		TargetType:  extutil.Ptr(fisTargetId),
		TimeControl: action_kit_api.Internal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Estimated duration",
				Description:  extutil.Ptr("The estimated total duration of your FIS experiment."),
				Name:         "duration",
				Type:         action_kit_api.Duration,
				Advanced:     extutil.Ptr(false),
				DefaultValue: extutil.Ptr("60s"),
			},
		},
		TargetSelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label:       "by template-id",
				Description: extutil.Ptr("Find fis-template by template-id"),
				Query:       "aws.fis.experiment.template.id=\"\"",
			},
			{
				Label:       "by template-name",
				Description: extutil.Ptr("Find fis-template by template-name"),
				Query:       "aws.fis.experiment.template.name=\"\"",
			},
		}),
		Prepare: action_kit_api.MutatingEndpointReference{},
		Start:   action_kit_api.MutatingEndpointReference{},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("5s"),
		}),
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (f FisExperimentAction) Prepare(ctx context.Context, state *FisExperimentState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	templateId := request.Target.Attributes["aws.fis.experiment.template.id"]
	if templateId == nil || len(templateId) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'aws.fis.experiment.template.id' target attribute.", nil)
	}

	account := request.Target.Attributes["aws.account"]
	if account == nil || len(account) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil))
	}

	state.TemplateId = templateId[0]
	state.Account = account[0]
	state.ExecutionId = request.ExecutionId
	return nil, nil
}

func (f FisExperimentAction) Start(ctx context.Context, state *FisExperimentState) (*action_kit_api.StartResult, error) {
	return startExperiment(ctx, state, func(account string) (FisStartExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
}

type FisStartExperimentClient interface {
	StartExperiment(ctx context.Context, params *fis.StartExperimentInput, optFns ...func(*fis.Options)) (*fis.StartExperimentOutput, error)
}

func startExperiment(ctx context.Context, state *FisExperimentState, clientProvider func(account string) (FisStartExperimentClient, error)) (*action_kit_api.StartResult, error) {
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
	return nil, nil
}

func (f FisExperimentAction) Status(ctx context.Context, state *FisExperimentState) (*action_kit_api.StatusResult, error) {
	return statusExperiment(ctx, state, func(account string) (FisStatusExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
}

type FisStatusExperimentClient interface {
	GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
}

func statusExperiment(ctx context.Context, state *FisExperimentState, clientProvider func(account string) (FisStatusExperimentClient, error)) (*action_kit_api.StatusResult, error) {
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

	return &result, nil
}

func actionSummary(experiment *types.Experiment) string {
	actionNames := make([]string, 0, len(experiment.Actions))
	for actionName := range experiment.Actions {
		actionNames = append(actionNames, actionName)
	}
	sort.Strings(actionNames)

	summary := "FIS experiment summary:\n"
	for _, actionName := range actionNames {
		action := experiment.Actions[actionName]
		status := "unknown"
		statusReason := ""
		if action.State != nil {
			status = string(action.State.Status)
			if action.State.Reason != nil {
				statusReason = " (" + *action.State.Reason + ")"
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

func (f FisExperimentAction) Stop(ctx context.Context, state *FisExperimentState) (*action_kit_api.StopResult, error) {
	return stopExperiment(ctx, state, func(account string) (FisStopExperimentClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return fis.NewFromConfig(awsAccount.AwsConfig), nil
	})
}

func stopExperiment(ctx context.Context, state *FisExperimentState, clientProvider func(account string) (FisStopExperimentClient, error)) (*action_kit_api.StopResult, error) {
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
	return nil, nil

}
