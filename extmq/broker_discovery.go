// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extmq

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type brokerDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*brokerDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*brokerDiscovery)(nil)
)

func NewBrokerDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&brokerDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalMq)*time.Second),
	)
}

func (d *brokerDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: brokerTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalMq)),
		},
	}
}

func (d *brokerDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       brokerTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "Amazon MQ broker", Other: "Amazon MQ brokers"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(mqIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.mq.broker.engine-type"},
				{Attribute: "aws.mq.broker.deployment-mode"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *brokerDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.mq.broker.id", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker ID", Other: "AWS MQ broker IDs"}},
		{Attribute: "aws.mq.broker.name", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker name", Other: "AWS MQ broker names"}},
		{Attribute: "aws.mq.broker.engine-type", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker engine type", Other: "AWS MQ broker engine types"}},
		{Attribute: "aws.mq.broker.engine-version", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker engine version", Other: "AWS MQ broker engine versions"}},
		{Attribute: "aws.mq.broker.deployment-mode", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker deployment mode", Other: "AWS MQ broker deployment modes"}},
		{Attribute: "aws.mq.broker.host-instance-type", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker host instance type", Other: "AWS MQ broker host instance types"}},
		{Attribute: "aws.mq.broker.publicly-accessible", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker public exposure", Other: "AWS MQ broker public exposure"}},
		{Attribute: "aws.mq.broker.auto-minor-version-upgrade", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker auto minor version upgrade", Other: "AWS MQ broker auto minor version upgrade"}},
		{Attribute: "aws.mq.broker.subnets", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker subnet", Other: "AWS MQ broker subnets"}},
		{Attribute: "aws.mq.broker.storage-type", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker storage type", Other: "AWS MQ broker storage types"}},
		{Attribute: "aws.mq.broker.encryption.use-aws-owned-key", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker AWS-owned KMS key", Other: "AWS MQ broker AWS-owned KMS keys"}},
		{Attribute: "aws.mq.broker.authentication-strategy", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker authentication strategy", Other: "AWS MQ broker authentication strategies"}},
		{Attribute: "aws.mq.broker.maintenance-window", Label: discovery_kit_api.PluralLabel{One: "AWS MQ broker maintenance window", Other: "AWS MQ broker maintenance windows"}},
	}
}

func (d *brokerDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getBrokerTargets, ctx, "mq-broker")
}

func getBrokerTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := mq.NewFromConfig(account.AwsConfig)
	result, err := getAllBrokers(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover Amazon MQ brokers for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_MQ=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllBrokers(ctx context.Context, client MqApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 10)
	paginator := mq.NewListBrokersPaginator(client, &mq.ListBrokersInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, summary := range output.BrokerSummaries {
			described, err := client.DescribeBroker(ctx, &mq.DescribeBrokerInput{BrokerId: summary.BrokerId})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to describe Amazon MQ broker %s", aws.ToString(summary.BrokerId))
				continue
			}
			if !matchesBrokerTagFilter(described.Tags, account.TagFilters) {
				continue
			}
			result = append(result, toBrokerTarget(described, account.AccountNumber, account.Region, account.AssumeRole))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesMq), nil
}

func matchesBrokerTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		if value, ok := tags[filter.Key]; ok {
			if slices.Contains(filter.Values, value) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toBrokerTarget(b *mq.DescribeBrokerOutput, account string, region string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(b.BrokerArn)
	id := aws.ToString(b.BrokerId)
	name := aws.ToString(b.BrokerName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.mq.broker.id"] = []string{id}
	attributes["aws.mq.broker.name"] = []string{name}

	if b.EngineType != "" {
		attributes["aws.mq.broker.engine-type"] = []string{string(b.EngineType)}
	}
	if b.EngineVersion != nil {
		attributes["aws.mq.broker.engine-version"] = []string{*b.EngineVersion}
	}
	if b.DeploymentMode != "" {
		attributes["aws.mq.broker.deployment-mode"] = []string{string(b.DeploymentMode)}
	}
	if b.HostInstanceType != nil {
		attributes["aws.mq.broker.host-instance-type"] = []string{*b.HostInstanceType}
	}
	if b.PubliclyAccessible != nil {
		attributes["aws.mq.broker.publicly-accessible"] = []string{strconv.FormatBool(*b.PubliclyAccessible)}
	}
	if b.AutoMinorVersionUpgrade != nil {
		attributes["aws.mq.broker.auto-minor-version-upgrade"] = []string{strconv.FormatBool(*b.AutoMinorVersionUpgrade)}
	}
	if len(b.SubnetIds) > 0 {
		subnets := append([]string(nil), b.SubnetIds...)
		sort.Strings(subnets)
		attributes["aws.mq.broker.subnets"] = subnets
	}
	if b.StorageType != "" {
		attributes["aws.mq.broker.storage-type"] = []string{string(b.StorageType)}
	}
	if b.EncryptionOptions != nil && b.EncryptionOptions.UseAwsOwnedKey != nil {
		attributes["aws.mq.broker.encryption.use-aws-owned-key"] = []string{strconv.FormatBool(*b.EncryptionOptions.UseAwsOwnedKey)}
	}
	if b.AuthenticationStrategy != "" {
		attributes["aws.mq.broker.authentication-strategy"] = []string{string(b.AuthenticationStrategy)}
	}
	if b.MaintenanceWindowStartTime != nil && b.MaintenanceWindowStartTime.TimeOfDay != nil {
		tz := "UTC"
		if b.MaintenanceWindowStartTime.TimeZone != nil && *b.MaintenanceWindowStartTime.TimeZone != "" {
			tz = *b.MaintenanceWindowStartTime.TimeZone
		}
		attributes["aws.mq.broker.maintenance-window"] = []string{fmt.Sprintf("%s %s %s", string(b.MaintenanceWindowStartTime.DayOfWeek), aws.ToString(b.MaintenanceWindowStartTime.TimeOfDay), tz)}
	}

	for k, v := range b.Tags {
		attributes[fmt.Sprintf("aws.mq.broker.label.%s", strings.ToLower(k))] = []string{v}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: brokerTargetId,
		Attributes: attributes,
	}
}
