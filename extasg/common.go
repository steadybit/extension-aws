// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extasg

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	asgIcon     = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgMmwzIDNoLTJ2NGgtMlY1SDlsMy0zem0wIDIwbC0zLTNoMnYtNGgyVjE5aDJsLTMgM3pNMiAxMmwzLTN2MmgxNHYyVjE1SDV2MmwtMy0zem0yMCAwbC0zIDN2LTJINXYtMmgxNHYtMmwzIDN6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
	asgTargetId = "com.steadybit.extension_aws.asg"
)

type AsgAttackState struct {
	AutoScalingGroupName string
	Account              string
	Region               string
	DiscoveredByRole     *string
	SuspendedProcesses   []string
}

type AsgApi interface {
	autoscaling.DescribeAutoScalingGroupsAPIClient
	SuspendProcesses(ctx context.Context, params *autoscaling.SuspendProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.SuspendProcessesOutput, error)
	ResumeProcesses(ctx context.Context, params *autoscaling.ResumeProcessesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.ResumeProcessesOutput, error)
}

func defaultAsgClientProvider(account string, region string, role *string) (AsgApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return autoscaling.NewFromConfig(awsAccess.AwsConfig), nil
}
