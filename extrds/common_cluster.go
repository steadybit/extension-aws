// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	rdsClusterTargetId = "com.steadybit.extension_aws.rds.cluster"
)

type RdsClusterAttackState struct {
	DBClusterIdentifier string
	Account             string
	Region              string
	DiscoveredByRole    *string
}

type rdsDBClusterApi interface {
	FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
	rds.DescribeDBClustersAPIClient
}

func convertClusterAttackState(request action_kit_api.PrepareActionRequestBody, state *RdsClusterAttackState) error {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")
	state.DBClusterIdentifier = extutil.MustHaveValue(request.Target.Attributes, "aws.rds.cluster.id")[0]
	return nil
}
func defaultClusterClientProvider(account string, region string, role *string) (rdsDBClusterApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return rds.NewFromConfig(awsAccess.AwsConfig), nil
}
