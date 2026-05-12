// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetAllEksNodegroups(t *testing.T) {
	api := new(eksApiMock)
	api.On("ListClusters", mock.Anything, mock.Anything).Return(&eks.ListClustersOutput{Clusters: []string{"prod"}}, nil)
	api.On("ListNodegroups", mock.Anything, mock.MatchedBy(func(p *eks.ListNodegroupsInput) bool {
		return aws.ToString(p.ClusterName) == "prod"
	})).Return(&eks.ListNodegroupsOutput{Nodegroups: []string{"workers"}}, nil)
	api.On("DescribeNodegroup", mock.Anything, mock.MatchedBy(func(p *eks.DescribeNodegroupInput) bool {
		return aws.ToString(p.ClusterName) == "prod" && aws.ToString(p.NodegroupName) == "workers"
	})).Return(&eks.DescribeNodegroupOutput{
		Nodegroup: &types.Nodegroup{
			NodegroupArn:  aws.String("arn:ng:prod/workers"),
			NodegroupName: aws.String("workers"),
			ClusterName:   aws.String("prod"),
			Status:        types.NodegroupStatusActive,
			Subnets:       []string{"subnet-b", "subnet-a"},
			ScalingConfig: &types.NodegroupScalingConfig{
				MinSize:     aws.Int32(2),
				MaxSize:     aws.Int32(10),
				DesiredSize: aws.Int32(7),
			},
			CapacityType:   types.CapacityTypesSpot,
			InstanceTypes:  []string{"m6i.large", "m6a.large"},
			AmiType:        types.AMITypesAl2023X8664Standard,
			ReleaseVersion: aws.String("1.29.0-20240101"),
			DiskSize:       aws.Int32(50),
			UpdateConfig: &types.NodegroupUpdateConfig{
				MaxUnavailable: aws.Int32(1),
			},
			Taints: []types.Taint{
				{Key: aws.String("dedicated"), Value: aws.String("gpu"), Effect: types.TaintEffectNoSchedule},
			},
			LaunchTemplate: &types.LaunchTemplateSpecification{
				Id:      aws.String("lt-eks"),
				Version: aws.String("3"),
			},
			Resources: &types.NodegroupResources{
				AutoScalingGroups: []types.AutoScalingGroup{
					{Name: aws.String("eks-workers-asg-1")},
					{Name: aws.String("eks-workers-asg-0")},
				},
			},
			Labels: map[string]string{"workload": "batch"},
			Tags:   map[string]string{"application": "Demo"},
		},
	}, nil)

	targets, err := getAllEksNodegroups(context.Background(), api, &utils.AwsAccess{AccountNumber: "42", Region: "us-east-1"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(targets))
	tgt := targets[0]
	assert.Equal(t, nodegroupTargetId, tgt.TargetType)
	assert.Equal(t, "prod/workers", tgt.Label)
	assert.Equal(t, []string{"prod"}, tgt.Attributes["aws.eks.cluster.name"])
	// Cluster name is also surfaced under k8s.cluster-name so extension-kubernetes enrichment can join.
	assert.Equal(t, []string{"prod"}, tgt.Attributes["k8s.cluster-name"])
	assert.Equal(t, []string{"workers"}, tgt.Attributes["aws.eks.nodegroup.name"])
	assert.Equal(t, []string{"subnet-a", "subnet-b"}, tgt.Attributes["aws.eks.nodegroup.subnets"])
	assert.Equal(t, []string{"2"}, tgt.Attributes["aws.eks.nodegroup.min-size"])
	assert.Equal(t, []string{"10"}, tgt.Attributes["aws.eks.nodegroup.max-size"])
	assert.Equal(t, []string{"SPOT"}, tgt.Attributes["aws.eks.nodegroup.capacity-type"])
	assert.Equal(t, []string{"m6i.large", "m6a.large"}, tgt.Attributes["aws.eks.nodegroup.instance-types"])
	assert.Equal(t, []string{"AL2023_x86_64_STANDARD"}, tgt.Attributes["aws.eks.nodegroup.ami-type"])
	assert.Equal(t, []string{"1.29.0-20240101"}, tgt.Attributes["aws.eks.nodegroup.release-version"])
	assert.Equal(t, []string{"50"}, tgt.Attributes["aws.eks.nodegroup.disk-size"])
	assert.Equal(t, []string{"1"}, tgt.Attributes["aws.eks.nodegroup.update-config.max-unavailable"])
	assert.Equal(t, []string{"dedicated=gpu:NO_SCHEDULE"}, tgt.Attributes["aws.eks.nodegroup.taints"])
	assert.Equal(t, []string{"lt-eks"}, tgt.Attributes["aws.eks.nodegroup.launch-template.id"])
	assert.Equal(t, []string{"3"}, tgt.Attributes["aws.eks.nodegroup.launch-template.version"])
	assert.Equal(t, []string{"eks-workers-asg-0", "eks-workers-asg-1"}, tgt.Attributes["aws.eks.nodegroup.autoscaling-groups"])
	assert.Equal(t, []string{"batch"}, tgt.Attributes["aws.eks.nodegroup.k8s-label.workload"])
	assert.Equal(t, []string{"Demo"}, tgt.Attributes["aws.eks.nodegroup.label.application"])

	// desired-size must NOT be exposed (mutated by cluster-autoscaler)
	_, hasDesired := tgt.Attributes["aws.eks.nodegroup.desired-size"]
	assert.False(t, hasDesired, "desired-size must not be exposed (volatile)")
}
