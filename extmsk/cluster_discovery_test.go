// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extmsk

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extConfig "github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

type mskClusterApiMock struct {
	mock.Mock
}

func (m *mskClusterApiMock) RebootBroker(ctx context.Context, params *kafka.RebootBrokerInput, opts ...func(*kafka.Options)) (*kafka.RebootBrokerOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kafka.RebootBrokerOutput), args.Error(1)
}

func (m *mskClusterApiMock) ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kafka.ListClustersV2Output), args.Error(1)
}

func (m *mskClusterApiMock) ListNodes(ctx context.Context, params *kafka.ListNodesInput, optFns ...func(*kafka.Options)) (*kafka.ListNodesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kafka.ListNodesOutput), args.Error(1)
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

func TestGetAllMskClusters(t *testing.T) {
	// Given
	mockedApi := new(mskClusterApiMock)
	mockedClusterReturnValue := kafka.ListClustersV2Output{
		ClusterInfoList: []types.Cluster{
			{
				ClusterName:    aws.String("dev-test"),
				ClusterArn:     aws.String("cluster-arn"),
				CurrentVersion: aws.String("version"),
				State:          types.ClusterStateActive,
				Provisioned: &types.Provisioned{
					BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
						BrokerAZDistribution: types.BrokerAZDistributionDefault,
						InstanceType:         aws.String("instance"),
						StorageInfo: &types.StorageInfo{
							EbsStorageInfo: &types.EBSStorageInfo{
								VolumeSize: aws.Int32(int32(1.000)),
								ProvisionedThroughput: &types.ProvisionedThroughput{
									Enabled:          aws.Bool(true),
									VolumeThroughput: aws.Int32(int32(300)),
								},
							},
						},
					},
					CurrentBrokerSoftwareInfo: &types.BrokerSoftwareInfo{
						KafkaVersion: aws.String("5.5.3"),
					},
				},
			},
		},
		NextToken:      nil,
		ResultMetadata: middleware.Metadata{},
	}

	mockedNodeReturnValue := kafka.ListNodesOutput{
		NodeInfoList: []types.NodeInfo{
			{
				ZookeeperNodeInfo: &types.ZookeeperNodeInfo{ZookeeperVersion: aws.String("5.5")},
				InstanceType:      aws.String("instance"),
				BrokerNodeInfo:    &types.BrokerNodeInfo{BrokerId: aws.Float64(1), CurrentBrokerSoftwareInfo: &types.BrokerSoftwareInfo{KafkaVersion: aws.String("5.5.3")}},
				NodeARN:           aws.String("node-arn"),
			},
		},
	}

	mockedApi.On("ListClustersV2", mock.Anything, mock.Anything).Return(&mockedClusterReturnValue, nil)
	mockedApi.On("ListNodes", mock.Anything, mock.Anything).Return(&mockedNodeReturnValue, nil)

	tagApi := new(tagClientMock)
	tags := resourcegroupstaggingapi.GetResourcesOutput{
		ResourceTagMappingList: []tagtypes.ResourceTagMapping{
			{
				ResourceARN: extutil.Ptr("node-arn"),
				Tags: []tagtypes.Tag{
					{
						Key:   extutil.Ptr("Example"),
						Value: extutil.Ptr("Tag123"),
					},
				},
			},
		},
	}
	tagApi.On("GetResources", mock.Anything, mock.Anything, mock.Anything).Return(&tags, nil)

	// When
	targets, err := getAllMskClusters(context.Background(), mockedApi, tagApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "Example",
				Values: []string{"Tag123"},
			},
		},
	})

	// Then
	fmt.Println(targets[0].Attributes)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))
	target := targets[0]
	assert.Equal(t, mskBrokerTargetId, target.TargetType)
	assert.Equal(t, "dev-test-1", target.Label)
	assert.Equal(t, "node-arn", target.Id)
	assert.Equal(t, 15, len(target.Attributes))
	assert.Equal(t, []string{"ACTIVE"}, target.Attributes["aws.msk.cluster.state"])
	assert.Equal(t, []string{"300"}, target.Attributes["aws.msk.cluster.broker.ebs-throughput"])
	assert.Equal(t, []string{"1"}, target.Attributes["aws.msk.cluster.broker.id"])
	assert.Equal(t, []string{"1"}, target.Attributes["aws.msk.cluster.broker.ebs-storage"])
	assert.Equal(t, []string{"instance"}, target.Attributes["aws.msk.cluster.broker.instance-type"])
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"5.5.3"}, target.Attributes["aws.msk.cluster.broker.kafka-version"])
	assert.Equal(t, []string{"5.5"}, target.Attributes["aws.msk.cluster.broker.zookeeper-version"])
	assert.Equal(t, []string{"node-arn"}, target.Attributes["aws.msk.cluster.broker.arn"])
	assert.Equal(t, []string{"cluster-arn"}, target.Attributes["aws.msk.cluster.arn"])
	assert.Equal(t, []string{"Tag123"}, target.Attributes["aws.msk.cluster.broker.label.example"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
}

func TestGetAllMskClustersWithPagination(t *testing.T) {
	// Given
	mockedApi := new(mskClusterApiMock)

	withMarkerCluster := mock.MatchedBy(func(arg *kafka.ListClustersV2Input) bool {
		return arg.NextToken != nil
	})
	withoutMarkerCluster := mock.MatchedBy(func(arg *kafka.ListClustersV2Input) bool {
		return arg.NextToken == nil
	})
	mockedApi.On("ListClustersV2", mock.Anything, withoutMarkerCluster).Return(discovery_kit_api.Ptr(kafka.ListClustersV2Output{
		NextToken: discovery_kit_api.Ptr("marker"),
		ClusterInfoList: []types.Cluster{
			{
				ClusterName:    aws.String("dev-test"),
				ClusterArn:     aws.String("arn1"),
				CurrentVersion: aws.String("version"),
				State:          types.ClusterStateActive,
				Provisioned: &types.Provisioned{
					BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
						BrokerAZDistribution: types.BrokerAZDistributionDefault,
						InstanceType:         aws.String("instance"),
						StorageInfo: &types.StorageInfo{
							EbsStorageInfo: &types.EBSStorageInfo{
								VolumeSize: aws.Int32(int32(1.000)),
								ProvisionedThroughput: &types.ProvisionedThroughput{
									Enabled:          aws.Bool(true),
									VolumeThroughput: aws.Int32(int32(300)),
								},
							},
						},
					},
					CurrentBrokerSoftwareInfo: &types.BrokerSoftwareInfo{
						KafkaVersion: aws.String("5.5.3"),
					},
				},
			},
		},
	}), nil)
	mockedApi.On("ListClustersV2", mock.Anything, withMarkerCluster).Return(discovery_kit_api.Ptr(kafka.ListClustersV2Output{
		ClusterInfoList: []types.Cluster{
			{
				ClusterName:    aws.String("dev-test"),
				ClusterArn:     aws.String("arn2"),
				CurrentVersion: aws.String("version"),
				State:          types.ClusterStateActive,
				Provisioned: &types.Provisioned{
					BrokerNodeGroupInfo: &types.BrokerNodeGroupInfo{
						BrokerAZDistribution: types.BrokerAZDistributionDefault,
						InstanceType:         aws.String("instance"),
						StorageInfo: &types.StorageInfo{
							EbsStorageInfo: &types.EBSStorageInfo{
								VolumeSize: aws.Int32(int32(1.000)),
								ProvisionedThroughput: &types.ProvisionedThroughput{
									Enabled:          aws.Bool(true),
									VolumeThroughput: aws.Int32(int32(300)),
								},
							},
						},
					},
					CurrentBrokerSoftwareInfo: &types.BrokerSoftwareInfo{
						KafkaVersion: aws.String("5.5.3"),
					},
				},
			},
		},
	}), nil)

	mockedApi.On("ListNodes", context.Background(), &kafka.ListNodesInput{ClusterArn: aws.String("arn1")}).Return(discovery_kit_api.Ptr(kafka.ListNodesOutput{
		NodeInfoList: []types.NodeInfo{
			{
				BrokerNodeInfo: &types.BrokerNodeInfo{
					BrokerId: aws.Float64(1),
				},
				NodeARN: aws.String("node-arn1"),
			},
		},
	}), nil)
	mockedApi.On("ListNodes", context.Background(), &kafka.ListNodesInput{ClusterArn: aws.String("arn2")}).Return(discovery_kit_api.Ptr(kafka.ListNodesOutput{
		NodeInfoList: []types.NodeInfo{
			{
				BrokerNodeInfo: &types.BrokerNodeInfo{
					BrokerId: aws.Float64(1),
				},
				NodeARN: aws.String("node-arn2"),
			},
		},
	}), nil)

	tagApi := new(tagClientMock)
	tags := resourcegroupstaggingapi.GetResourcesOutput{
		ResourceTagMappingList: []tagtypes.ResourceTagMapping{
			{
				ResourceARN: extutil.Ptr("node-arn1"),
				Tags: []tagtypes.Tag{
					{
						Key:   extutil.Ptr("Example"),
						Value: extutil.Ptr("Tag123"),
					},
				},
			},
		},
	}
	tagApi.On("GetResources", mock.Anything, mock.Anything, mock.Anything).Return(&tags, nil)

	// When
	targets, err := getAllMskClusters(context.Background(), mockedApi, tagApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	fmt.Println(targets)
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(targets))
	assert.Equal(t, "node-arn1", targets[0].Id)
	assert.Equal(t, "node-arn2", targets[1].Id)
}

func TestGetAllMskClustersError(t *testing.T) {
	// Given
	mockedApi := new(mskClusterApiMock)

	mockedApi.On("ListClustersV2", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	tagApi := new(tagClientMock)

	// When
	_, err := getAllMskClusters(context.Background(), mockedApi, tagApi, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, err.Error(), "expected")
}
