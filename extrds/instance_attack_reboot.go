// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func RegisterRdsAttackHandlers() {
	utils.RegisterHttpHandler("/rds/instance/attack/reboot", utils.GetterAsHandler(getRebootInstanceAttackDescription))
	utils.RegisterHttpHandler("/rds/instance/attack/reboot/prepare", prepareInstanceReboot)
	utils.RegisterHttpHandler("/rds/instance/attack/reboot/start", startInstanceReboot)

}

func getRebootInstanceAttackDescription() attack_kit_api.AttackDescription {
	return attack_kit_api.AttackDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "reboot instance",
		Description: "Reboot a single database instance",
		Version:     "1.0.0",
		Icon:        attack_kit_api.Ptr(rdsIcon),
		TargetType:  rdsTargetId,
		Category:    attack_kit_api.Resource,
		TimeControl: attack_kit_api.INSTANTANEOUS,
		Parameters:  []attack_kit_api.AttackParameter{},
		Prepare: attack_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/prepare",
		},
		Start: attack_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/start",
		},
	}
}

type InstanceRebootState struct {
	DBInstanceIdentifier string
}

func prepareInstanceReboot(w http.ResponseWriter, _ *http.Request, body []byte) {
	state, err := PrepareInstanceReboot(body)
	if err != nil {
		utils.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

func PrepareInstanceReboot(body []byte) (*InstanceRebootState, *attack_kit_api.AttackKitError) {
	var request attack_kit_api.PrepareAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse request body", err))
	}

	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, attack_kit_api.Ptr(utils.ToError("Target is missing the 'aws.rds.instance.id' tag.", nil))
	}

	return attack_kit_api.Ptr(InstanceRebootState{
		DBInstanceIdentifier: instanceId[0],
	}), nil
}

func startInstanceReboot(w http.ResponseWriter, r *http.Request, body []byte) {
	client := rds.NewFromConfig(utils.AwsConfig)
	state, err := StartInstanceReboot(r.Context(), body, client)
	if err != nil {
		utils.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

type RdsRebootDBInstanceApi interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

func StartInstanceReboot(ctx context.Context, body []byte, client RdsRebootDBInstanceApi) (*InstanceRebootState, *attack_kit_api.AttackKitError) {
	var request attack_kit_api.StartAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse request body", err))
	}

	var state InstanceRebootState
	err = utils.DecodeAttackState(request.State, &state)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to parse attack state", err))
	}

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.RebootDBInstance(ctx, &input)
	if err != nil {
		return nil, attack_kit_api.Ptr(utils.ToError("Failed to execute database instance reboot", err))
	}

	return &state, nil
}
