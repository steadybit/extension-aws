// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type natGatewayApiMock struct {
	mock.Mock
}

func (m *natGatewayApiMock) DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeNatGatewaysOutput), args.Error(1)
}

func (m *natGatewayApiMock) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeSubnetsOutput), args.Error(1)
}

func natGatewayTestAccount() *utils.AwsAccess {
	return &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "eu-central-1",
		AssumeRole:    aws.String("arn:aws:iam::42:role/extension-aws-role"),
	}
}

func TestGetAllNatGateways(t *testing.T) {
	// Given
	mockedApi := new(natGatewayApiMock)
	mockedApi.On("DescribeNatGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeNatGatewaysOutput{
		NatGateways: []types.NatGateway{
			{
				NatGatewayId:     aws.String("nat-0ce34a3dbfee3eada"),
				State:            types.NatGatewayStateAvailable,
				ConnectivityType: types.ConnectivityTypePublic,
				SubnetId:         aws.String("subnet-0465f80961ecf4f43"),
				VpcId:            aws.String("vpc-0d72f9311b54adcc0"),
				NatGatewayAddresses: []types.NatGatewayAddress{
					{PublicIp: aws.String("3.3.3.3")},
					{PublicIp: aws.String("1.1.1.1")},
				},
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("demo-nat-gateway-0")},
					{Key: aws.String("Env"), Value: aws.String("demo")},
				},
			},
		},
	}, nil)
	mockedApi.On("DescribeSubnets", mock.Anything, mock.MatchedBy(func(in *ec2.DescribeSubnetsInput) bool {
		return len(in.SubnetIds) == 0 &&
			len(in.Filters) == 1 &&
			aws.ToString(in.Filters[0].Name) == "subnet-id" &&
			assert.ObjectsAreEqual([]string{"subnet-0465f80961ecf4f43"}, in.Filters[0].Values)
	})).Return(&ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{SubnetId: aws.String("subnet-0465f80961ecf4f43"), AvailabilityZone: aws.String("eu-central-1a")},
		},
	}, nil)

	// When
	targets, err := getAllNatGateways(context.Background(), mockedApi, natGatewayTestAccount())

	// Then
	require.NoError(t, err)
	require.Len(t, targets, 1)
	target := targets[0]
	assert.Equal(t, natGatewayTargetType, target.TargetType)
	assert.Equal(t, "nat-0ce34a3dbfee3eada", target.Id)
	assert.Equal(t, "demo-nat-gateway-0", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"eu-central-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"eu-central-1a"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"subnet-0465f80961ecf4f43"}, target.Attributes["aws.nat-gateway.subnet"])
	assert.Equal(t, []string{"vpc-0d72f9311b54adcc0"}, target.Attributes["aws.nat-gateway.vpc"])
	assert.Equal(t, []string{"1"}, target.Attributes["aws.vpc.nat-gateway-count-in-vpc"])
	assert.Equal(t, []string{"public"}, target.Attributes["aws.nat-gateway.connectivity-type"])
	assert.Equal(t, []string{"available"}, target.Attributes["aws.nat-gateway.state"])
	assert.Equal(t, []string{"1.1.1.1", "3.3.3.3"}, target.Attributes["aws.nat-gateway.elastic-ips"])
	assert.Equal(t, []string{"demo"}, target.Attributes["aws.nat-gateway.label.env"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
	mockedApi.AssertExpectations(t)
}

// A deleted NAT gateway stays visible in DescribeNatGateways for about an hour and still
// references its (possibly already deleted) subnet. Its subnet must not be looked up, so
// that the AZ mapping of the remaining gateways is not affected.
func TestGetAllNatGatewaysIgnoresSubnetsOfDeletedGateways(t *testing.T) {
	// Given
	mockedApi := new(natGatewayApiMock)
	mockedApi.On("DescribeNatGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeNatGatewaysOutput{
		NatGateways: []types.NatGateway{
			{
				NatGatewayId: aws.String("nat-0e2f53a89e316af53"),
				State:        types.NatGatewayStateAvailable,
				SubnetId:     aws.String("subnet-07fef4d517001da01"),
				VpcId:        aws.String("vpc-0ef050003b8d6d8f3"),
			},
			{
				NatGatewayId: aws.String("nat-04466c8c4d8ef8fce"),
				State:        types.NatGatewayStateDeleted,
				SubnetId:     aws.String("subnet-0c6ea4eb7dbe6e7dc"),
				VpcId:        aws.String("vpc-0ef050003b8d6d8f3"),
			},
			{
				NatGatewayId: aws.String("nat-0000000000deleting"),
				State:        types.NatGatewayStateDeleting,
				SubnetId:     aws.String("subnet-0000000000deleting"),
				VpcId:        aws.String("vpc-0ef050003b8d6d8f3"),
			},
		},
	}, nil)
	mockedApi.On("DescribeSubnets", mock.Anything, mock.MatchedBy(func(in *ec2.DescribeSubnetsInput) bool {
		return len(in.Filters) == 1 &&
			assert.ObjectsAreEqual([]string{"subnet-07fef4d517001da01"}, in.Filters[0].Values)
	})).Return(&ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{SubnetId: aws.String("subnet-07fef4d517001da01"), AvailabilityZone: aws.String("eu-central-1b")},
		},
	}, nil)

	// When
	targets, err := getAllNatGateways(context.Background(), mockedApi, natGatewayTestAccount())

	// Then
	require.NoError(t, err)
	require.Len(t, targets, 2, "deleted gateways are not reported, deleting ones still are")
	assert.Equal(t, "nat-0e2f53a89e316af53", targets[0].Id)
	assert.Equal(t, []string{"eu-central-1b"}, targets[0].Attributes["aws.zone"])
	assert.Equal(t, []string{"2"}, targets[0].Attributes["aws.vpc.nat-gateway-count-in-vpc"])
	assert.Equal(t, "nat-0000000000deleting", targets[1].Id)
	_, hasZone := targets[1].Attributes["aws.zone"]
	assert.False(t, hasZone)
	mockedApi.AssertExpectations(t)
}

