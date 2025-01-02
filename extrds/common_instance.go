// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	rdsInstanceTargetId = "com.steadybit.extension_aws.rds.instance"
)

type RdsInstanceAttackState struct {
	DBInstanceIdentifier string
	Account              string
	Region               string
	ForceFailover        bool
}

type rdsDBInstanceApi interface {
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
	StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	rds.DescribeDBInstancesAPIClient
}

func convertInstanceAttackState(request action_kit_api.PrepareActionRequestBody, state *RdsInstanceAttackState) error {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DBInstanceIdentifier = extutil.MustHaveValue(request.Target.Attributes, "aws.rds.instance.id")[0]
	state.ForceFailover = extutil.ToBool(request.Config["force-failover"])
	return nil
}

func defaultInstanceClientProvider(account string, region string) (rdsDBInstanceApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccess.AwsConfig), nil
}
