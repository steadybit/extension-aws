// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type eksApiMock struct {
	mock.Mock
}

func (m *eksApiMock) ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eks.ListClustersOutput), args.Error(1)
}

func (m *eksApiMock) ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eks.ListNodegroupsOutput), args.Error(1)
}

func (m *eksApiMock) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eks.DescribeClusterOutput), args.Error(1)
}

func (m *eksApiMock) DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eks.DescribeNodegroupOutput), args.Error(1)
}

func (m *eksApiMock) UpdateNodegroupConfig(ctx context.Context, params *eks.UpdateNodegroupConfigInput, optFns ...func(*eks.Options)) (*eks.UpdateNodegroupConfigOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*eks.UpdateNodegroupConfigOutput), args.Error(1)
}

func TestGetAllEksClusters(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{
		Clusters: []string{"prod-cluster"},
	}, nil)
	api.On("DescribeCluster", mock.Anything, mock.MatchedBy(func(p *eks.DescribeClusterInput) bool {
		return aws.ToString(p.Name) == "prod-cluster"
	})).Return(&eks.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Arn:             aws.String("arn:aws:eks:us-east-1:42:cluster/prod-cluster"),
			Name:            aws.String("prod-cluster"),
			Version:         aws.String("1.29"),
			PlatformVersion: aws.String("eks.10"),
			Status:          types.ClusterStatusActive,
			ResourcesVpcConfig: &types.VpcConfigResponse{
				EndpointPublicAccess:  true,
				EndpointPrivateAccess: false,
				PublicAccessCidrs:     []string{"0.0.0.0/0"},
				SubnetIds:             []string{"subnet-b", "subnet-a"},
				VpcId:                 aws.String("vpc-123"),
			},
			Logging: &types.Logging{
				ClusterLogging: []types.LogSetup{
					{Enabled: aws.Bool(true), Types: []types.LogType{types.LogTypeApi, types.LogTypeAudit}},
					{Enabled: aws.Bool(false), Types: []types.LogType{types.LogTypeAuthenticator, types.LogTypeControllerManager, types.LogTypeScheduler}},
				},
			},
			EncryptionConfig: []types.EncryptionConfig{
				{Resources: []string{"secrets"}, Provider: &types.Provider{KeyArn: aws.String("kms-arn")}},
			},
			DeletionProtection: aws.Bool(true),
			Tags: map[string]string{
				"application": "Demo",
				"Environment": "prod",
			},
		},
	}, nil)

	targets, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{
		AccountNumber: "42",
		Region:        "us-east-1",
		AssumeRole:    aws.String("arn:role"),
		TagFilters:    []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	target := targets[0]
	assert.Equal(t, clusterTargetId, target.TargetType)
	assert.Equal(t, "prod-cluster", target.Label)
	assert.Equal(t, []string{"prod-cluster"}, target.Attributes["aws.eks.cluster.name"])
	// Cluster name is also surfaced under k8s.cluster-name so extension-kubernetes enrichment can join.
	assert.Equal(t, []string{"prod-cluster"}, target.Attributes["k8s.cluster-name"])
	assert.Equal(t, []string{"1.29"}, target.Attributes["aws.eks.cluster.version"])
	assert.Equal(t, []string{"eks.10"}, target.Attributes["aws.eks.cluster.platform-version"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.eks.cluster.endpoint-public-access"])
	assert.Equal(t, []string{"false"}, target.Attributes["aws.eks.cluster.endpoint-private-access"])
	assert.Equal(t, []string{"0.0.0.0/0"}, target.Attributes["aws.eks.cluster.public-access-cidrs"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.eks.cluster.public-access-open-to-internet"])
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, target.Attributes["aws.eks.cluster.subnets"])
	assert.Equal(t, []string{"vpc-123"}, target.Attributes["aws.eks.cluster.vpc"])
	assert.Equal(t, []string{"api", "audit"}, target.Attributes["aws.eks.cluster.logging.enabled-types"])
	assert.Equal(t, []string{"authenticator", "controllerManager", "scheduler"}, target.Attributes["aws.eks.cluster.logging.disabled-types"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.eks.cluster.secrets-encryption.enabled"])
	assert.Equal(t, []string{"true"}, target.Attributes["aws.eks.cluster.deletion-protection"])
	assert.Equal(t, []string{"Demo"}, target.Attributes["aws.eks.cluster.label.application"])
	assert.Equal(t, []string{"prod"}, target.Attributes["aws.eks.cluster.label.environment"])
	assert.Equal(t, []string{"arn:role"}, target.Attributes["extension-aws.discovered-by-role"])
}

func TestPublicAccessNotOpenWhenRestricted(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{Clusters: []string{"c"}}, nil)
	api.On("DescribeCluster", mock.Anything, mock.Anything).Return(&eks.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Arn: aws.String("arn:c"), Name: aws.String("c"),
			ResourcesVpcConfig: &types.VpcConfigResponse{
				EndpointPublicAccess: true,
				PublicAccessCidrs:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			},
		},
	}, nil)
	targets, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.eks.cluster.public-access-open-to-internet"])
}

func TestPublicAccessNotOpenWhenPublicDisabled(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{Clusters: []string{"c"}}, nil)
	api.On("DescribeCluster", mock.Anything, mock.Anything).Return(&eks.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Arn: aws.String("arn:c"), Name: aws.String("c"),
			ResourcesVpcConfig: &types.VpcConfigResponse{
				EndpointPublicAccess: false,
			},
		},
	}, nil)
	targets, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.eks.cluster.public-access-open-to-internet"])
}

