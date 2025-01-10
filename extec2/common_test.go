// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/extension-aws/utils"
	"github.com/stretchr/testify/mock"
)

type zoneMock struct {
	mock.Mock
}

func (m *zoneMock) GetZones(account *utils.AwsAccess, ctx context.Context, updateCache bool) []types.AvailabilityZone {
	args := m.Called(account, ctx, updateCache)
	return args.Get(0).([]types.AvailabilityZone)
}

func (m *zoneMock) GetZone(awsAccountNumber string, awsZone string, region string) *types.AvailabilityZone {
	args := m.Called(awsAccountNumber, awsZone, region)
	return args.Get(0).(*types.AvailabilityZone)
}
