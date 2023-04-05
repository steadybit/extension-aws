// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type RdsInstanceAttack struct {
}

type RdsInstanceAttackState struct {
	DBInstanceIdentifier string
	Account              string
}

func NewRdsInstanceAttack() action_kit_sdk.Action[RdsInstanceAttackState] {
	return RdsInstanceAttack{}
}

// Make sure FisExperimentAction implements all required interfaces
var _ action_kit_sdk.Action[RdsInstanceAttackState] = (*RdsInstanceAttack)(nil)

func (f RdsInstanceAttack) NewEmptyState() RdsInstanceAttackState {
	return RdsInstanceAttackState{}
}

func (f RdsInstanceAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "Reboot Instance",
		Description: "Reboot a single database instance",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(rdsIcon),
		TargetType:  extutil.Ptr(rdsTargetId),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.Instantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
		Prepare:     action_kit_api.MutatingEndpointReference{},
		Start:       action_kit_api.MutatingEndpointReference{},
	}
}

func (f RdsInstanceAttack) Prepare(_ context.Context, state *RdsInstanceAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if instanceId == nil || len(instanceId) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.rds.instance.id' target attribute.", nil))
	}

	account := request.Target.Attributes["aws.account"]
	if account == nil || len(account) == 0 {
		return nil, extutil.Ptr(extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil))
	}

	state.Account = account[0]
	state.DBInstanceIdentifier = instanceId[0]

	return nil, nil
}

func (f RdsInstanceAttack) Start(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StartResult, error) {
	return startAttack(ctx, state, func(account string) (RdsRebootDBInstanceClient, error) {
		awsAccount, err := utils.Accounts.GetAccount(account)
		if err != nil {
			return nil, err
		}
		return rds.NewFromConfig(awsAccount.AwsConfig), nil
	})
}

type RdsRebootDBInstanceClient interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

func startAttack(ctx context.Context, state *RdsInstanceAttackState, clientProvider func(account string) (RdsRebootDBInstanceClient, error)) (*action_kit_api.StartResult, error) {
	client, err := clientProvider(state.Account)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError(fmt.Sprintf("Failed to initialize RDS client for AWS account %s", state.Account), err))
	}

	input := rds.RebootDBInstanceInput{
		DBInstanceIdentifier: &state.DBInstanceIdentifier,
	}
	_, err = client.RebootDBInstance(ctx, &input)
	if err != nil {
		return nil, extutil.Ptr(extension_kit.ToError("Failed to execute database instance reboot", err))
	}

	return nil, nil
}