func TestSecretsEncryptionFalseWhenAbsent(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{Clusters: []string{"c"}}, nil)
	api.On("DescribeCluster", mock.Anything, mock.Anything).Return(&eks.DescribeClusterOutput{
		Cluster: &types.Cluster{
			Arn: aws.String("arn:c"), Name: aws.String("c"),
		},
	}, nil)
	targets, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"false"}, targets[0].Attributes["aws.eks.cluster.secrets-encryption.enabled"])
}

func TestGetAllEksClustersError(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))
	_, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.EqualError(t, err, "expected")
}

func TestTagFilterMismatchSkipsCluster(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{Clusters: []string{"c"}}, nil)
	api.On("DescribeCluster", mock.Anything, mock.Anything).Return(&eks.DescribeClusterOutput{
		Cluster: &types.Cluster{Arn: aws.String("arn:c"), Name: aws.String("c"), Tags: map[string]string{"application": "Other"}},
	}, nil)
	targets, err := getAllEksClusters(context.Background(), api, &utils.AwsAccess{
		AccountNumber: "42", Region: "us-east-1",
		TagFilters: []extConfig.TagFilter{{Key: "application", Values: []string{"Demo"}}},
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(targets))
}

func TestClusterDiscoveryEmitsK8sEnrichmentRules(t *testing.T) {
	d := &eksClusterDiscovery{}
	rules := d.DescribeEnrichmentRules()
	// One rule per K8s target type we forward EKS reliability config to.
	assert.Equal(t, len(eksEnrichmentTargetTypes), len(rules))

	destTypes := make(map[string]bool, len(rules))
	for _, r := range rules {
		// Both sides join on k8s.cluster-name.
		assert.Equal(t, "${dest.k8s.cluster-name}", r.Src.Selector["k8s.cluster-name"])
		assert.Equal(t, "${src.k8s.cluster-name}", r.Dest.Selector["k8s.cluster-name"])
		assert.Equal(t, clusterTargetId, r.Src.Type)
		destTypes[r.Dest.Type] = true
	}
	// All expected K8s target types are covered.
	for _, dt := range eksEnrichmentTargetTypes {
		assert.True(t, destTypes[dt], "expected enrichment rule for %s", dt)
	}
}
