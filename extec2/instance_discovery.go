package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterEc2InstanceDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/ec2/instance/discovery", exthttp.GetterAsHandler(getEc2InstanceDiscoveryDescription))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/target-description", exthttp.GetterAsHandler(getEc2InstanceTargetDescription))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/attribute-descriptions", exthttp.GetterAsHandler(getEc2InstanceAttributeDescriptions))
	exthttp.RegisterHttpHandler("/ec2/instance/discovery/discovered-targets", getEc2InstanceTargets)
}

func getEc2InstanceDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         ec2TargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/ec2/instance/discovery/discovered-targets",
			CallInterval: extutil.Ptr("30s"),
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
			},
		},
	}
}

func getEc2InstanceTargets(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect EC2 instance information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	targets, err := GetAllEc2Instances(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		return nil, err
	}
	return &targets, nil
}

func mergeTargets(merged []discovery_kit_api.Target, eachResult []discovery_kit_api.Target) ([]discovery_kit_api.Target, error) {
	return append(merged, eachResult...), nil
}

type Ec2DescribeInstancesApi interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func GetAllEc2Instances(ctx context.Context, ec2Api Ec2DescribeInstancesApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	var nextToken *string = nil
	for {
		output, err := ec2Api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return result, err
		}

		for _, reservation := range output.Reservations {
			for _, ec2Instance := range reservation.Instances {
				result = append(result, toTarget(ec2Instance, awsAccountNumber, awsRegion))
			}
		}

		if output.NextToken == nil {
			break
		} else {
			nextToken = output.NextToken
		}
	}

	return result, nil
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

	return discovery_kit_api.Target{
		Id:         *ec2Instance.InstanceId,
		Label:      label,
		TargetType: ec2TargetId,
		Attributes: attributes,
	}
}
