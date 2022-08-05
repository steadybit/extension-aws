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
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterRdsAttackHandlers() {
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot", exthttp.GetterAsHandler(getRebootInstanceAttackDescription))
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot/prepare", prepareInstanceReboot)
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot/start", startInstanceReboot)
}

func getRebootInstanceAttackDescription() attack_kit_api.AttackDescription {
	return attack_kit_api.AttackDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "reboot instance",
		Description: "Reboot a single database instance",
		Version:     "1.0.0",
		Icon:        extutil.Ptr(rdsIcon),
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
		exthttp.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

func PrepareInstanceReboot(body []byte) (*InstanceRebootState, *extension_kit.ExtensionError) {
	var request attack_kit_api.PrepareAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.rds.instance.id' tag.", nil))
	}

	return extutil.Ptr(InstanceRebootState{
		DBInstanceIdentifier: instanceId[0],
	}), nil
}

func startInstanceReboot(w http.ResponseWriter, r *http.Request, body []byte) {
	client := rds.NewFromConfig(utils.AwsConfig)
	state, err := StartInstanceReboot(r.Context(), body, client)
	if err != nil {
		exthttp.WriteError(w, *err)
	} else {
		utils.WriteAttackState(w, *state)
	}
}

type RdsRebootDBInstanceApi interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

func StartInstanceReboot(ctx context.Context, body []byte, client RdsRebootDBInstanceApi) (*InstanceRebootState, *extension_kit.ExtensionError) {
	var request attack_kit_api.StartAttackRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state InstanceRebootState
	err = utils.DecodeAttackState(request.State, &state)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse attack state", err))
	}

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.RebootDBInstance(ctx, &input)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to execute database instance reboot", err))
	}

	return &state, nil
}
