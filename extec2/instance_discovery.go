// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"os"
	"strings"
)

var (
	targets        *[]discovery_kit_api.Target
	discoveryError *extension_kit.ExtensionError
)

func RegisterDiscoveryHandlers(stopCh chan os.Signal) {
	exthttp.RegisterHttpHandler("/ec2/instance/discovery", exthttp.GetterAsHandler(getEc2InstanceDiscoveryDescription))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/target-description", exthttp.GetterAsHandler(getEc2InstanceTargetDescription))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/attribute-descriptions", exthttp.GetterAsHandler(getEc2InstanceAttributeDescriptions))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/discovered-targets", getEc2InstanceTargets)
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/rules/ec2-to-host", exthttp.GetterAsHandler(getEc2InstanceToHostEnrichmentRule))

	log.Info().Msgf("Enriching EC2 data for target types: %v", config.Config.EnrichEc2DataForTargetTypes)
	for _, targetType := range config.Config.EnrichEc2DataForTargetTypes {
		exthttp.RegisterHttpHandler(fmt.Sprintf("/ec2/instance/discovery/rules/ec2-to-%s", targetType), exthttp.GetterAsHandler(getEc2InstanceToXEnrichmentRule(targetType)))
	}

	utils.StartDiscoveryTask(
		stopCh,
		"ec2 instance",
		config.Config.DiscoveryIntervalEc2,
		getTargetsForAccount,
		func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError) {
			targets = &updatedTargets
			discoveryError = err
		})
}

func getEc2InstanceDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         ec2TargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/ec2/instance/discovery/discovered-targets",
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalEc2)),
		},
	}
}

func getEc2InstanceTargetDescription() discovery_kit_api.TargetDescription {
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

func getEc2InstanceToHostEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_aws.ec2-instance-to-host",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: ec2TargetId,
			Selector: map[string]string{
				"aws-ec2.hostname.internal": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_host.host",
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

func getEc2InstanceToXEnrichmentRule(destTargetType string) func() discovery_kit_api.TargetEnrichmentRule {
	id := fmt.Sprintf("com.steadybit.extension_aws.ec2-instance-to-%s", destTargetType)
	return func() discovery_kit_api.TargetEnrichmentRule {
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
					Name:    "aws-ec2.instance.id",
				},
				{
					Matcher: discovery_kit_api.StartsWith,
					Name:    "aws-ec2.label.",
				},
			},
		}
	}
}

func getEc2InstanceAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
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
		},
	}
}

func getEc2InstanceTargets(w http.ResponseWriter, _ *http.Request, _ []byte) {
	if discoveryError != nil {
		exthttp.WriteError(w, *discoveryError)
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveryData{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := GetAllEc2Instances(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover ec2-instances for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EC2=true. Details: %s", account.AccountNumber, re.Error())
			return extutil.Ptr([]discovery_kit_api.Target{}), nil
		}
		return nil, err
	}
	return &result, nil
}

type Ec2DescribeInstancesApi interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func GetAllEc2Instances(ctx context.Context, ec2Api Ec2DescribeInstancesApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := ec2.NewDescribeInstancesPaginator(ec2Api, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}
		for _, reservation := range output.Reservations {
			for _, ec2Instance := range reservation.Instances {
				result = append(result, toTarget(ec2Instance, awsAccountNumber, awsRegion))
			}
		}
	}

	return discovery_kit_api.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributeExcludesEc2), nil
}

func toTarget(ec2Instance types.Instance, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
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

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws-ec2.image"] = []string{aws.ToString(ec2Instance.ImageId)}
	attributes["aws.zone"] = []string{aws.ToString(ec2Instance.Placement.AvailabilityZone)}
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
