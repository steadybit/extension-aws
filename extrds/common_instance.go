// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	rdsInstanceTargetId = "com.steadybit.extension_aws.rds.instance"
)

type RdsInstanceAttackState struct {
	DBInstanceIdentifier string
	Account              string
	ForceFailover        bool
}

type rdsDBInstanceApi interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
	StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

func convertInstanceAttackState(request action_kit_api.PrepareActionRequestBody, state *RdsInstanceAttackState) error {
	instanceId := request.Target.Attributes["aws.rds.instance.id"]
	if len(instanceId) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.rds.instance.id' target attribute.", nil)
	}

	account := request.Target.Attributes["aws.account"]
	if len(account) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil)
	}

	state.Account = account[0]
	state.DBInstanceIdentifier = instanceId[0]
	state.ForceFailover = extutil.ToBool(request.Config["forceFailover"])
	return nil
}

func defaultInstanceClientProvider(account string) (rdsDBInstanceApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccount.AwsConfig), nil
}
