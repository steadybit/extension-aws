// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	rdsClusterTargetId = "com.steadybit.extension_aws.rds.cluster"
)

type RdsClusterAttackState struct {
	DBClusterIdentifier string
	Account             string
	Region              string
}

type rdsDBClusterApi interface {
	FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
	DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

func convertClusterAttackState(request action_kit_api.PrepareActionRequestBody, state *RdsClusterAttackState) error {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DBClusterIdentifier = extutil.MustHaveValue(request.Target.Attributes, "aws.rds.cluster.id")[0]
	return nil
}
func defaultClusterClientProvider(account string, region string) (rdsDBClusterApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccess.AwsConfig), nil
}
