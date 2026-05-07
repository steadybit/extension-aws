// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extec2

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type ebsVolumeDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*ebsVolumeDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*ebsVolumeDiscovery)(nil)
)

func NewEbsVolumeDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&ebsVolumeDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEbs)*time.Second),
	)
}

func (d *ebsVolumeDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: ebsTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEbs)),
		},
	}
}

func (d *ebsVolumeDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       ebsTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "EBS volume", Other: "EBS volumes"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(ebsIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.ebs.volume.type"},
				{Attribute: "aws.ebs.volume.size-gb"},
				{Attribute: "aws.ebs.volume.encrypted"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *ebsVolumeDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.ebs.volume.id", Label: discovery_kit_api.PluralLabel{One: "EBS volume ID", Other: "EBS volume IDs"}},
		{Attribute: "aws.ebs.volume.type", Label: discovery_kit_api.PluralLabel{One: "EBS volume type", Other: "EBS volume types"}},
		{Attribute: "aws.ebs.volume.size-gb", Label: discovery_kit_api.PluralLabel{One: "EBS volume size (GiB)", Other: "EBS volume sizes (GiB)"}},
		{Attribute: "aws.ebs.volume.iops", Label: discovery_kit_api.PluralLabel{One: "EBS volume IOPS", Other: "EBS volume IOPS"}},
		{Attribute: "aws.ebs.volume.throughput", Label: discovery_kit_api.PluralLabel{One: "EBS volume throughput (MiB/s)", Other: "EBS volume throughputs (MiB/s)"}},
		{Attribute: "aws.ebs.volume.multi-attach.enabled", Label: discovery_kit_api.PluralLabel{One: "EBS volume multi-attach", Other: "EBS volume multi-attach"}},
		{Attribute: "aws.ebs.volume.kms-key-id", Label: discovery_kit_api.PluralLabel{One: "EBS volume KMS key", Other: "EBS volume KMS keys"}},
		{Attribute: "aws.ebs.volume.attachment.instance-id", Label: discovery_kit_api.PluralLabel{One: "EBS volume attachment instance", Other: "EBS volume attachment instances"}},
		{Attribute: "aws.ebs.volume.attachment.device", Label: discovery_kit_api.PluralLabel{One: "EBS volume attachment device", Other: "EBS volume attachment devices"}},
		{Attribute: "aws.ebs.volume.attachment.delete-on-termination", Label: discovery_kit_api.PluralLabel{One: "EBS volume delete-on-termination", Other: "EBS volume delete-on-termination"}},
		{Attribute: "aws.ebs.volume.snapshot.most-recent.created-at", Label: discovery_kit_api.PluralLabel{One: "EBS volume most-recent snapshot timestamp", Other: "EBS volume most-recent snapshot timestamps"}},
		{Attribute: "aws.ebs.volume.snapshot.most-recent.id", Label: discovery_kit_api.PluralLabel{One: "EBS volume most-recent snapshot ID", Other: "EBS volume most-recent snapshot IDs"}},
		{Attribute: "aws.ebs.volume.encrypted", Label: discovery_kit_api.PluralLabel{One: "EBS volume encrypted", Other: "EBS volume encrypted"}},
	}
}

func (d *ebsVolumeDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getEbsVolumeTargets, ctx, "ebs-volume")
}

func getEbsVolumeTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := getAllEbsVolumes(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover EBS volumes for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EBS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type ebsApi interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
}

func getAllEbsVolumes(ctx context.Context, client ebsApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	volumes, err := listAllVolumes(ctx, client)
	if err != nil {
		return nil, err
	}
	latestSnapshot, err := buildLatestSnapshotMap(ctx, client, volumes)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch EBS snapshots; snapshot.most-recent.* attributes will be missing.")
		latestSnapshot = map[string]types.Snapshot{}
	}

	result := make([]discovery_kit_api.Target, 0, len(volumes))
	for _, v := range volumes {
		if !matchesEc2TagFilter(v.Tags, account.TagFilters) {
			continue
		}
		result = append(result, toEbsVolumeTarget(v, latestSnapshot[aws.ToString(v.VolumeId)], account.AccountNumber, account.Region, account.AssumeRole))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesEbs), nil
}

