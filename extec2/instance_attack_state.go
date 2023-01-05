// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterEc2AttackHandlers() {
	exthttp.RegisterHttpHandler("/ec2/instance/attack/state", exthttp.GetterAsHandler(getInstanceStateAttackDescription))
	exthttp.RegisterHttpHandler("/ec2/instance/attack/state/prepare", prepareInstanceStateChange)
	exthttp.RegisterHttpHandler("/ec2/instance/attack/state/start", startInstanceStateChange)
}

func getInstanceStateAttackDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.state", ec2TargetId),
		Label:       "change instance state",
		Description: "Reboot, terminate, stop or hibernate EC2 instances",
		Version:     "1.0.0",
		Icon:        extutil.Ptr(ec2Icon),
		TargetType:  extutil.Ptr(ec2TargetId),
		Category:    extutil.Ptr("state"),
		TimeControl: action_kit_api.Instantaneous,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:        "action",
				Label:       "Action",
				Description: extutil.Ptr("The kind of state change operation to execute for the EC2 instances"),
				Required:    extutil.Ptr(true),
				Type:        "string",
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Reboot",
						Value: "reboot",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Stop",
						Value: "stop",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Hibernate",
						Value: "hibernate",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Terminate",
						Value: "terminate",
					},
				}),
			},
		},
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/ec2/instance/attack/state/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/ec2/instance/attack/state/start",
		},
	}
}

type InstanceStateChangeState struct {
	InstanceId string
	Action     string
}

func prepareInstanceStateChange(w http.ResponseWriter, _ *http.Request, body []byte) {
	state, extKitErr := PrepareInstanceStateChange(body)
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

func PrepareInstanceStateChange(body []byte) (*InstanceStateChangeState, *extension_kit.ExtensionError) {
	var request action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	instanceId := request.Target.Attributes["aws-ec2.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws-ec2.instance.id' tag.", nil))
	}

	action := request.Config["action"]
	if action == nil {
		return nil, extutil.Ptr(extension_kit.ToError("Missing attack action parameter.", nil))
	}

	return extutil.Ptr(InstanceStateChangeState{
		InstanceId: instanceId[0],
		Action:     action.(string),
	}), nil
}

func startInstanceStateChange(w http.ResponseWriter, r *http.Request, body []byte) {
	client := ec2.NewFromConfig(utils.AwsConfig)
	err := StartInstanceStateChange(r.Context(), body, client)
	if err != nil {
		exthttp.WriteError(w, *err)
		return
	}

	exthttp.WriteBody(w, extutil.Ptr(action_kit_api.StartResult{}))
}

type Ec2InstanceStateChangeApiApi interface {
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error)
}

func StartInstanceStateChange(ctx context.Context, body []byte, client Ec2InstanceStateChangeApiApi) *extension_kit.ExtensionError {
	var request action_kit_api.StartActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state InstanceStateChangeState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError("Failed to parse attack state", err))
	}

	instanceIds := []string{state.InstanceId}

	if state.Action == "reboot" {
		in := ec2.RebootInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.RebootInstances(ctx, &in)
	} else if state.Action == "stop" {
		in := ec2.StopInstancesInput{
			InstanceIds: instanceIds,
			Hibernate:   extutil.Ptr(false),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "hibernate" {
		in := ec2.StopInstancesInput{
			InstanceIds: instanceIds,
			Hibernate:   extutil.Ptr(true),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "terminate" {
		in := ec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.TerminateInstances(ctx, &in)
	}

	if err != nil {
		return extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on instance '%s'", state.Action, state.InstanceId), err))
	}

	return nil
}
