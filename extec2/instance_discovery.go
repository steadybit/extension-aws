// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
	"time"
)

type ec2Discovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber          = (*ec2Discovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber       = (*ec2Discovery)(nil)
	_ discovery_kit_sdk.EnrichmentRulesDescriber = (*ec2Discovery)(nil)
)

func NewEc2InstanceDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &ec2Discovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEc2)*time.Second),
	)
}

func (e *ec2Discovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         ec2TargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEc2)),
		},
	}
}

func (e *ec2Discovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       ec2TargetId,
		Label:    discovery_kit_api.PluralLabel{One: "EC2-instance", Other: "EC2-instances"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(ec2Icon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws-ec2.instance.name"},
				{Attribute: "aws-ec2.instance.id"},
				{Attribute: "aws.account"},
				{Attribute: "aws.zone"},
				{Attribute: "aws-ec2.hostname.public"},
				{Attribute: "aws-ec2.hostname.internal"},
				{Attribute: "aws-ec2.state"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws-ec2.instance.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (e *ec2Discovery) DescribeEnrichmentRules() []discovery_kit_api.TargetEnrichmentRule {
	rules := []discovery_kit_api.TargetEnrichmentRule{
		getEc2InstanceToHostEnrichmentRule("com.steadybit.extension_host.host"),
		getEc2InstanceToHostEnrichmentRule("com.steadybit.extension_kubernetes.kubernetes-node"),
	}
	for _, targetType := range config.Config.EnrichEc2DataForTargetTypes {
		rules = append(rules, getEc2InstanceToXEnrichmentRule(targetType))
	}
	return rules
}

func getEc2InstanceToHostEnrichmentRule(target string) discovery_kit_api.TargetEnrichmentRule {
	id := fmt.Sprintf("com.steadybit.extension_aws.ec2-instance-to-%s", target)
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      id,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: ec2TargetId,
			Selector: map[string]string{
				"aws-ec2.hostname.internal": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: target,
			Selector: map[string]string{
				"host.hostname": "${src.aws-ec2.hostname.internal}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.account",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.region",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.zone.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.arn",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.image",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.instance.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.instance.name",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.ipv4.private",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.ipv4.public",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.vpc",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.hostname.internal",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.hostname.public",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "aws-ec2.label.",
			},
		},
	}
}

func getEc2InstanceToXEnrichmentRule(destTargetType string) discovery_kit_api.TargetEnrichmentRule {
	id := fmt.Sprintf("com.steadybit.extension_aws.ec2-instance-to-%s", destTargetType)
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      id,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: ec2TargetId,
			Selector: map[string]string{
				"aws-ec2.hostname.internal": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: destTargetType,
			Selector: map[string]string{
				"host.hostname": "${src.aws-ec2.hostname.internal}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.account",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.region",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.zone",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws.zone.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "aws-ec2.instance.id",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "aws-ec2.label.",
			},
		},
	}
}

func (e *ec2Discovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws-ec2.hostname.internal",
			Label: discovery_kit_api.PluralLabel{
				One:   "internal hostname",
				Other: "internal hostnames",
			},
		}, {
			Attribute: "aws-ec2.hostname.public",
			Label: discovery_kit_api.PluralLabel{
				One:   "public hostname",
				Other: "public hostnames",
			},
		}, {
			Attribute: "aws.ec2.instance.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Instance ID",
				Other: "Instance IDs",
			},
		}, {
			Attribute: "aws.ec2.instance.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Instance Name",
				Other: "Instance Names",
			},
		}, {
			Attribute: "aws-ec2.state",
			Label: discovery_kit_api.PluralLabel{
				One:   "Instance State",
				Other: "Instance States",
			},
		},
	}
}

func (e *ec2Discovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, ctx, "ec2-instance")
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := GetAllEc2Instances(ctx, client, utils.Zones, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover ec2-instances for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EC2=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type Ec2DescribeInstancesApi interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func GetAllEc2Instances(ctx context.Context, ec2Api Ec2DescribeInstancesApi, zoneUtil utils.GetZoneUtil, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := ec2.NewDescribeInstancesPaginator(ec2Api, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, reservation := range output.Reservations {
			for _, ec2Instance := range reservation.Instances {
				result = append(result, toTarget(ec2Instance, zoneUtil, awsAccountNumber, awsRegion))
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEc2), nil
}

func toTarget(ec2Instance types.Instance, zoneUtil utils.GetZoneUtil, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	var name *string
	for _, tag := range ec2Instance.Tags {
		if *tag.Key == "Name" {
			name = tag.Value
		}
	}

	arn := fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", awsRegion, awsAccountNumber, *ec2Instance.InstanceId)
	label := *ec2Instance.InstanceId
	if name != nil {
		label = label + " / " + *name
	}
	availabilityZoneName := aws.ToString(ec2Instance.Placement.AvailabilityZone)
	availabilityZoneApi := zoneUtil.GetZone(awsAccountNumber, availabilityZoneName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws-ec2.image"] = []string{aws.ToString(ec2Instance.ImageId)}
	attributes["aws.zone"] = []string{availabilityZoneName}
	if availabilityZoneApi != nil {
		attributes["aws.zone.id"] = []string{*availabilityZoneApi.ZoneId}
	}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws-ec2.ipv4.private"] = []string{aws.ToString(ec2Instance.PrivateIpAddress)}
	if ec2Instance.PublicIpAddress != nil {
		attributes["aws-ec2.ipv4.public"] = []string{aws.ToString(ec2Instance.PublicIpAddress)}
	}
	attributes["aws-ec2.instance.id"] = []string{aws.ToString(ec2Instance.InstanceId)}
	if name != nil {
		attributes["aws-ec2.instance.name"] = []string{aws.ToString(name)}
	}
	attributes["aws-ec2.hostname.internal"] = []string{aws.ToString(ec2Instance.PrivateDnsName)}
	if ec2Instance.PublicDnsName != nil {
		attributes["aws-ec2.hostname.public"] = []string{aws.ToString(ec2Instance.PublicDnsName)}
	}
	attributes["aws-ec2.arn"] = []string{arn}
	attributes["aws-ec2.vpc"] = []string{aws.ToString(ec2Instance.VpcId)}
	if ec2Instance.State != nil {
		attributes["aws-ec2.state"] = []string{string(ec2Instance.State.Name)}
	}
	for _, tag := range ec2Instance.Tags {
		if aws.ToString(tag.Key) == "Name" {
			continue
		}
		attributes[fmt.Sprintf("aws-ec2.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	return discovery_kit_api.Target{
		Id:         *ec2Instance.InstanceId,
		Label:      label,
		TargetType: ec2TargetId,
		Attributes: attributes,
	}
}
