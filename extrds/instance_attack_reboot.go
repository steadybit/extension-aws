// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterRdsAttackHandlers() {
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot", exthttp.GetterAsHandler(getRebootInstanceAttackDescription))
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot/prepare", prepareInstanceReboot)
	exthttp.RegisterHttpHandler("/rds/instance/attack/reboot/start", startInstanceReboot)
}

func getRebootInstanceAttackDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "reboot instance",
		Description: "Reboot a single database instance",
		Version:     "1.0.0",
		Icon:        extutil.Ptr(rdsIcon),
		TargetType:  extutil.Ptr(rdsTargetId),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.Instantaneous,
		Parameters:  []action_kit_api.ActionParameter{},
		Prepare: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/prepare",
		},
		Start: action_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/start",
		},
	}
}

type InstanceRebootState struct {
	DBInstanceIdentifier string
	Account              string
}

func prepareInstanceReboot(w http.ResponseWriter, _ *http.Request, body []byte) {
	state, extKitErr := PrepareInstanceReboot(body)
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

func PrepareInstanceReboot(body []byte) (*InstanceRebootState, *extension_kit.ExtensionError) {
	var request action_kit_api.PrepareActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.rds.instance.id' target attribute.", nil))
	}

	account := request.Target.Attributes["aws.account"]
	if account == nil || len(account) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil))
	}

	return extutil.Ptr(InstanceRebootState{
		Account:              account[0],
		DBInstanceIdentifier: instanceId[0],
	}), nil
}

func startInstanceReboot(w http.ResponseWriter, r *http.Request, body []byte) {
	extKitErr := StartInstanceReboot(r.Context(), body, func(account string) (RdsRebootDBInstanceClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return rds.NewFromConfig(awsAccount.AwsConfig), nil
	})
	if extKitErr != nil {
		exthttp.WriteError(w, *extKitErr)
		return
	}

	exthttp.WriteBody(w, extutil.Ptr(action_kit_api.StartResult{}))
}

type RdsRebootDBInstanceClient interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

func StartInstanceReboot(ctx context.Context, body []byte, clientProvider func(account string) (RdsRebootDBInstanceClient, error)) *extension_kit.ExtensionError {
	var request action_kit_api.StartActionRequestBody
	err := json.Unmarshal(body, &request)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError("Failed to parse request body", err))
	}

	var state InstanceRebootState
	err = extconversion.Convert(request.State, &state)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError("Failed to parse attack state", err))
	}

	client, err := clientProvider(state.Account)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err))
	}

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.RebootDBInstance(ctx, &input)
	if err != nil {
		return extutil.Ptr(extension_kit.ToError("Failed to execute database instance reboot", err))
	}

	return nil
}
