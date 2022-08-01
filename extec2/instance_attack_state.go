// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extec2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func RegisterEc2AttackHandlers() {
	utils.RegisterHttpHandler("/ec2/instance/attack/state", utils.GetterAsHandler(getInstanceStateAttackDescription))
	utils.RegisterHttpHandler("/ec2/instance/attack/state/prepare", prepareInstanceStateChange)
	utils.RegisterHttpHandler("/ec2/instance/attack/state/start", startInstanceStateChange)
}

func getInstanceStateAttackDescription() attack_kit_api.AttackDescription {
	return attack_kit_api.AttackDescription{
		Id:          fmt.Sprintf("%s.state", ec2TargetId),
		Label:       "change instance state",
		Description: "Reboot, terminate, stop or hibernate EC2 instances",
		Version:     "1.0.0",
		Icon:        attack_kit_api.Ptr(ec2Icon),
		TargetType:  ec2TargetId,
		Category:    attack_kit_api.State,
		TimeControl: attack_kit_api.INSTANTANEOUS,
		Parameters: []attack_kit_api.AttackParameter{
			{
				Name:        "action",
				Label:       "Action",
				Description: attack_kit_api.Ptr("The kind of state change operation to execute for the EC2 instances"),
				Required:    attack_kit_api.Ptr(true),
				Type:        "string",
				Options: attack_kit_api.Ptr([]attack_kit_api.ParameterOption{
					{
						Label: "Reboot",
						Value: "reboot",
					},
					{
						Label: "Stop",
						Value: "stop",
					},
					{
						Label: "Hibernate",
						Value: "hibernate",
					},
					{
						Label: "Terminate",
						Value: "terminate",
					},
				}),
			},
		},
		Prepare: attack_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/ec2/instance/attack/state/prepare",
		},
		Start: attack_kit_api.MutatingEndpointReference{
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
	state, err := PrepareInstanceStateChange(body)
	if err != nil {
		utils.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

func PrepareInstanceStateChange(body []byte) (*InstanceStateChangeState, *attack_kit_api.AttackKitError) {
	var request attack_kit_api.PrepareAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse request body", err))
	}

	instanceId := request.Target.Attributes["aws-ec2.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, attack_kit_api.Ptr(utils.ToError("Target is missing the 'aws-ec2.instance.id' tag.", nil))
	}

	action := request.Config["action"]
	if action == nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Missing attack action parameter.", nil))
	}

	return attack_kit_api.Ptr(InstanceStateChangeState{
		InstanceId: instanceId[0],
		Action:     action.(string),
	}), nil
}

func startInstanceStateChange(w http.ResponseWriter, r *http.Request, body []byte) {
	client := ec2.NewFromConfig(utils.AwsConfig)
	state, err := StartInstanceStateChange(r.Context(), body, client)
	if err != nil {
		utils.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

type Ec2InstanceStateChangeApiApi interface {
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	RebootInstances(ctx context.Context, params *ec2.RebootInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RebootInstancesOutput, error)
}

func StartInstanceStateChange(ctx context.Context, body []byte, client Ec2InstanceStateChangeApiApi) (*InstanceStateChangeState, *attack_kit_api.AttackKitError) {
	var request attack_kit_api.StartAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse request body", err))
	}

	var state InstanceStateChangeState
	err = utils.DecodeAttackState(request.State, &state)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse attack state", err))
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
			Hibernate:   attack_kit_api.Ptr(false),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "hibernate" {
		in := ec2.StopInstancesInput{
			InstanceIds: instanceIds,
			Hibernate:   attack_kit_api.Ptr(true),
		}
		_, err = client.StopInstances(ctx, &in)
	} else if state.Action == "terminate" {
		in := ec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		}
		_, err = client.TerminateInstances(ctx, &in)
	}

	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on instance '%s'", state.Action, state.InstanceId), err))
	}

	return &state, nil
}
