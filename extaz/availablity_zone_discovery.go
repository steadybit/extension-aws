// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	types2 "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"time"
)

type azDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber = (*azDiscovery)(nil)
)

func NewAzDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &azDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalZone)*time.Second),
	)
}

func (a *azDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: azTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalZone)),
		},
	}
}

func (a *azDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       azTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "Availability Zone", Other: "Availability Zones"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(azIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws.zone"},
				{Attribute: "aws.zone.id"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws.zone",
					Direction: "ASC",
				},
			},
		},
	}
}

func (a *azDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, ctx, "availability zone")
}

func getTargetsForAccount(account *utils.AwsAccount, _ context.Context) ([]discovery_kit_api.Target, error) {
	return getAllAvailabilityZones(utils.Zones, account.AccountNumber), nil
}

func getAllAvailabilityZones(zones utils.GetZonesUtil, awsAccountNumber string) []discovery_kit_api.Target {
	result := make([]discovery_kit_api.Target, 0, 20)
	for _, availabilityZone := range zones.GetZones(awsAccountNumber) {
		result = append(result, toTarget(availabilityZone, awsAccountNumber))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesZone)
}

func toTarget(availabilityZone types2.AvailabilityZone, awsAccountNumber string) discovery_kit_api.Target {
	label := aws.ToString(availabilityZone.ZoneName)
	id := aws.ToString(availabilityZone.ZoneName) + "@" + awsAccountNumber

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{aws.ToString(availabilityZone.RegionName)}
	attributes["aws.zone"] = []string{aws.ToString(availabilityZone.ZoneName)}
	attributes["aws.zone.id"] = []string{aws.ToString(availabilityZone.ZoneId)}
	attributes["aws.zone@account"] = []string{id}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      label,
		TargetType: azTargetType,
		Attributes: attributes,
	}
}
