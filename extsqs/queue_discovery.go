// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extsqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type queueDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*queueDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*queueDiscovery)(nil)
)

func NewQueueDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&queueDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalSqs)*time.Second),
	)
}

func (d *queueDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: queueTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalSqs)),
		},
	}
}

func (d *queueDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       queueTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "SQS queue", Other: "SQS queues"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(sqsIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.sqs.queue.fifo"},
				{Attribute: "aws.sqs.queue.dlq.configured"},
				{Attribute: "aws.sqs.queue.sqs-managed-sse-enabled"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *queueDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.sqs.queue.name", Label: discovery_kit_api.PluralLabel{One: "SQS queue name", Other: "SQS queue names"}},
		{Attribute: "aws.sqs.queue.url", Label: discovery_kit_api.PluralLabel{One: "SQS queue URL", Other: "SQS queue URLs"}},
		{Attribute: "aws.sqs.queue.fifo", Label: discovery_kit_api.PluralLabel{One: "SQS queue FIFO", Other: "SQS queue FIFO"}},
		{Attribute: "aws.sqs.queue.visibility-timeout", Label: discovery_kit_api.PluralLabel{One: "SQS queue visibility timeout", Other: "SQS queue visibility timeouts"}},
		{Attribute: "aws.sqs.queue.message-retention-seconds", Label: discovery_kit_api.PluralLabel{One: "SQS queue message retention", Other: "SQS queue message retention"}},
		{Attribute: "aws.sqs.queue.delay-seconds", Label: discovery_kit_api.PluralLabel{One: "SQS queue delay seconds", Other: "SQS queue delay seconds"}},
		{Attribute: "aws.sqs.queue.receive-wait-time-seconds", Label: discovery_kit_api.PluralLabel{One: "SQS queue receive wait time", Other: "SQS queue receive wait times"}},
		{Attribute: "aws.sqs.queue.dlq.configured", Label: discovery_kit_api.PluralLabel{One: "SQS queue DLQ configured", Other: "SQS queue DLQ configured"}},
		{Attribute: "aws.sqs.queue.dlq.arn", Label: discovery_kit_api.PluralLabel{One: "SQS queue DLQ ARN", Other: "SQS queue DLQ ARNs"}},
		{Attribute: "aws.sqs.queue.dlq.max-receive-count", Label: discovery_kit_api.PluralLabel{One: "SQS queue DLQ max receive count", Other: "SQS queue DLQ max receive counts"}},
		{Attribute: "aws.sqs.queue.kms-master-key-id", Label: discovery_kit_api.PluralLabel{One: "SQS queue KMS master key", Other: "SQS queue KMS master keys"}},
		{Attribute: "aws.sqs.queue.sqs-managed-sse-enabled", Label: discovery_kit_api.PluralLabel{One: "SQS queue SQS-managed SSE", Other: "SQS queue SQS-managed SSE"}},
		{Attribute: "aws.sqs.queue.content-based-deduplication", Label: discovery_kit_api.PluralLabel{One: "SQS queue content-based deduplication", Other: "SQS queue content-based deduplication"}},
	}
}

func (d *queueDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getQueueTargets, ctx, "sqs-queue")
}

func getQueueTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := sqs.NewFromConfig(account.AwsConfig)
	result, err := getAllQueues(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover SQS queues for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_SQS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllQueues(ctx context.Context, client SqsApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0)
	paginator := sqs.NewListQueuesPaginator(client, &sqs.ListQueuesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, url := range out.QueueUrls {
			attrsOut, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
				QueueUrl:       aws.String(url),
				AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameAll},
			})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to get attributes for SQS queue %s", url)
				continue
			}
			tags := map[string]string{}
			tagsOut, err := client.ListQueueTags(ctx, &sqs.ListQueueTagsInput{QueueUrl: aws.String(url)})
			if err != nil {
				log.Debug().Err(err).Msgf("Failed to list tags for SQS queue %s", url)
			} else if tagsOut != nil {
				tags = tagsOut.Tags
			}
			if !matchesSqsTagFilter(tags, account.TagFilters) {
				continue
			}
			result = append(result, toQueueTarget(url, attrsOut.Attributes, tags, account.AccountNumber, account.Region, account.AssumeRole))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesSqs), nil
}

func matchesSqsTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		if v, ok := tags[filter.Key]; ok {
			if slices.Contains(filter.Values, v) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toQueueTarget(url string, attrs map[string]string, tags map[string]string, account string, region string, role *string) discovery_kit_api.Target {
	queueArn := attrs[string(types.QueueAttributeNameQueueArn)]
	name := nameFromQueueUrl(url)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{queueArn}
	attributes["aws.sqs.queue.name"] = []string{name}
	attributes["aws.sqs.queue.url"] = []string{url}

	fifo := attrs[string(types.QueueAttributeNameFifoQueue)] == "true" || strings.HasSuffix(name, ".fifo")
	attributes["aws.sqs.queue.fifo"] = []string{strconv.FormatBool(fifo)}

	if v, ok := attrs[string(types.QueueAttributeNameVisibilityTimeout)]; ok {
		attributes["aws.sqs.queue.visibility-timeout"] = []string{v}
	}
	if v, ok := attrs[string(types.QueueAttributeNameMessageRetentionPeriod)]; ok {
		attributes["aws.sqs.queue.message-retention-seconds"] = []string{v}
	}
	if v, ok := attrs[string(types.QueueAttributeNameDelaySeconds)]; ok {
		attributes["aws.sqs.queue.delay-seconds"] = []string{v}
	}
	if v, ok := attrs[string(types.QueueAttributeNameReceiveMessageWaitTimeSeconds)]; ok {
		attributes["aws.sqs.queue.receive-wait-time-seconds"] = []string{v}
	}
	if v, ok := attrs[string(types.QueueAttributeNameContentBasedDeduplication)]; ok {
		attributes["aws.sqs.queue.content-based-deduplication"] = []string{v}
	}

	dlqArn, dlqMaxReceive := parseRedrivePolicy(attrs[string(types.QueueAttributeNameRedrivePolicy)])
	attributes["aws.sqs.queue.dlq.configured"] = []string{strconv.FormatBool(dlqArn != "")}
	if dlqArn != "" {
		attributes["aws.sqs.queue.dlq.arn"] = []string{dlqArn}
	}
	if dlqMaxReceive > 0 {
		attributes["aws.sqs.queue.dlq.max-receive-count"] = []string{strconv.Itoa(dlqMaxReceive)}
	}

	if v, ok := attrs[string(types.QueueAttributeNameKmsMasterKeyId)]; ok && v != "" {
		attributes["aws.sqs.queue.kms-master-key-id"] = []string{v}
	}
	if v, ok := attrs["SqsManagedSseEnabled"]; ok {
		attributes["aws.sqs.queue.sqs-managed-sse-enabled"] = []string{v}
	}

	for k, v := range tags {
		attributes[fmt.Sprintf("aws.sqs.queue.label.%s", strings.ToLower(k))] = []string{v}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         queueArn,
		Label:      name,
		TargetType: queueTargetType,
		Attributes: attributes,
	}
}

func nameFromQueueUrl(url string) string {
	idx := strings.LastIndex(url, "/")
	if idx >= 0 && idx+1 < len(url) {
		return url[idx+1:]
	}
	return url
}

// parseRedrivePolicy returns the DLQ ARN and maxReceiveCount from a queue's RedrivePolicy JSON.
func parseRedrivePolicy(raw string) (string, int) {
	if raw == "" {
		return "", 0
	}
	var p struct {
		DeadLetterTargetArn string `json:"deadLetterTargetArn"`
		MaxReceiveCount     any    `json:"maxReceiveCount"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return "", 0
	}
	maxReceive := 0
	switch v := p.MaxReceiveCount.(type) {
	case float64:
		maxReceive = int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			maxReceive = n
		}
	}
	return p.DeadLetterTargetArn, maxReceive
}
