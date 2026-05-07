// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exteks

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
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

type eksClusterDiscovery struct {
}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*eksClusterDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*eksClusterDiscovery)(nil)
)

func NewEksClusterDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &eksClusterDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalEks)*time.Second),
	)
}

func (d *eksClusterDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: clusterTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEks)),
		},
	}
}

func (d *eksClusterDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       clusterTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "EKS cluster", Other: "EKS clusters"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(eksIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.eks.cluster.version"},
				{Attribute: "aws.eks.cluster.endpoint-public-access"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{Attribute: "steadybit.label", Direction: "ASC"},
			},
		},
	}
}

func (d *eksClusterDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.eks.cluster.name", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster name", Other: "AWS EKS cluster names"}},
		{Attribute: "aws.eks.cluster.version", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster version", Other: "AWS EKS cluster versions"}},
		{Attribute: "aws.eks.cluster.platform-version", Label: discovery_kit_api.PluralLabel{One: "AWS EKS platform version", Other: "AWS EKS platform versions"}},
		{Attribute: "aws.eks.cluster.endpoint-public-access", Label: discovery_kit_api.PluralLabel{One: "AWS EKS public endpoint", Other: "AWS EKS public endpoints"}},
		{Attribute: "aws.eks.cluster.endpoint-private-access", Label: discovery_kit_api.PluralLabel{One: "AWS EKS private endpoint", Other: "AWS EKS private endpoints"}},
		{Attribute: "aws.eks.cluster.public-access-cidrs", Label: discovery_kit_api.PluralLabel{One: "AWS EKS public-access CIDR", Other: "AWS EKS public-access CIDRs"}},
		{Attribute: "aws.eks.cluster.public-access-open-to-internet", Label: discovery_kit_api.PluralLabel{One: "AWS EKS public-access open to the internet", Other: "AWS EKS public-access open to the internet"}},
		{Attribute: "aws.eks.cluster.subnets", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster subnet", Other: "AWS EKS cluster subnets"}},
		{Attribute: "aws.eks.cluster.vpc", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster VPC", Other: "AWS EKS cluster VPCs"}},
		{Attribute: "aws.eks.cluster.logging.enabled-types", Label: discovery_kit_api.PluralLabel{One: "AWS EKS enabled log type", Other: "AWS EKS enabled log types"}},
		{Attribute: "aws.eks.cluster.logging.disabled-types", Label: discovery_kit_api.PluralLabel{One: "AWS EKS disabled log type", Other: "AWS EKS disabled log types"}},
		{Attribute: "aws.eks.cluster.secrets-encryption.enabled", Label: discovery_kit_api.PluralLabel{One: "AWS EKS secrets-encryption", Other: "AWS EKS secrets-encryption"}},
		{Attribute: "aws.eks.cluster.deletion-protection", Label: discovery_kit_api.PluralLabel{One: "AWS EKS deletion-protection", Other: "AWS EKS deletion-protection"}},
		{Attribute: "aws.eks.cluster.status", Label: discovery_kit_api.PluralLabel{One: "AWS EKS cluster status", Other: "AWS EKS cluster status"}},
	}
}

func (d *eksClusterDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getEksClusterTargets, ctx, "eks-cluster")
}

func getEksClusterTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := eks.NewFromConfig(account.AwsConfig)
	result, err := getAllEksClusters(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover EKS clusters for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EKS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllEksClusters(ctx context.Context, client EksApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 10)
	paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, name := range output.Clusters {
			described, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to describe EKS cluster %s", name)
				continue
			}
			if described.Cluster == nil {
				continue
			}
			if !matchesClusterTagFilter(described.Cluster.Tags, account.TagFilters) {
				continue
			}
			result = append(result, toClusterTarget(*described.Cluster, account.AccountNumber, account.Region, account.AssumeRole))
		}
	}
	return result, nil
}

func matchesClusterTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		if value, ok := tags[filter.Key]; ok {
			for _, v := range filter.Values {
				if value == v {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toClusterTarget(cluster types.Cluster, account string, region string, role *string) discovery_kit_api.Target {
	arn := aws.ToString(cluster.Arn)
	name := aws.ToString(cluster.Name)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.eks.cluster.name"] = []string{name}
	attributes["aws.eks.cluster.status"] = []string{string(cluster.Status)}

	if cluster.Version != nil {
		attributes["aws.eks.cluster.version"] = []string{*cluster.Version}
	}
	if cluster.PlatformVersion != nil {
		attributes["aws.eks.cluster.platform-version"] = []string{*cluster.PlatformVersion}
	}

	if cluster.ResourcesVpcConfig != nil {
		vpc := cluster.ResourcesVpcConfig
		attributes["aws.eks.cluster.endpoint-public-access"] = []string{strconv.FormatBool(vpc.EndpointPublicAccess)}
		attributes["aws.eks.cluster.endpoint-private-access"] = []string{strconv.FormatBool(vpc.EndpointPrivateAccess)}
		if len(vpc.PublicAccessCidrs) > 0 {
			attributes["aws.eks.cluster.public-access-cidrs"] = vpc.PublicAccessCidrs
		}
		attributes["aws.eks.cluster.public-access-open-to-internet"] = []string{strconv.FormatBool(isOpenToInternet(vpc.EndpointPublicAccess, vpc.PublicAccessCidrs))}
		if len(vpc.SubnetIds) > 0 {
			subnets := append([]string(nil), vpc.SubnetIds...)
			sort.Strings(subnets)
			attributes["aws.eks.cluster.subnets"] = subnets
		}
		if vpc.VpcId != nil {
			attributes["aws.eks.cluster.vpc"] = []string{*vpc.VpcId}
		}
	}

	enabled, disabled := splitLogTypes(cluster.Logging)
	if len(enabled) > 0 {
		attributes["aws.eks.cluster.logging.enabled-types"] = enabled
	}
	if len(disabled) > 0 {
		attributes["aws.eks.cluster.logging.disabled-types"] = disabled
	}

	attributes["aws.eks.cluster.secrets-encryption.enabled"] = []string{strconv.FormatBool(hasSecretsEncryption(cluster.EncryptionConfig))}

	if cluster.DeletionProtection != nil {
		attributes["aws.eks.cluster.deletion-protection"] = []string{strconv.FormatBool(*cluster.DeletionProtection)}
	}

	for k, v := range cluster.Tags {
		attributes[fmt.Sprintf("aws.eks.cluster.label.%s", strings.ToLower(k))] = []string{v}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: clusterTargetId,
		Attributes: attributes,
	}
}

func splitLogTypes(logging *types.Logging) (enabled []string, disabled []string) {
	if logging == nil {
		return nil, nil
	}
	for _, ls := range logging.ClusterLogging {
		on := ls.Enabled != nil && *ls.Enabled
		for _, t := range ls.Types {
			if on {
				enabled = append(enabled, string(t))
			} else {
				disabled = append(disabled, string(t))
			}
		}
	}
	sort.Strings(enabled)
	sort.Strings(disabled)
	return enabled, disabled
}

func hasSecretsEncryption(configs []types.EncryptionConfig) bool {
	for _, c := range configs {
		for _, r := range c.Resources {
			if strings.EqualFold(r, "secrets") {
				return true
			}
		}
	}
	return false
}

func isOpenToInternet(publicEndpoint bool, cidrs []string) bool {
	if !publicEndpoint {
		return false
	}
	if len(cidrs) == 0 {
		return true
	}
	for _, c := range cidrs {
		if c == "0.0.0.0/0" || c == "::/0" {
			return true
		}
	}
	return false
}
