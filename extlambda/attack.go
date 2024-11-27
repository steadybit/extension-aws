/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extlambda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
)

type lambdaAction struct {
	description    action_kit_api.ActionDescription
	configProvider func(request action_kit_api.PrepareActionRequestBody) (*FailureInjectionConfig, error)
	clientProvider func(account string, region string) (ssmApi, error)
}

type ssmApi interface {
	PutParameter(ctx context.Context, s *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, s *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	AddTagsToResource(ctx context.Context, s *ssm.AddTagsToResourceInput, optFns ...func(*ssm.Options)) (*ssm.AddTagsToResourceOutput, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[LambdaActionState] = (*lambdaAction)(nil)
var _ action_kit_sdk.ActionWithStop[LambdaActionState] = (*lambdaAction)(nil)

type FailureInjectionConfig struct {
	FailureMode  string    `json:"failureMode"`
	Rate         float64   `json:"rate"`
	IsEnabled    bool      `json:"isEnabled"`
	StatusCode   *int      `json:"statusCode,omitempty"`
	MinLatency   *int      `json:"minLatency,omitempty"`
	MaxLatency   *int      `json:"maxLatency,omitempty"`
	ExceptionMsg *string   `json:"exceptionMsg,omitempty"`
	Denylist     *[]string `json:"denylist,omitempty"`
	DiskSpace    *int      `json:"diskSpace,omitempty"`
}

type LambdaActionState struct {
	Account       string                  `json:"account"`
	Region        string                  `json:"region"`
	Param         string                  `json:"param"`
	Config        *FailureInjectionConfig `json:"config"`
	ExperimentKey *string                 `json:"experimentKey"`
	ExecutionId   *int                    `json:"executionId"`
}

func (a *lambdaAction) Describe() action_kit_api.ActionDescription {
	return a.description
}

func (a *lambdaAction) NewEmptyState() LambdaActionState {
	return LambdaActionState{}
}

func (a *lambdaAction) Prepare(_ context.Context, state *LambdaActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	failureInjectionParam := request.Target.Attributes["aws.lambda.failure-injection-param"]
	if len(failureInjectionParam) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'aws.lambda.failure-injection-param' attribute. Did you wrap the lambda with https://github.com/steadybit/failure-lambda ?", nil)
	}

	config, err := a.configProvider(request)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create config", err)
	}

	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.Param = failureInjectionParam[0]
	state.ExperimentKey = request.ExecutionContext.ExperimentKey
	state.ExecutionId = request.ExecutionContext.ExecutionId
	state.Config = config
	return nil, nil
}

func (a *lambdaAction) Start(ctx context.Context, state *LambdaActionState) (*action_kit_api.StartResult, error) {
	value, err := json.Marshal(state.Config)
	if err != nil {
		return nil, extension_kit.ToError("Failed to convert ssm parameter", err)
	}

	client, err := a.clientProvider(state.Account, state.Region)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize lambda client for AWS account %s", state.Account), err)
	}

	_, err = client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:        extutil.Ptr(state.Param),
		Value:       extutil.Ptr(string(value)),
		Type:        types.ParameterTypeString,
		DataType:    extutil.Ptr("text"),
		Description: extutil.Ptr(fmt.Sprintf("lambda failure injection config - set by steadybit experiment %s / execution %d", *state.ExperimentKey, *state.ExecutionId)),
		Overwrite:   extutil.Ptr(false),
	})
	if err != nil {
		var pae *types.ParameterAlreadyExists
		if errors.As(err, &pae) {
			return nil, extension_kit.ToError("Failed to put ssm parameter. This might be caused by trying to run multiple parallel failure injections on the same lambda function, which is not supported.", err)
		}
		return nil, extension_kit.ToError("Failed to put ssm parameter", err)
	}

	_, _ = client.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceId:   extutil.Ptr(state.Param),
		ResourceType: types.ResourceTypeForTaggingParameter,
		Tags:         []types.Tag{{Key: extutil.Ptr("created-by"), Value: extutil.Ptr("steadybit")}},
	})
	return nil, nil
}

func (a *lambdaAction) Stop(ctx context.Context, state *LambdaActionState) (*action_kit_api.StopResult, error) {
	client, err := a.clientProvider(state.Account, state.Region)
	if err != nil {
		return nil, extension_kit.ToError("Failed to create ssm client", err)
	}

	_, err = client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: extutil.Ptr(state.Param),
	})
	if err != nil {
		var notFound *types.ParameterNotFound
		if !errors.As(err, &notFound) {
			return nil, extension_kit.ToError("Failed to delete ssm parameter", err)
		}
	}

	return nil, nil
}

func defaultClientProvider(account string, region string) (ssmApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account, region)
	if err != nil {
		return nil, err
	}
	client := ssm.NewFromConfig(awsAccount.AwsConfig)
	return client, nil
}
