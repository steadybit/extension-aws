// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
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

func prepareInstanceReboot(w http.ResponseWriter, r *http.Request, body []byte) {
	var request attack_kit_api.PrepareAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		utils.WriteError(w, "Failed to parse request body", err)
		return
	}

	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		utils.WriteError(w, "Target is missing the 'aws.rds.instance.id' tag.", err)
		return
	}

	utils.WriteAttackState(w, InstanceRebootState{
		DBInstanceIdentifier: instanceId[0],
	})
}

func startInstanceReboot(w http.ResponseWriter, r *http.Request, body []byte) {
	var request attack_kit_api.StartAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		utils.WriteError(w, "Failed to parse request body", err)
		return
	}

	var state InstanceRebootState
	err = utils.DecodeAttackState(request.State, &state)
	if err != nil {
		utils.WriteError(w, "Failed to parse attack state", err)
		return
	}

	client := rds.NewFromConfig(utils.AwsConfig)

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.RebootDBInstance(r.Context(), &input)
	if err != nil {
		utils.WriteError(w, "Failed to execute database instance reboot", err)
		return
	}

	utils.WriteAttackState(w, state)
}