func listAllVolumes(ctx context.Context, client ebsApi) ([]types.Volume, error) {
	result := make([]types.Volume, 0)
	var nextToken *string
	for {
		out, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		result = append(result, out.Volumes...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return result, nil
}

// buildLatestSnapshotMap returns the most recent COMPLETED snapshot per volume, keyed by volume id.
// Snapshots without start time or not in completed state are skipped.
func buildLatestSnapshotMap(ctx context.Context, client ebsApi, volumes []types.Volume) (map[string]types.Snapshot, error) {
	if len(volumes) == 0 {
		return map[string]types.Snapshot{}, nil
	}
	latest := make(map[string]types.Snapshot)
	var nextToken *string
	for {
		out, err := client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
			OwnerIds:  []string{"self"},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, s := range out.Snapshots {
			if s.VolumeId == nil || s.StartTime == nil {
				continue
			}
			if s.State != types.SnapshotStateCompleted {
				continue
			}
			vol := *s.VolumeId
			if cur, ok := latest[vol]; !ok || s.StartTime.After(*cur.StartTime) {
				latest[vol] = s
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return latest, nil
}

func toEbsVolumeTarget(v types.Volume, latestSnapshot types.Snapshot, account string, region string, role *string) discovery_kit_api.Target {
	id := aws.ToString(v.VolumeId)
	name := nameFromTags(v.Tags, id)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.ebs.volume.id"] = []string{id}

	if v.AvailabilityZone != nil {
		attributes["aws.zone"] = []string{*v.AvailabilityZone}
	}
	if v.VolumeType != "" {
		attributes["aws.ebs.volume.type"] = []string{string(v.VolumeType)}
	}
	if v.Size != nil {
		attributes["aws.ebs.volume.size-gb"] = []string{strconv.Itoa(int(*v.Size))}
	}
	if v.Iops != nil {
		attributes["aws.ebs.volume.iops"] = []string{strconv.Itoa(int(*v.Iops))}
	}
	if v.Throughput != nil {
		attributes["aws.ebs.volume.throughput"] = []string{strconv.Itoa(int(*v.Throughput))}
	}
	if v.MultiAttachEnabled != nil {
		attributes["aws.ebs.volume.multi-attach.enabled"] = []string{strconv.FormatBool(*v.MultiAttachEnabled)}
	}
	if v.Encrypted != nil {
		attributes["aws.ebs.volume.encrypted"] = []string{strconv.FormatBool(*v.Encrypted)}
	}
	if v.KmsKeyId != nil {
		attributes["aws.ebs.volume.kms-key-id"] = []string{*v.KmsKeyId}
	}

	if len(v.Attachments) > 0 {
		instanceIds := make([]string, 0, len(v.Attachments))
		devices := make([]string, 0, len(v.Attachments))
		dotValues := make([]string, 0, len(v.Attachments))
		for _, a := range v.Attachments {
			if a.InstanceId != nil {
				instanceIds = append(instanceIds, *a.InstanceId)
			}
			if a.Device != nil {
				devices = append(devices, *a.Device)
			}
			if a.DeleteOnTermination != nil {
				dotValues = append(dotValues, strconv.FormatBool(*a.DeleteOnTermination))
			}
		}
		if len(instanceIds) > 0 {
			attributes["aws.ebs.volume.attachment.instance-id"] = instanceIds
		}
		if len(devices) > 0 {
			attributes["aws.ebs.volume.attachment.device"] = devices
		}
		if len(dotValues) > 0 {
			attributes["aws.ebs.volume.attachment.delete-on-termination"] = dotValues
		}
	}

	if latestSnapshot.SnapshotId != nil && latestSnapshot.StartTime != nil {
		attributes["aws.ebs.volume.snapshot.most-recent.id"] = []string{*latestSnapshot.SnapshotId}
		// RFC3339 timestamp — immutable per snapshot, agent can compute age client-side.
		attributes["aws.ebs.volume.snapshot.most-recent.created-at"] = []string{latestSnapshot.StartTime.UTC().Format("2006-01-02T15:04:05Z")}
	}

	for _, tag := range v.Tags {
		if tag.Key == nil {
			continue
		}
		attributes[fmt.Sprintf("aws.ebs.volume.label.%s", strings.ToLower(*tag.Key))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      name,
		TargetType: ebsTargetType,
		Attributes: attributes,
	}
}
