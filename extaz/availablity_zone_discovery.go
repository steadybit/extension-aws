// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extaz

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	types2 "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"os"
)

var (
	targets        *[]discovery_kit_api.Target
	discoveryError *extension_kit.ExtensionError
)

func RegisterDiscoveryHandlers(stopCh chan os.Signal) {
	exthttp.RegisterHttpHandler("/az/discovery", exthttp.GetterAsHandler(getAZDiscoveryDescription))
	exthttp.RegisterHttpHandler("/az/discovery/target-description", exthttp.GetterAsHandler(getAZTargetDescription))
	exthttp.RegisterHttpHandler("/az/discovery/discovered-targets", getAZDiscoveryResults)
	utils.StartDiscoveryTask(
		stopCh,
		"availability zone",
		config.Config.DiscoveryIntervalZone,
		getTargetsForAccount,
		func(updatedTargets []discovery_kit_api.Target, err *extension_kit.ExtensionError) {
			targets = &updatedTargets
			discoveryError = err
		})
}

func getAZDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         azTargetType,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/az/discovery/discovered-targets",
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalZone)),
		},
	}
}

func getAZTargetDescription() discovery_kit_api.TargetDescription {
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

func getAZDiscoveryResults(w http.ResponseWriter, r *http.Request, _ []byte) {
	if discoveryError != nil {
		exthttp.WriteError(w, *discoveryError)
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveryData{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := ec2.NewFromConfig(account.AwsConfig)
	result, err := GetAllAvailabilityZones(ctx, client, account.AccountNumber)
	if err != nil {
		return nil, err
	}
	return &result, nil
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

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesZone), nil
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
		TargetType: azTargetType,
		Attributes: attributes,
	}
}
