// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"errors"
	"fmt"
	"github.com/steadybit/extension-aws/v2/extec2"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type rdsInstanceDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*rdsInstanceDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*rdsInstanceDiscovery)(nil)
)

func NewRdsInstanceDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &rdsInstanceDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalRds)*time.Second),
	)
}

func (r *rdsInstanceDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: rdsInstanceTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalRds)),
		},
	}
}

func (r *rdsInstanceDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       rdsInstanceTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "RDS instance", Other: "RDS instances"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(rdsIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.rds.cluster"},
				{Attribute: "aws.rds.instance.status"},
				{Attribute: "aws.zone"},
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

func (r *rdsInstanceDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.rds.instance.id",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS instance ID",
				Other: "AWS RDS instance IDs",
			},
		}, {
			// See https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html#Overview.DBInstance.Status
			Attribute: "aws.rds.instance.status",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS RDS instance status",
				Other: "AWS RDS instance status",
			},
		},
	}
}

func (r *rdsInstanceDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getInstanceTargetsForAccount, ctx, "rds-instance")
}

func getInstanceTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(account.AwsConfig)
	result, err := getAllRdsInstances(ctx, client, extec2.Util, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover rds-instances for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type rdsInstanceDiscoveryEc2Util interface {
	extec2.GetZoneUtil
	extec2.GetVpcNameUtil
}

func getAllRdsInstances(ctx context.Context, rdsApi rdsDBInstanceApi, ec2Util rdsInstanceDiscoveryEc2Util, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := rds.NewDescribeDBInstancesPaginator(rdsApi, &rds.DescribeDBInstancesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, dbInstance := range output.DBInstances {
			if matchesTagFilter(dbInstance.TagList, account.TagFilters) {
				result = append(result, toInstanceTarget(dbInstance, ec2Util, account.AccountNumber, account.Region, account.AssumeRole))
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesRds), nil
}

func toInstanceTarget(dbInstance types.DBInstance, ec2util rdsInstanceDiscoveryEc2Util, awsAccountNumber string, awsRegion string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(dbInstance.DBInstanceArn)
	label := aws.ToString(dbInstance.DBInstanceIdentifier)
	availabilityZoneName := aws.ToString(dbInstance.AvailabilityZone)
	availabilityZoneApi := ec2util.GetZone(awsAccountNumber, awsRegion, availabilityZoneName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.zone"] = []string{availabilityZoneName}
	if availabilityZoneApi != nil {
		attributes["aws.zone.id"] = []string{*availabilityZoneApi.ZoneId}
	}
	attributes["aws.region"] = []string{awsRegion}
	if dbInstance.DBSubnetGroup != nil && dbInstance.DBSubnetGroup.VpcId != nil {
		attributes["aws.vpc.id"] = []string{aws.ToString(dbInstance.DBSubnetGroup.VpcId)}
		attributes["aws.vpc.name"] = []string{ec2util.GetVpcName(awsAccountNumber, awsRegion, aws.ToString(dbInstance.DBSubnetGroup.VpcId))}
	}
	attributes["aws.rds.engine"] = []string{aws.ToString(dbInstance.Engine)}
	attributes["aws.rds.instance.id"] = []string{label}
	attributes["aws.rds.instance.status"] = []string{aws.ToString(dbInstance.DBInstanceStatus)}

	if dbInstance.DBClusterIdentifier != nil {
		attributes["aws.rds.cluster"] = []string{aws.ToString(dbInstance.DBClusterIdentifier)}
	}
	for _, tag := range dbInstance.TagList {
		attributes[fmt.Sprintf("aws.rds.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}
	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: rdsInstanceTargetId,
		Attributes: attributes,
	}
}
