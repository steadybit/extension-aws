// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extasg

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type asgDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*asgDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*asgDiscovery)(nil)
)

func NewAsgDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &asgDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalAsg)*time.Second),
	)
}

func (d *asgDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: asgTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalAsg)),
		},
	}
}

func (d *asgDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       asgTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Auto Scaling group", Other: "Auto Scaling groups"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(asgIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.asg.min-size"},
				{Attribute: "aws.asg.max-size"},
				{Attribute: "aws.asg.availability-zones"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func (d *asgDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.asg.name",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group name", Other: "AWS Auto Scaling group names"},
		}, {
			Attribute: "aws.asg.availability-zones",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group Availability Zone", Other: "AWS Auto Scaling group Availability Zones"},
		}, {
			Attribute: "aws.asg.subnets",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group subnet", Other: "AWS Auto Scaling group subnets"},
		}, {
			Attribute: "aws.asg.min-size",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group min size", Other: "AWS Auto Scaling group min sizes"},
		}, {
			Attribute: "aws.asg.max-size",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group max size", Other: "AWS Auto Scaling group max sizes"},
		}, {
			Attribute: "aws.asg.suspended-processes",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group suspended process", Other: "AWS Auto Scaling group suspended processes"},
		}, {
			Attribute: "aws.asg.health-check-type",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group health-check type", Other: "AWS Auto Scaling group health-check types"},
		}, {
			Attribute: "aws.asg.health-check-grace-period",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group health-check grace period", Other: "AWS Auto Scaling group health-check grace periods"},
		}, {
			Attribute: "aws.asg.default-cooldown",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group default cooldown", Other: "AWS Auto Scaling group default cooldowns"},
		}, {
			Attribute: "aws.asg.capacity-rebalance",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group capacity-rebalance", Other: "AWS Auto Scaling group capacity-rebalance"},
		}, {
			Attribute: "aws.asg.mixed-instances-policy.enabled",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group mixed-instances policy", Other: "AWS Auto Scaling group mixed-instances policies"},
		}, {
			Attribute: "aws.asg.launch-template.id",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group launch template id", Other: "AWS Auto Scaling group launch template ids"},
		}, {
			Attribute: "aws.asg.launch-template.version",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group launch template version", Other: "AWS Auto Scaling group launch template versions"},
		}, {
			Attribute: "aws.asg.launch-template.version-mode",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group launch template version mode", Other: "AWS Auto Scaling group launch template version modes"},
		}, {
			Attribute: "aws.asg.target-group-arns",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group target-group ARN", Other: "AWS Auto Scaling group target-group ARNs"},
		}, {
			Attribute: "aws.asg.termination-policies",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group termination policy", Other: "AWS Auto Scaling group termination policies"},
		}, {
			Attribute: "aws.asg.new-instances-protected-from-scale-in",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group new-instance scale-in protection", Other: "AWS Auto Scaling group new-instance scale-in protection"},
		}, {
			Attribute: "aws.asg.max-instance-lifetime",
			Label:     discovery_kit_api.PluralLabel{One: "AWS Auto Scaling group max instance lifetime", Other: "AWS Auto Scaling group max instance lifetimes"},
		},
	}
}

func (d *asgDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getAsgTargetsForAccount, ctx, "asg")
}

func getAsgTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := autoscaling.NewFromConfig(account.AwsConfig)
	result, err := getAllAsgs(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover Auto Scaling groups for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ASG=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllAsgs(ctx context.Context, client autoscaling.DescribeAutoScalingGroupsAPIClient, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client, &autoscaling.DescribeAutoScalingGroupsInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, asg := range output.AutoScalingGroups {
			if matchesTagFilter(asg.Tags, account.TagFilters) {
				result = append(result, toAsgTarget(asg, account.AccountNumber, account.Region, account.AssumeRole))
			}
		}
	}
	return result, nil
}

