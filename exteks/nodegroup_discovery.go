// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type eksNodegroupDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*eksNodegroupDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*eksNodegroupDiscovery)(nil)
)

func NewEksNodegroupDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &eksNodegroupDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEks)*time.Second),
	)
}

func (d *eksNodegroupDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: nodegroupTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEks)),
		},
	}
}

func (d *eksNodegroupDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       nodegroupTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "EKS node group", Other: "EKS node groups"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(eksIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.eks.cluster.name"},
				{Attribute: "aws.eks.nodegroup.capacity-type"},
				{Attribute: "aws.eks.nodegroup.min-size"},
				{Attribute: "aws.eks.nodegroup.max-size"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{Attribute: "steadybit.label", Direction: "ASC"},
			},
		},
	}
}

func (d *eksNodegroupDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.eks.nodegroup.name", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group name", Other: "AWS EKS node group names"}},
		{Attribute: "aws.eks.cluster.name", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster name", Other: "AWS EKS cluster names"}},
		{Attribute: "aws.eks.nodegroup.subnets", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group subnet", Other: "AWS EKS node group subnets"}},
		{Attribute: "aws.eks.nodegroup.min-size", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group min size", Other: "AWS EKS node group min sizes"}},
		{Attribute: "aws.eks.nodegroup.max-size", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group max size", Other: "AWS EKS node group max sizes"}},
		{Attribute: "aws.eks.nodegroup.capacity-type", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group capacity-type", Other: "AWS EKS node group capacity-types"}},
		{Attribute: "aws.eks.nodegroup.instance-types", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group instance type", Other: "AWS EKS node group instance types"}},
		{Attribute: "aws.eks.nodegroup.ami-type", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group AMI type", Other: "AWS EKS node group AMI types"}},
		{Attribute: "aws.eks.nodegroup.release-version", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group release version", Other: "AWS EKS node group release versions"}},
		{Attribute: "aws.eks.nodegroup.disk-size", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group disk size", Other: "AWS EKS node group disk sizes"}},
		{Attribute: "aws.eks.nodegroup.update-config.max-unavailable", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group max-unavailable", Other: "AWS EKS node group max-unavailable"}},
		{Attribute: "aws.eks.nodegroup.update-config.max-unavailable-percentage", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group max-unavailable percentage", Other: "AWS EKS node group max-unavailable percentages"}},
		{Attribute: "aws.eks.nodegroup.taints", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group taint", Other: "AWS EKS node group taints"}},
		{Attribute: "aws.eks.nodegroup.launch-template.id", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group launch template id", Other: "AWS EKS node group launch template ids"}},
		{Attribute: "aws.eks.nodegroup.launch-template.version", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group launch template version", Other: "AWS EKS node group launch template versions"}},
		{Attribute: "aws.eks.nodegroup.autoscaling-groups", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group underlying Auto Scaling group", Other: "AWS EKS node group underlying Auto Scaling groups"}},
		{Attribute: "aws.eks.nodegroup.status", Label: discovery_kit_api.PluralLabel{One: "AWS EKS node group status", Other: "AWS EKS node group status"}},
	}
}

func (d *eksNodegroupDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getEksNodegroupTargets, ctx, "eks-nodegroup")
}

func getEksNodegroupTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := eks.NewFromConfig(account.AwsConfig)
	result, err := getAllEksNodegroups(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover EKS node groups for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EKS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllEksNodegroups(ctx context.Context, client EksApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 10)
	clusterPaginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for clusterPaginator.HasMorePages() {
		output, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, clusterName := range output.Clusters {
			ngPaginator := eks.NewListNodegroupsPaginator(client, &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)})
			for ngPaginator.HasMorePages() {
				ngOutput, err := ngPaginator.NextPage(ctx)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to list node groups for EKS cluster %s", clusterName)
					break
				}
				for _, ngName := range ngOutput.Nodegroups {
					described, err := client.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
						ClusterName:   aws.String(clusterName),
						NodegroupName: aws.String(ngName),
					})
					if err != nil {
						log.Warn().Err(err).Msgf("Failed to describe EKS node group %s/%s", clusterName, ngName)
						continue
					}
					if described.Nodegroup == nil {
						continue
					}
					if !matchesClusterTagFilter(described.Nodegroup.Tags, account.TagFilters) {
						continue
					}
					result = append(result, toNodegroupTarget(*described.Nodegroup, account.AccountNumber, account.Region, account.AssumeRole))
				}
			}
		}
	}
	return result, nil
}

func toNodegroupTarget(ng types.Nodegroup, account string, region string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(ng.NodegroupArn)
	clusterName := aws.ToString(ng.ClusterName)
	name := aws.ToString(ng.NodegroupName)
	label := fmt.Sprintf("%s/%s", clusterName, name)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.eks.cluster.name"] = []string{clusterName}
	attributes["aws.eks.nodegroup.name"] = []string{name}
	attributes["aws.eks.nodegroup.status"] = []string{string(ng.Status)}

	if len(ng.Subnets) > 0 {
		subnets := append([]string(nil), ng.Subnets...)
		sort.Strings(subnets)
		attributes["aws.eks.nodegroup.subnets"] = subnets
	}

	if ng.ScalingConfig != nil {
		if ng.ScalingConfig.MinSize != nil {
			attributes["aws.eks.nodegroup.min-size"] = []string{strconv.Itoa(int(*ng.ScalingConfig.MinSize))}
		}
		if ng.ScalingConfig.MaxSize != nil {
			attributes["aws.eks.nodegroup.max-size"] = []string{strconv.Itoa(int(*ng.ScalingConfig.MaxSize))}
		}
		// DesiredSize is intentionally NOT exposed: cluster-autoscaler will mutate it,
		// causing target updates that put pressure on the platform.
	}

	if ng.CapacityType != "" {
		attributes["aws.eks.nodegroup.capacity-type"] = []string{string(ng.CapacityType)}
	}
	if len(ng.InstanceTypes) > 0 {
		attributes["aws.eks.nodegroup.instance-types"] = ng.InstanceTypes
	}
	if ng.AmiType != "" {
		attributes["aws.eks.nodegroup.ami-type"] = []string{string(ng.AmiType)}
	}
	if ng.ReleaseVersion != nil {
		attributes["aws.eks.nodegroup.release-version"] = []string{*ng.ReleaseVersion}
	}
	if ng.DiskSize != nil && *ng.DiskSize > 0 {
		attributes["aws.eks.nodegroup.disk-size"] = []string{strconv.Itoa(int(*ng.DiskSize))}
	}

	if ng.UpdateConfig != nil {
		if ng.UpdateConfig.MaxUnavailable != nil {
			attributes["aws.eks.nodegroup.update-config.max-unavailable"] = []string{strconv.Itoa(int(*ng.UpdateConfig.MaxUnavailable))}
		}
		if ng.UpdateConfig.MaxUnavailablePercentage != nil {
			attributes["aws.eks.nodegroup.update-config.max-unavailable-percentage"] = []string{strconv.Itoa(int(*ng.UpdateConfig.MaxUnavailablePercentage))}
		}
	}

	if len(ng.Taints) > 0 {
		taints := make([]string, 0, len(ng.Taints))
		for _, t := range ng.Taints {
			taints = append(taints, fmt.Sprintf("%s=%s:%s", aws.ToString(t.Key), aws.ToString(t.Value), string(t.Effect)))
		}
		sort.Strings(taints)
		attributes["aws.eks.nodegroup.taints"] = taints
	}

	if ng.LaunchTemplate != nil {
		if ng.LaunchTemplate.Id != nil {
			attributes["aws.eks.nodegroup.launch-template.id"] = []string{*ng.LaunchTemplate.Id}
		}
		if ng.LaunchTemplate.Version != nil {
			attributes["aws.eks.nodegroup.launch-template.version"] = []string{*ng.LaunchTemplate.Version}
		}
	}

	if ng.Resources != nil && len(ng.Resources.AutoScalingGroups) > 0 {
		asgNames := make([]string, 0, len(ng.Resources.AutoScalingGroups))
		for _, asg := range ng.Resources.AutoScalingGroups {
			if asg.Name != nil {
				asgNames = append(asgNames, *asg.Name)
			}
		}
		sort.Strings(asgNames)
		if len(asgNames) > 0 {
			attributes["aws.eks.nodegroup.autoscaling-groups"] = asgNames
		}
	}

	for k, v := range ng.Labels {
		attributes[fmt.Sprintf("aws.eks.nodegroup.k8s-label.%s", strings.ToLower(k))] = []string{v}
	}
	for k, v := range ng.Tags {
		attributes[fmt.Sprintf("aws.eks.nodegroup.label.%s", strings.ToLower(k))] = []string{v}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: nodegroupTargetId,
		Attributes: attributes,
	}
}
