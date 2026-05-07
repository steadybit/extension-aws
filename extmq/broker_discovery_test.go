// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extmq

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/aws/aws-sdk-go-v2/service/mq/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mqApiMock struct {
	mock.Mock
}

func (m *mqApiMock) ListBrokers(ctx context.Context, params *mq.ListBrokersInput, optFns ...func(*mq.Options)) (*mq.ListBrokersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mq.ListBrokersOutput), args.Error(1)
}

func (m *mqApiMock) DescribeBroker(ctx context.Context, params *mq.DescribeBrokerInput, optFns ...func(*mq.Options)) (*mq.DescribeBrokerOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mq.DescribeBrokerOutput), args.Error(1)
}

func (m *mqApiMock) RebootBroker(ctx context.Context, params *mq.RebootBrokerInput, optFns ...func(*mq.Options)) (*mq.RebootBrokerOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mq.RebootBrokerOutput), args.Error(1)
}

func TestGetAllBrokers(t *testing.T) {
	api := new(mqApiMock)
	api.On("ListBrokers", mock.Anything, mock.Anything).Return(&mq.ListBrokersOutput{
		BrokerSummaries: []types.BrokerSummary{
			{BrokerId: aws.String("b-1234")},
		},
	}, nil)
	api.On("DescribeBroker", mock.Anything, mock.MatchedBy(func(p *mq.DescribeBrokerInput) bool {
		return aws.ToString(p.BrokerId) == "b-1234"
	})).Return(&mq.DescribeBrokerOutput{
		BrokerArn:               aws.String("arn:aws:mq:us-east-1:42:broker:my-broker:b-1234"),
		BrokerId:                aws.String("b-1234"),
		BrokerName:              aws.String("my-broker"),
		EngineType:              types.EngineTypeRabbitmq,
		EngineVersion:           aws.String("3.13.7"),
		DeploymentMode:          types.DeploymentModeSingleInstance,
		HostInstanceType:        aws.String("mq.t3.micro"),
		PubliclyAccessible:      aws.Bool(true),
		AutoMinorVersionUpgrade: aws.Bool(false),
		SubnetIds:               []string{"subnet-b", "subnet-a"},
		StorageType:             types.BrokerStorageTypeEbs,
		EncryptionOptions:       &types.EncryptionOptions{UseAwsOwnedKey: aws.Bool(true)},
		AuthenticationStrategy:  types.AuthenticationStrategySimple,
		MaintenanceWindowStartTime: &types.WeeklyStartTime{
			DayOfWeek: types.DayOfWeekSunday,
			TimeOfDay: aws.String("03:00"),
			TimeZone:  aws.String("UTC"),
		},
		Tags: map[string]string{"application": "Demo", "Environment": "prod"},
	}, nil)

	targets, err := getAllBrokers(context.Background(), api, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    aws.String("arn:role"),
		TagFilters:    []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	tgt := targets[0]
	assert.Equal(t, brokerTargetId, tgt.TargetType)
	assert.Equal(t, "my-broker", tgt.Label)
	assert.Equal(t, []string{"b-1234"}, tgt.Attributes["aws.mq.broker.id"])
	assert.Equal(t, []string{"my-broker"}, tgt.Attributes["aws.mq.broker.name"])
	assert.Equal(t, []string{"RABBITMQ"}, tgt.Attributes["aws.mq.broker.engine-type"])
	assert.Equal(t, []string{"3.13.7"}, tgt.Attributes["aws.mq.broker.engine-version"])
	assert.Equal(t, []string{"SINGLE_INSTANCE"}, tgt.Attributes["aws.mq.broker.deployment-mode"])
	assert.Equal(t, []string{"mq.t3.micro"}, tgt.Attributes["aws.mq.broker.host-instance-type"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.mq.broker.publicly-accessible"])
	assert.Equal(t, []string{"false"}, tgt.Attributes["aws.mq.broker.auto-minor-version-upgrade"])
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, tgt.Attributes["aws.mq.broker.subnets"])
	assert.Equal(t, []string{"EBS"}, tgt.Attributes["aws.mq.broker.storage-type"])
	assert.Equal(t, []string{"true"}, tgt.Attributes["aws.mq.broker.encryption.use-aws-owned-key"])
	assert.Equal(t, []string{"SIMPLE"}, tgt.Attributes["aws.mq.broker.authentication-strategy"])
	assert.Equal(t, []string{"SUNDAY 03:00 UTC"}, tgt.Attributes["aws.mq.broker.maintenance-window"])
	assert.Equal(t, []string{"Demo"}, tgt.Attributes["aws.mq.broker.label.application"])
	assert.Equal(t, []string{"prod"}, tgt.Attributes["aws.mq.broker.label.environment"])
	assert.Equal(t, []string{"arn:role"}, tgt.Attributes["extension-aws.discovered-by-role"])
}

func TestGetAllBrokersTagFilterMismatch(t *testing.T) {
	api := new(mqApiMock)
	api.On("ListBrokers", mock.Anything, mock.Anything).Return(&mq.ListBrokersOutput{
		BrokerSummaries: []types.BrokerSummary{{BrokerId: aws.String("b-1")}},
	}, nil)
	api.On("DescribeBroker", mock.Anything, mock.Anything).Return(&mq.DescribeBrokerOutput{
		BrokerArn:  aws.String("arn:b1"),
		BrokerId:   aws.String("b-1"),
		BrokerName: aws.String("b1"),
		Tags:       map[string]string{"application": "Other"},
	}, nil)
	targets, err := getAllBrokers(context.Background(), api, &utils.AwsAccess{
		AccountNumber: "42", Region: "us-east-1",
		TagFilters: []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(targets))
}

func TestGetAllBrokersError(t *testing.T) {
	api := new(mqApiMock)
	api.On("ListBrokers", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	_, err := getAllBrokers(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.EqualError(t, err, "expected")
}
