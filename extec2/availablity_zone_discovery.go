// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	types2 "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
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
	return utils.ForEveryConfiguredAwsAccess(getAllAvailabilityZonesForAccount, ctx, "availability zone")
}

func getAllAvailabilityZonesForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	_, _ = InitEc2UtilForAccount(account, ctx)
	return getAllAvailabilityZonesFromCache(Util, account), nil
}

func getAllAvailabilityZonesFromCache(getZonesUtil GetZonesUtil, account *utils.AwsAccess) []discovery_kit_api.Target {
	result := make([]discovery_kit_api.Target, 0, 20)
	if len(account.TagFilters) > 0 {
		//Zones can not be tagged, return no targets. (Blackhole Zone Attack will not have any targets, which  makes sense because it can't be isolated to resources with a specific tag)
		return result
	}
	for _, availabilityZone := range getZonesUtil.GetZones(account) {
		result = append(result, toAvailabilityZoneTarget(availabilityZone, account.AccountNumber, account.AssumeRole))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesZone)
}

func toAvailabilityZoneTarget(availabilityZone types2.AvailabilityZone, awsAccountNumber string, role *string) discovery_kit_api.Target {
	label := aws.ToString(availabilityZone.ZoneName)
	id := aws.ToString(availabilityZone.ZoneName) + "@" + awsAccountNumber

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{aws.ToString(availabilityZone.RegionName)}
	attributes["aws.zone"] = []string{aws.ToString(availabilityZone.ZoneName)}
	attributes["aws.zone.id"] = []string{aws.ToString(availabilityZone.ZoneId)}
	attributes["aws.zone@account"] = []string{id}
	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      label,
		TargetType: azTargetType,
		Attributes: attributes,
	}
}
