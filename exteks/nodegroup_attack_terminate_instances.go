// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-aws/v2/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type EksNodegroupTerminateInstancesAttackState struct {
	ClusterName      string
	NodegroupName    string
	Account          string
	Region           string
	DiscoveredByRole *string
	Percentage       int
	InstanceIds      []string
}

type eksNodegroupTerminateInstancesAttack struct {
	eksClientProvider func(account string, region string, role *string) (EksApi, error)
	asgClientProvider func(account string, region string, role *string) (EksAsgApi, error)
	ec2ClientProvider func(account string, region string, role *string) (EksEc2Api, error)
	rng               func(n int) []int // returns a permutation of [0,n)
}

var _ action_kit_sdk.Action[EksNodegroupTerminateInstancesAttackState] = (*eksNodegroupTerminateInstancesAttack)(nil)

func NewEksNodegroupTerminateInstancesAttack() action_kit_sdk.Action[EksNodegroupTerminateInstancesAttackState] {
	return &eksNodegroupTerminateInstancesAttack{
		eksClientProvider: defaultEksClientProvider,
		asgClientProvider: defaultEksAsgClientProvider,
		ec2ClientProvider: defaultEksEc2ClientProvider,
		rng:               rand.Perm,
	}
}

func (a *eksNodegroupTerminateInstancesAttack) NewEmptyState() EksNodegroupTerminateInstancesAttackState {
	return EksNodegroupTerminateInstancesAttackState{}
}

func (a *eksNodegroupTerminateInstancesAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    fmt.Sprintf("%s.terminate-instances", nodegroupTargetId),
		Label: "Terminate EKS node group instances",
		Description: "Terminates a percentage of EC2 instances backing an EKS managed node group. The underlying Auto Scaling group automatically replaces the terminated instances within minutes. " +
			"Validates pod rescheduling, PDB enforcement, cluster-autoscaler scale-up timing, and stateful workload AZ failover. " +
			"This is an instantaneous attack — there is no automatic rollback; AWS handles instance replacement.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    new(eksIcon),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType: nodegroupTargetId,
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by cluster name and node group name",
					Description: new("Find node group by cluster name and node group name"),
					Query:       "aws.eks.cluster.name=\"\" and aws.eks.nodegroup.name=\"\"",
				},
			}),
		}),
		Technology:  new("AWS"),
		Category:    new("EKS"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "percentage",
				Label:        "Percentage of instances to terminate",
				Description:  new("Percentage (1-100) of the node group's InService instances to terminate. Defaults to 33%."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("33"),
				Order:        new(1),
				Required:     new(true),
				MinValue:     extutil.Ptr(1),
				MaxValue:     extutil.Ptr(100),
			},
		},
	}
}

func (a *eksNodegroupTerminateInstancesAttack) Prepare(ctx context.Context, state *EksNodegroupTerminateInstancesAttackState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.Account = extutil.MustHaveValue(request.Target.Attributes, "aws.account")[0]
	state.Region = extutil.MustHaveValue(request.Target.Attributes, "aws.region")[0]
	state.ClusterName = extutil.MustHaveValue(request.Target.Attributes, "aws.eks.cluster.name")[0]
	state.NodegroupName = extutil.MustHaveValue(request.Target.Attributes, "aws.eks.nodegroup.name")[0]
	state.DiscoveredByRole = utils.GetOptionalTargetAttribute(request.Target.Attributes, "extension-aws.discovered-by-role")

	pct := extutil.ToInt(request.Config["percentage"])
	if pct < 1 || pct > 100 {
		return nil, extension_kit.ToError("percentage must be between 1 and 100.", nil)
	}
	state.Percentage = pct

	eksClient, err := a.eksClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EKS client for AWS account %s", state.Account), err)
	}
	asgClient, err := a.asgClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Auto Scaling client for AWS account %s", state.Account), err)
	}

	described, err := eksClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(state.ClusterName),
		NodegroupName: aws.String(state.NodegroupName),
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe EKS node group %s/%s", state.ClusterName, state.NodegroupName), err)
	}
	if described.Nodegroup == nil || described.Nodegroup.Resources == nil || len(described.Nodegroup.Resources.AutoScalingGroups) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("EKS node group %s/%s has no underlying Auto Scaling groups; cannot resolve instances", state.ClusterName, state.NodegroupName), nil)
	}

	asgNames := make([]string, 0, len(described.Nodegroup.Resources.AutoScalingGroups))
	for _, asg := range described.Nodegroup.Resources.AutoScalingGroups {
		if asg.Name != nil {
			asgNames = append(asgNames, *asg.Name)
		}
	}
	if len(asgNames) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("EKS node group %s/%s has no resolvable Auto Scaling group names", state.ClusterName, state.NodegroupName), nil)
	}

	asgOut, err := asgClient.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgNames,
	})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe Auto Scaling groups for EKS node group %s/%s", state.ClusterName, state.NodegroupName), err)
	}

	allInstances := make([]string, 0)
	for _, group := range asgOut.AutoScalingGroups {
		for _, inst := range group.Instances {
			if inst.InstanceId == nil {
				continue
			}
			// Only target healthy InService instances; skip Pending/Terminating/Standby.
			if inst.LifecycleState != autoscalingtypes.LifecycleStateInService {
				continue
			}
			allInstances = append(allInstances, *inst.InstanceId)
		}
	}
	sort.Strings(allInstances)

	if len(allInstances) == 0 {
		return nil, extension_kit.ToError(fmt.Sprintf("EKS node group %s/%s currently has no InService instances to terminate", state.ClusterName, state.NodegroupName), nil)
	}

	sampleSize := int(math.Ceil(float64(len(allInstances)) * float64(pct) / 100.0))
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize > len(allInstances) {
		sampleSize = len(allInstances)
	}

	perm := a.rng(len(allInstances))
	state.InstanceIds = make([]string, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		state.InstanceIds = append(state.InstanceIds, allInstances[perm[i]])
	}
	sort.Strings(state.InstanceIds)

	return &action_kit_api.PrepareResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level: extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Selected %d of %d InService instance(s) (%d%%) in EKS node group %s/%s for termination: %v",
				sampleSize, len(allInstances), pct, state.ClusterName, state.NodegroupName, state.InstanceIds),
		}}),
	}, nil
}

func (a *eksNodegroupTerminateInstancesAttack) Start(ctx context.Context, state *EksNodegroupTerminateInstancesAttackState) (*action_kit_api.StartResult, error) {
	if len(state.InstanceIds) == 0 {
		return nil, extension_kit.ToError("No instances selected for termination.", nil)
	}
	ec2Client, err := a.ec2ClientProvider(state.Account, state.Region, state.DiscoveredByRole)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize EC2 client for AWS account %s", state.Account), err)
	}
	_, err = ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: state.InstanceIds})
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to terminate EKS node group instances %v", state.InstanceIds), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level: extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Termination requested for %d instance(s) in EKS node group %s/%s: %v. The underlying Auto Scaling group will replace them.",
				len(state.InstanceIds), state.ClusterName, state.NodegroupName, state.InstanceIds),
		}}),
	}, nil
}