func matchesTagFilter(tags []types.TagDescription, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == filter.Key {
				for _, filterValue := range filter.Values {
					if tag.Value != nil && *tag.Value == filterValue {
						matched = true
						break
					}
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toAsgTarget(asg types.AutoScalingGroup, awsAccountNumber string, awsRegion string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(asg.AutoScalingGroupARN)
	name := aws.ToString(asg.AutoScalingGroupName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.asg.name"] = []string{name}

	if len(asg.AvailabilityZones) > 0 {
		attributes["aws.zone"] = asg.AvailabilityZones
		attributes["aws.asg.availability-zones"] = asg.AvailabilityZones
	}

	if asg.VPCZoneIdentifier != nil && *asg.VPCZoneIdentifier != "" {
		subnets := strings.Split(*asg.VPCZoneIdentifier, ",")
		trimmed := make([]string, 0, len(subnets))
		for _, s := range subnets {
			s = strings.TrimSpace(s)
			if s != "" {
				trimmed = append(trimmed, s)
			}
		}
		if len(trimmed) > 0 {
			attributes["aws.asg.subnets"] = trimmed
		}
	}

	if asg.MinSize != nil {
		attributes["aws.asg.min-size"] = []string{strconv.Itoa(int(*asg.MinSize))}
	}
	if asg.MaxSize != nil {
		attributes["aws.asg.max-size"] = []string{strconv.Itoa(int(*asg.MaxSize))}
	}
	if asg.HealthCheckType != nil {
		attributes["aws.asg.health-check-type"] = []string{*asg.HealthCheckType}
	}
	if asg.HealthCheckGracePeriod != nil {
		attributes["aws.asg.health-check-grace-period"] = []string{strconv.Itoa(int(*asg.HealthCheckGracePeriod))}
	}
	if asg.DefaultCooldown != nil {
		attributes["aws.asg.default-cooldown"] = []string{strconv.Itoa(int(*asg.DefaultCooldown))}
	}
	if asg.CapacityRebalance != nil {
		attributes["aws.asg.capacity-rebalance"] = []string{strconv.FormatBool(*asg.CapacityRebalance)}
	}
	if asg.NewInstancesProtectedFromScaleIn != nil {
		attributes["aws.asg.new-instances-protected-from-scale-in"] = []string{strconv.FormatBool(*asg.NewInstancesProtectedFromScaleIn)}
	}
	if asg.MaxInstanceLifetime != nil && *asg.MaxInstanceLifetime > 0 {
		attributes["aws.asg.max-instance-lifetime"] = []string{strconv.Itoa(int(*asg.MaxInstanceLifetime))}
	}

	if len(asg.SuspendedProcesses) > 0 {
		processes := make([]string, 0, len(asg.SuspendedProcesses))
		for _, p := range asg.SuspendedProcesses {
			if p.ProcessName != nil {
				processes = append(processes, *p.ProcessName)
			}
		}
		attributes["aws.asg.suspended-processes"] = processes
	}

	attributes["aws.asg.mixed-instances-policy.enabled"] = []string{strconv.FormatBool(asg.MixedInstancesPolicy != nil)}

	lt := asg.LaunchTemplate
	if asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil && asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification != nil {
		lt = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if lt != nil {
		if lt.LaunchTemplateId != nil {
			attributes["aws.asg.launch-template.id"] = []string{*lt.LaunchTemplateId}
		}
		if lt.Version != nil {
			version := *lt.Version
			attributes["aws.asg.launch-template.version"] = []string{version}
			switch version {
			case "$Latest":
				attributes["aws.asg.launch-template.version-mode"] = []string{"latest"}
			case "$Default":
				attributes["aws.asg.launch-template.version-mode"] = []string{"default"}
			default:
				attributes["aws.asg.launch-template.version-mode"] = []string{"pinned"}
			}
		}
	}

	if len(asg.TargetGroupARNs) > 0 {
		attributes["aws.asg.target-group-arns"] = asg.TargetGroupARNs
	}
	if len(asg.TerminationPolicies) > 0 {
		attributes["aws.asg.termination-policies"] = asg.TerminationPolicies
	}

	for _, tag := range asg.Tags {
		if tag.Key != nil {
			attributes[fmt.Sprintf("aws.asg.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
		}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: asgTargetId,
		Attributes: attributes,
	}
}
