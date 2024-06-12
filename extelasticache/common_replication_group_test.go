// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extelasticache

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/stretchr/testify/mock"
)

type replicationGroupApiMock struct {
	mock.Mock
}

func (m *replicationGroupApiMock) TestFailover(ctx context.Context, params *elasticache.TestFailoverInput, optFns ...func(*elasticache.Options)) (*elasticache.TestFailoverOutput, error) {
	args := m.Called(ctx, params, optFns)
	return nil, args.Error(1)
}

func (m *replicationGroupApiMock) DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticache.DescribeReplicationGroupsOutput), args.Error(1)
}
