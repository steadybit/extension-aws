// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
)

const (
	rdsClusterTargetId = "com.github.steadybit.extension_aws.rds.cluster"
)

type RdsClusterAttackState struct {
	DBClusterIdentifier string
	Account             string
}

type rdsDBClusterApi interface {
	FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
	DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

func convertClusterAttackState(request action_kit_api.PrepareActionRequestBody, state *RdsClusterAttackState) error {
	clusterId := request.Target.Attributes["aws.rds.cluster.id"]
	if len(clusterId) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.rds.cluster.id' target attribute.", nil)
	}

	account := request.Target.Attributes["aws.account"]
	if len(account) == 0 {
		return extension_kit.ToError("Target is missing the 'aws.account' target attribute.", nil)
	}

	state.Account = account[0]
	state.DBClusterIdentifier = clusterId[0]
	return nil
}
func defaultClusterClientProvider(account string) (rdsDBClusterApi, error) {
	awsAccount, err := utils.Accounts.GetAccount(account)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccount.AwsConfig), nil
}
