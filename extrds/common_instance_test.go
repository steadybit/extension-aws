// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/stretchr/testify/mock"
)

type rdsDBInstanceApiMock struct {
	mock.Mock
}

func (m *rdsDBInstanceApiMock) RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, opts ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
	args := m.Called(ctx, params, opts)
	return nil, args.Error(1)
}
func (m *rdsDBInstanceApiMock) StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error) {
	args := m.Called(ctx, params, optFns)
	return nil, args.Error(1)
}

func (m *rdsDBInstanceApiMock) StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error) {
	args := m.Called(ctx, params, optFns)
	return nil, args.Error(1)
}

func (m *rdsDBInstanceApiMock) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rds.DescribeDBInstancesOutput), args.Error(1)
}

type zoneMock struct {
	mock.Mock
}

func (m *zoneMock) GetZone(awsAccountNumber string, awsZone string) *types.AvailabilityZone {
	args := m.Called(awsAccountNumber, awsZone)
	return args.Get(0).(*types.AvailabilityZone)
}