// When the subnet lookup returns fewer subnets than requested (e.g. a subnet was deleted
// between the two calls), the remaining gateways keep their AZ and the unknown one is
// reported without it.
func TestGetAllNatGatewaysToleratesMissingSubnets(t *testing.T) {
	// Given
	mockedApi := new(natGatewayApiMock)
	mockedApi.On("DescribeNatGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeNatGatewaysOutput{
		NatGateways: []types.NatGateway{
			{NatGatewayId: aws.String("nat-a"), State: types.NatGatewayStateAvailable, SubnetId: aws.String("subnet-a"), VpcId: aws.String("vpc-1")},
			{NatGatewayId: aws.String("nat-b"), State: types.NatGatewayStateAvailable, SubnetId: aws.String("subnet-b"), VpcId: aws.String("vpc-1")},
		},
	}, nil)
	mockedApi.On("DescribeSubnets", mock.Anything, mock.MatchedBy(func(in *ec2.DescribeSubnetsInput) bool {
		return len(in.Filters) == 1 &&
			assert.ObjectsAreEqual([]string{"subnet-a", "subnet-b"}, in.Filters[0].Values)
	})).Return(&ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{SubnetId: aws.String("subnet-a"), AvailabilityZone: aws.String("eu-central-1a")},
		},
	}, nil)

	// When
	targets, err := getAllNatGateways(context.Background(), mockedApi, natGatewayTestAccount())

	// Then
	require.NoError(t, err)
	require.Len(t, targets, 2)
	assert.Equal(t, []string{"eu-central-1a"}, targets[0].Attributes["aws.zone"])
	_, hasZone := targets[1].Attributes["aws.zone"]
	assert.False(t, hasZone)
	mockedApi.AssertExpectations(t)
}

func TestGetAllNatGatewaysStillReportsTargetsWhenSubnetLookupFails(t *testing.T) {
	// Given
	mockedApi := new(natGatewayApiMock)
	mockedApi.On("DescribeNatGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeNatGatewaysOutput{
		NatGateways: []types.NatGateway{
			{NatGatewayId: aws.String("nat-a"), State: types.NatGatewayStateAvailable, SubnetId: aws.String("subnet-a"), VpcId: aws.String("vpc-1")},
		},
	}, nil)
	mockedApi.On("DescribeSubnets", mock.Anything, mock.Anything).Return(nil, errors.New("UnauthorizedOperation"))

	// When
	targets, err := getAllNatGateways(context.Background(), mockedApi, natGatewayTestAccount())

	// Then
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, []string{"subnet-a"}, targets[0].Attributes["aws.nat-gateway.subnet"])
	_, hasZone := targets[0].Attributes["aws.zone"]
	assert.False(t, hasZone)
}

func TestGetAllNatGatewaysSkipsSubnetLookupWhenNoGatewayIsActive(t *testing.T) {
	// Given
	mockedApi := new(natGatewayApiMock)
	mockedApi.On("DescribeNatGateways", mock.Anything, mock.Anything).Return(&ec2.DescribeNatGatewaysOutput{
		NatGateways: []types.NatGateway{
			{NatGatewayId: aws.String("nat-a"), State: types.NatGatewayStateDeleted, SubnetId: aws.String("subnet-a"), VpcId: aws.String("vpc-1")},
		},
	}, nil)

	// When
	targets, err := getAllNatGateways(context.Background(), mockedApi, natGatewayTestAccount())

	// Then
	require.NoError(t, err)
	assert.Empty(t, targets)
	mockedApi.AssertNotCalled(t, "DescribeSubnets", mock.Anything, mock.Anything)
}
