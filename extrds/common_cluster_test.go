// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/stretchr/testify/mock"
)

type rdsDBClusterApiMock struct {
	mock.Mock
}

func (m *rdsDBClusterApiMock) FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, opts ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error) {
	args := m.Called(ctx, params, opts)
	return nil, args.Error(1)
}

func (m *rdsDBClusterApiMock) DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rds.DescribeDBClustersOutput), args.Error(1)
}
