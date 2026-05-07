// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extdynamodb

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
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aastypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type tableDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*tableDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*tableDiscovery)(nil)
)

func NewTableDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&tableDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalDynamodb)*time.Second),
	)
}

func (d *tableDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: tableTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalDynamodb)),
		},
	}
}

func (d *tableDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       tableTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "DynamoDB table", Other: "DynamoDB tables"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(dynamodbIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.dynamodb.billing-mode"},
				{Attribute: "aws.dynamodb.pitr.enabled"},
				{Attribute: "aws.dynamodb.deletion-protection"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *tableDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.dynamodb.table.name", Label: discovery_kit_api.PluralLabel{One: "DynamoDB table name", Other: "DynamoDB table names"}},
		{Attribute: "aws.dynamodb.billing-mode", Label: discovery_kit_api.PluralLabel{One: "DynamoDB billing mode", Other: "DynamoDB billing modes"}},
		{Attribute: "aws.dynamodb.table-class", Label: discovery_kit_api.PluralLabel{One: "DynamoDB table class", Other: "DynamoDB table classes"}},
		{Attribute: "aws.dynamodb.pitr.enabled", Label: discovery_kit_api.PluralLabel{One: "DynamoDB PITR", Other: "DynamoDB PITR"}},
		{Attribute: "aws.dynamodb.deletion-protection", Label: discovery_kit_api.PluralLabel{One: "DynamoDB deletion protection", Other: "DynamoDB deletion protection"}},
		{Attribute: "aws.dynamodb.streams.enabled", Label: discovery_kit_api.PluralLabel{One: "DynamoDB streams", Other: "DynamoDB streams"}},
		{Attribute: "aws.dynamodb.streams.view-type", Label: discovery_kit_api.PluralLabel{One: "DynamoDB streams view type", Other: "DynamoDB streams view types"}},
		{Attribute: "aws.dynamodb.sse.type", Label: discovery_kit_api.PluralLabel{One: "DynamoDB SSE type", Other: "DynamoDB SSE types"}},
		{Attribute: "aws.dynamodb.ttl.enabled", Label: discovery_kit_api.PluralLabel{One: "DynamoDB TTL", Other: "DynamoDB TTL"}},
		{Attribute: "aws.dynamodb.gsi.count", Label: discovery_kit_api.PluralLabel{One: "DynamoDB GSI count", Other: "DynamoDB GSI counts"}},
		{Attribute: "aws.dynamodb.gsi.names", Label: discovery_kit_api.PluralLabel{One: "DynamoDB GSI name", Other: "DynamoDB GSI names"}},
		{Attribute: "aws.dynamodb.lsi.count", Label: discovery_kit_api.PluralLabel{One: "DynamoDB LSI count", Other: "DynamoDB LSI counts"}},
		{Attribute: "aws.dynamodb.global-table.replicas", Label: discovery_kit_api.PluralLabel{One: "DynamoDB global table replica region", Other: "DynamoDB global table replica regions"}},
		{Attribute: "aws.dynamodb.global-table.version", Label: discovery_kit_api.PluralLabel{One: "DynamoDB global table version", Other: "DynamoDB global table versions"}},
		{Attribute: "aws.dynamodb.autoscaling.read.enabled", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling read", Other: "DynamoDB autoscaling read"}},
		{Attribute: "aws.dynamodb.autoscaling.read.min", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling read min", Other: "DynamoDB autoscaling read min"}},
		{Attribute: "aws.dynamodb.autoscaling.read.max", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling read max", Other: "DynamoDB autoscaling read max"}},
		{Attribute: "aws.dynamodb.autoscaling.write.enabled", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling write", Other: "DynamoDB autoscaling write"}},
		{Attribute: "aws.dynamodb.autoscaling.write.min", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling write min", Other: "DynamoDB autoscaling write min"}},
		{Attribute: "aws.dynamodb.autoscaling.write.max", Label: discovery_kit_api.PluralLabel{One: "DynamoDB autoscaling write max", Other: "DynamoDB autoscaling write max"}},
	}
}

func (d *tableDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getTableTargets, ctx, "dynamodb-table")
}

func getTableTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	ddb := dynamodb.NewFromConfig(account.AwsConfig)
	aas := applicationautoscaling.NewFromConfig(account.AwsConfig)
	result, err := getAllTables(ctx, ddb, aas, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover DynamoDB tables for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_DYNAMODB=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

// scalableTargetKey identifies a single scalable resource: e.g. (table/foo, dynamodb:table:ReadCapacityUnits)
type scalableTargetKey struct {
	resourceId string
	dimension  aastypes.ScalableDimension
}

func getAllTables(ctx context.Context, ddb DynamodbApi, aas AppAutoScalingApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	scalableTargets, err := fetchDynamodbScalableTargets(ctx, aas)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch DynamoDB application-autoscaling targets. Autoscaling attributes will be missing.")
		scalableTargets = map[scalableTargetKey]aastypes.ScalableTarget{}
	}

	result := make([]discovery_kit_api.Target, 0, 20)
	paginator := dynamodb.NewListTablesPaginator(ddb, &dynamodb.ListTablesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, name := range output.TableNames {
			described, err := ddb.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(name)})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to describe DynamoDB table %s", name)
				continue
			}
			if described.Table == nil {
				continue
			}

			tags, err := fetchTableTags(ctx, ddb, described.Table.TableArn)
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to fetch tags for DynamoDB table %s", name)
			}
			if !matchesTableTagFilter(tags, account.TagFilters) {
				continue
			}

			pitr := fetchPITR(ctx, ddb, name)
			ttl := fetchTTL(ctx, ddb, name)

			result = append(result, toTableTarget(described.Table, tags, pitr, ttl, scalableTargets, account.AccountNumber, account.Region, account.AssumeRole))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesDynamodb), nil
}

func fetchDynamodbScalableTargets(ctx context.Context, aas AppAutoScalingApi) (map[scalableTargetKey]aastypes.ScalableTarget, error) {
	out := make(map[scalableTargetKey]aastypes.ScalableTarget)
	paginator := applicationautoscaling.NewDescribeScalableTargetsPaginator(aas, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: aastypes.ServiceNamespaceDynamodb,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range page.ScalableTargets {
			if t.ResourceId == nil {
				continue
			}
			out[scalableTargetKey{resourceId: *t.ResourceId, dimension: t.ScalableDimension}] = t
		}
	}
	return out, nil
}

func fetchTableTags(ctx context.Context, ddb DynamodbApi, arn *string) ([]types.Tag, error) {
	if arn == nil {
		return nil, nil
	}
	tags := make([]types.Tag, 0)
	var nextToken *string
	for {
		out, err := ddb.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{ResourceArn: arn, NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		tags = append(tags, out.Tags...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return tags, nil
}

func fetchPITR(ctx context.Context, ddb DynamodbApi, tableName string) *bool {
	out, err := ddb.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{TableName: aws.String(tableName)})
	if err != nil {
		log.Debug().Err(err).Msgf("DescribeContinuousBackups failed for table %s", tableName)
		return nil
	}
	if out.ContinuousBackupsDescription == nil || out.ContinuousBackupsDescription.PointInTimeRecoveryDescription == nil {
		return nil
	}
	enabled := out.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus == types.PointInTimeRecoveryStatusEnabled
	return &enabled
}

func fetchTTL(ctx context.Context, ddb DynamodbApi, tableName string) *bool {
	out, err := ddb.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{TableName: aws.String(tableName)})
	if err != nil {
		log.Debug().Err(err).Msgf("DescribeTimeToLive failed for table %s", tableName)
		return nil
	}
	if out.TimeToLiveDescription == nil {
		return nil
	}
	enabled := out.TimeToLiveDescription.TimeToLiveStatus == types.TimeToLiveStatusEnabled || out.TimeToLiveDescription.TimeToLiveStatus == types.TimeToLiveStatusEnabling
	return &enabled
}

func matchesTableTagFilter(tags []types.Tag, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == filter.Key {
				for _, v := range filter.Values {
					if tag.Value != nil && *tag.Value == v {
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

func toTableTarget(t *types.TableDescription, tags []types.Tag, pitr *bool, ttl *bool, scalable map[scalableTargetKey]aastypes.ScalableTarget, account string, region string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(t.TableArn)
	name := aws.ToString(t.TableName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.dynamodb.table.name"] = []string{name}

	if t.BillingModeSummary != nil && t.BillingModeSummary.BillingMode != "" {
		attributes["aws.dynamodb.billing-mode"] = []string{string(t.BillingModeSummary.BillingMode)}
	} else if t.ProvisionedThroughput != nil {
		// Tables created without explicit BillingModeSummary default to PROVISIONED.
		attributes["aws.dynamodb.billing-mode"] = []string{string(types.BillingModeProvisioned)}
	}

	if t.TableClassSummary != nil && t.TableClassSummary.TableClass != "" {
		attributes["aws.dynamodb.table-class"] = []string{string(t.TableClassSummary.TableClass)}
	}

	if t.DeletionProtectionEnabled != nil {
		attributes["aws.dynamodb.deletion-protection"] = []string{strconv.FormatBool(*t.DeletionProtectionEnabled)}
	}

	if pitr != nil {
		attributes["aws.dynamodb.pitr.enabled"] = []string{strconv.FormatBool(*pitr)}
	}
	if ttl != nil {
		attributes["aws.dynamodb.ttl.enabled"] = []string{strconv.FormatBool(*ttl)}
	}

	if t.StreamSpecification != nil && t.StreamSpecification.StreamEnabled != nil {
		attributes["aws.dynamodb.streams.enabled"] = []string{strconv.FormatBool(*t.StreamSpecification.StreamEnabled)}
		if t.StreamSpecification.StreamViewType != "" {
			attributes["aws.dynamodb.streams.view-type"] = []string{string(t.StreamSpecification.StreamViewType)}
		}
	} else {
		attributes["aws.dynamodb.streams.enabled"] = []string{"false"}
	}

	if t.SSEDescription != nil && t.SSEDescription.SSEType != "" {
		attributes["aws.dynamodb.sse.type"] = []string{string(t.SSEDescription.SSEType)}
	} else {
		attributes["aws.dynamodb.sse.type"] = []string{"AWS_OWNED"}
	}

	gsiNames := make([]string, 0, len(t.GlobalSecondaryIndexes))
	for _, gsi := range t.GlobalSecondaryIndexes {
		if gsi.IndexName != nil {
			gsiNames = append(gsiNames, *gsi.IndexName)
		}
	}
	sort.Strings(gsiNames)
	attributes["aws.dynamodb.gsi.count"] = []string{strconv.Itoa(len(gsiNames))}
	if len(gsiNames) > 0 {
		attributes["aws.dynamodb.gsi.names"] = gsiNames
		for _, gsi := range t.GlobalSecondaryIndexes {
			if gsi.IndexName == nil {
				continue
			}
			gsiName := *gsi.IndexName
			gsiKey := strings.ToLower(gsiName)
			if gsi.ProvisionedThroughput != nil &&
				gsi.ProvisionedThroughput.ReadCapacityUnits != nil &&
				gsi.ProvisionedThroughput.WriteCapacityUnits != nil &&
				(*gsi.ProvisionedThroughput.ReadCapacityUnits > 0 || *gsi.ProvisionedThroughput.WriteCapacityUnits > 0) {
				attributes[fmt.Sprintf("aws.dynamodb.gsi.%s.billing-mode", gsiKey)] = []string{string(types.BillingModeProvisioned)}
			} else {
				attributes[fmt.Sprintf("aws.dynamodb.gsi.%s.billing-mode", gsiKey)] = []string{string(types.BillingModePayPerRequest)}
			}
			gsiResourceId := fmt.Sprintf("table/%s/index/%s", name, gsiName)
			addAutoscalingAttributes(attributes, fmt.Sprintf("aws.dynamodb.autoscaling.gsi.%s", gsiKey), gsiResourceId, scalable, aastypes.ScalableDimensionDynamoDBIndexReadCapacityUnits, aastypes.ScalableDimensionDynamoDBIndexWriteCapacityUnits)
		}
	}

	attributes["aws.dynamodb.lsi.count"] = []string{strconv.Itoa(len(t.LocalSecondaryIndexes))}

	if len(t.Replicas) > 0 {
		regions := make([]string, 0, len(t.Replicas))
		for _, r := range t.Replicas {
			if r.RegionName != nil {
				regions = append(regions, *r.RegionName)
			}
		}
		sort.Strings(regions)
		attributes["aws.dynamodb.global-table.replicas"] = regions
	}
	if t.GlobalTableVersion != nil && *t.GlobalTableVersion != "" {
		attributes["aws.dynamodb.global-table.version"] = []string{*t.GlobalTableVersion}
	}

	tableResourceId := fmt.Sprintf("table/%s", name)
	addAutoscalingAttributes(attributes, "aws.dynamodb.autoscaling", tableResourceId, scalable, aastypes.ScalableDimensionDynamoDBTableReadCapacityUnits, aastypes.ScalableDimensionDynamoDBTableWriteCapacityUnits)

	for _, tag := range tags {
		if tag.Key == nil {
			continue
		}
		attributes[fmt.Sprintf("aws.dynamodb.label.%s", strings.ToLower(*tag.Key))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: tableTargetId,
		Attributes: attributes,
	}
}

func addAutoscalingAttributes(attributes map[string][]string, prefix string, resourceId string, scalable map[scalableTargetKey]aastypes.ScalableTarget, readDim aastypes.ScalableDimension, writeDim aastypes.ScalableDimension) {
	for _, dim := range []struct {
		dim     aastypes.ScalableDimension
		subName string
	}{{readDim, "read"}, {writeDim, "write"}} {
		key := scalableTargetKey{resourceId: resourceId, dimension: dim.dim}
		if t, ok := scalable[key]; ok {
			attributes[prefix+"."+dim.subName+".enabled"] = []string{"true"}
			if t.MinCapacity != nil {
				attributes[prefix+"."+dim.subName+".min"] = []string{strconv.Itoa(int(*t.MinCapacity))}
			}
			if t.MaxCapacity != nil {
				attributes[prefix+"."+dim.subName+".max"] = []string{strconv.Itoa(int(*t.MaxCapacity))}
			}
		} else {
			attributes[prefix+"."+dim.subName+".enabled"] = []string{"false"}
		}
	}
}
