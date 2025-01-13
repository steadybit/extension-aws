// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/extension-aws/utils"
	"github.com/stretchr/testify/mock"
)

type ec2UtilMock struct {
	mock.Mock
}

func (m *ec2UtilMock) GetZones(account *utils.AwsAccess) []types.AvailabilityZone {
	args := m.Called(account)
	return args.Get(0).([]types.AvailabilityZone)
}
func (m *ec2UtilMock) GetZone(awsAccountNumber string, awsZone string, region string) *types.AvailabilityZone {
	args := m.Called(awsAccountNumber, awsZone, region)
	return args.Get(0).(*types.AvailabilityZone)
}
func (m *ec2UtilMock) GetVpcName(awsAccountNumber string, region string, vpcId string) string {
	args := m.Called(awsAccountNumber, region, vpcId)
	return args.Get(0).(string)
}
