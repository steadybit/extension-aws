// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extelasticache

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/stretchr/testify/mock"
)

type elasticacheReplicationGroupApiMock struct {
	mock.Mock
}

func (m *elasticacheReplicationGroupApiMock) TestFailover(ctx context.Context, params *elasticache.TestFailoverInput, optFns ...func(*elasticache.Options)) (*elasticache.TestFailoverOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticache.TestFailoverOutput), args.Error(1)
}

func (m *elasticacheReplicationGroupApiMock) DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, opts ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*elasticache.DescribeReplicationGroupsOutput), args.Error(1)
}

type tagClientMock struct {
	mock.Mock
}

func (m *tagClientMock) GetResources(ctx context.Context, params *resourcegroupstaggingapi.GetResourcesInput, optFns ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resourcegroupstaggingapi.GetResourcesOutput), args.Error(1)
}
