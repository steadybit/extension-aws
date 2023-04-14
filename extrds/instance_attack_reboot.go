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

type rdsInstanceAttack struct {
	clientProvider func(account string) (rdsRebootDBInstanceApi, error)
}

// Make sure FisExperimentAction implements all required interfaces
var _ action_kit_sdk.Action[RdsInstanceAttackState] = (*rdsInstanceAttack)(nil)

type RdsInstanceAttackState struct {
	DBInstanceIdentifier string
	Account              string
}

type rdsRebootDBInstanceApi interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

func NewRdsInstanceAttack() action_kit_sdk.Action[RdsInstanceAttackState] {
	return rdsInstanceAttack{defaultClientProvider}
}

func (f rdsInstanceAttack) NewEmptyState() RdsInstanceAttackState {
	return RdsInstanceAttackState{}
}

func (f rdsInstanceAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "Reboot Instance",
		Description: "Reboot a single database instance",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(rdsIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: rdsTargetId,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label: "by rds instance id",
					Query: "aws.rds.instance.id=\"\"",
				},
			}),
		}),
		Category:    extutil.Ptr("resource"),
		TimeControl: action_kit_api.Instantaneous,
		Kind:        action_kit_api.Attack,
		Parameters:  []action_kit_api.ActionParameter{},
	}
}

func (f rdsInstanceAttack) Prepare(_ context.Context, state *RdsInstanceAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
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

func (f rdsInstanceAttack) Start(ctx context.Context, state *RdsInstanceAttackState) (*action_kit_api.StartResult, error) {
	client, err := f.clientProvider(state.Account)
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

func defaultClientProvider(account string) (rdsRebootDBInstanceApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccount.AwsConfig), nil
}
