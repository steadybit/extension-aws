// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	types2 "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/az/discovery", exthttp.GetterAsHandler(getAZDiscoveryDescription))
	exthttp.RegisterHttpHandler("/az/discovery/target-description", exthttp.GetterAsHandler(getAZTargetDescription))
	exthttp.RegisterHttpHandler("/az/discovery/discovered-targets", getAZDiscoveryResults)
}

func getAZDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         azTargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/az/discovery/discovered-targets",
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func getAZTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       azTargetId,
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

func getAZDiscoveryResults(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect availability zones information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	targets, err := GetAllAvailabilityZones(ctx, client, account.AccountNumber)
	if err != nil {
		return nil, err
	}
	return &targets, nil
}

func mergeTargets(merged []discovery_kit_api.Target, eachResult []discovery_kit_api.Target) ([]discovery_kit_api.Target, error) {
	return append(merged, eachResult...), nil
}

type AZDescribeAvailabilityZonesApi interface {
	DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
}

func GetAllAvailabilityZones(ctx context.Context, ec2Api AZDescribeAvailabilityZonesApi, awsAccountNumber string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	output, err := ec2Api.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		AllAvailabilityZones: aws.Bool(false),
	})
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover availability zones for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE=true. Details: %s", awsAccountNumber, re.Error())
			return result, nil
		}
		return result, err
	}

	for _, availabilityZone := range output.AvailabilityZones {
		result = append(result, toTarget(availabilityZone, awsAccountNumber))
	}

	return result, nil
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
		TargetType: azTargetId,
		Attributes: attributes,
	}
}
