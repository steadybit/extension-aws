// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

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

type subnetDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*subnetDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*subnetDiscovery)(nil)
)

func NewSubnetDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &subnetDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalSubnet)*time.Second),
	)
}

func (e *subnetDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: subnetTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalSubnet)),
		},
	}
}

func (e *subnetDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       subnetTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "Subnet", Other: "Subnets"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(subnetIcon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws.ec2.subnet.id"},
				{Attribute: "aws.ec2.subnet.name"},
				{Attribute: "aws.ec2.subnet.cidr"},
				{Attribute: "aws.account"},
				{Attribute: "aws.zone"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws.ec2.subnet.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (e *subnetDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.ec2.subnet.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Subnet name",
				Other: "Subnet names",
			},
		}, {
			Attribute: "aws.ec2.subnet.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Subnet ID",
				Other: "Subnet IDs",
			},
		}, {
			Attribute: "aws.ec2.subnet.cidr",
			Label: discovery_kit_api.PluralLabel{
				One:   "Subnet CIDR",
				Other: "Subnet CIDRs",
			},
		},
	}
}

func (e *subnetDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getEc2SubnetsForAccount, ctx, "ec2-subnet")
}

func getEc2SubnetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := GetAllSubnets(ctx, client, Util, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover ec2-subnets for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_SUBNET=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type subnetDiscoveryEc2Util interface {
	GetZoneUtil
	GetVpcNameUtil
}

func GetAllSubnets(ctx context.Context, ec2Api ec2.DescribeSubnetsAPIClient, ec2Util instanceDiscoveryEc2Util, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := ec2.NewDescribeSubnetsPaginator(ec2Api, &ec2.DescribeSubnetsInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, subnet := range output.Subnets {
			result = append(result, toSubnetTarget(subnet, ec2Util, awsAccountNumber, awsRegion))
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesSubnet), nil
}

func toSubnetTarget(subnet types.Subnet, ec2Util instanceDiscoveryEc2Util, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	var name *string
	for _, tag := range subnet.Tags {
		if *tag.Key == "Name" {
			name = tag.Value
		}
	}

	label := *subnet.SubnetId
	if name != nil {
		label = label + " / " + *name
	}

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.zone"] = []string{aws.ToString(subnet.AvailabilityZone)}
	if subnet.AvailabilityZoneId != nil {
		attributes["aws.zone.id"] = []string{aws.ToString(subnet.AvailabilityZoneId)}
	}
	if name != nil {
		attributes["aws.ec2.subnet.name"] = []string{aws.ToString(name)}
	}
	attributes["aws.ec2.subnet.id"] = []string{aws.ToString(subnet.SubnetId)}
	attributes["aws.ec2.subnet.cidr"] = []string{aws.ToString(subnet.CidrBlock)}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.vpc.id"] = []string{aws.ToString(subnet.VpcId)}
	attributes["aws.vpc.name"] = []string{ec2Util.GetVpcName(awsAccountNumber, awsRegion, aws.ToString(subnet.VpcId))}
	for _, tag := range subnet.Tags {
		if aws.ToString(tag.Key) == "Name" {
			continue
		}
		attributes[fmt.Sprintf("aws.ec2.subnet.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	return discovery_kit_api.Target{
		Id:         *subnet.SubnetId,
		Label:      label,
		TargetType: subnetTargetType,
		Attributes: attributes,
	}
}
