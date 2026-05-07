// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	eksIcon           = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmwxMCA1djEwbC0xMCA1LTEwLTVWN2wxMC01em0wIDIuM0w0IDh2OGw4IDQgOC00VjhsLTgtMy43em0wIDIuN2w2IDIuOHY1LjlsLTYgMi44LTYtMi44VjkuOEwxMiA3em0wIDIuNGwtMy42IDEuN3YzbDMuNiAxLjcgMy42LTEuN3YtM0wxMiA5LjR6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
	clusterTargetId   = "com.steadybit.extension_aws.eks.cluster"
	nodegroupTargetId = "com.steadybit.extension_aws.eks.nodegroup"
)

type EksApi interface {
	eks.ListClustersAPIClient
	eks.ListNodegroupsAPIClient
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}

// EksAsgApi is the subset of the autoscaling API used by EKS attacks (resolving the underlying ASG instances).
type EksAsgApi interface {
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

// EksEc2Api is the subset of the EC2 API used by EKS attacks.
type EksEc2Api interface {
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}

func defaultEksClientProvider(account string, region string, role *string) (EksApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return eks.NewFromConfig(awsAccess.AwsConfig), nil
}

func defaultEksAsgClientProvider(account string, region string, role *string) (EksAsgApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return autoscaling.NewFromConfig(awsAccess.AwsConfig), nil
}

func defaultEksEc2ClientProvider(account string, region string, role *string) (EksEc2Api, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return ec2.NewFromConfig(awsAccess.AwsConfig), nil
}
