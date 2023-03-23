// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
)

func RegisterRdsDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/rds/instance/discovery", exthttp.GetterAsHandler(getRdsInstanceDiscoveryDescription))
	exthttp.RegisterHttpHandler("/rds/instance/discovery/target-description", exthttp.GetterAsHandler(getRdsInstanceTargetDescription))
	exthttp.RegisterHttpHandler("/rds/instance/discovery/attribute-descriptions", exthttp.GetterAsHandler(getRdsInstanceAttributeDescriptions))
	exthttp.RegisterHttpHandler("/rds/instance/discovery/discovered-targets", getRdsInstanceDiscoveryResults)
}

func getRdsInstanceDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         rdsTargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/rds/instance/discovery/discovered-targets",
			CallInterval: extutil.Ptr("30s"),
		},
	}
}

func getRdsInstanceTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       rdsTargetId,
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

func getRdsInstanceAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
			{
				Attribute: "aws.rds.engine",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS RDS database engine",
					Other: "AWS RDS database engines",
				},
			}, {
				Attribute: "aws.rds.cluster",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS RDS cluster",
					Other: "AWS RDS clusters",
				},
			}, {
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
		},
	}
}

func getRdsInstanceDiscoveryResults(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect RDS instance information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(account.AwsConfig)
	targets, err := GetAllRdsInstances(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		return nil, err
	}
	return &targets, nil
}

func mergeTargets(merged []discovery_kit_api.Target, eachResult []discovery_kit_api.Target) ([]discovery_kit_api.Target, error) {
	return append(merged, eachResult...), nil
}

type RdsDescribeInstancesApi interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

func GetAllRdsInstances(ctx context.Context, rdsApi RdsDescribeInstancesApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	var marker *string = nil
	for {
		output, err := rdsApi.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
			Marker: marker,
		})
		if err != nil {
			return result, err
		}

		for _, dbInstance := range output.DBInstances {
			result = append(result, toTarget(dbInstance, awsAccountNumber, awsRegion))
		}

		if output.Marker == nil {
			break
		} else {
			marker = output.Marker
		}
	}

	return result, nil
}

func toTarget(dbInstance types.DBInstance, awsAccountNumber string, awsRegion string) discovery_kit_api.Target {
	arn := aws.ToString(dbInstance.DBInstanceArn)
	label := aws.ToString(dbInstance.DBInstanceIdentifier)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.zone"] = []string{aws.ToString(dbInstance.AvailabilityZone)}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.rds.engine"] = []string{aws.ToString(dbInstance.Engine)}
	attributes["aws.rds.instance.id"] = []string{label}
	attributes["aws.rds.instance.status"] = []string{aws.ToString(dbInstance.DBInstanceStatus)}

	if dbInstance.DBClusterIdentifier != nil {
		attributes["aws.rds.cluster"] = []string{aws.ToString(dbInstance.DBClusterIdentifier)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: rdsTargetId,
		Attributes: attributes,
	}
}
