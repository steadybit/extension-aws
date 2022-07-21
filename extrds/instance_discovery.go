// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func RegisterRdsDiscoveryHandlers() {
	utils.RegisterHttpHandler("/rds/instance/discovery", utils.GetterAsHandler(getRdsInstanceDiscoveryDescription))
	utils.RegisterHttpHandler("/rds/instance/discovery/target-description", utils.GetterAsHandler(getRdsInstanceTargetDescription))
	utils.RegisterHttpHandler("/rds/instance/discovery/attribute-descriptions", utils.GetterAsHandler(getRdsInstanceAttributeDescriptions))
	utils.RegisterHttpHandler("/rds/instance/discovery/discovered-targets", getRdsInstanceDiscoveryResults)
}

func getRdsInstanceDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         "com.github.steadybit.extension_aws.rds",
		RestrictTo: discovery_kit_api.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/rds/instance/discovery/discovered-targets",
			CallInterval: discovery_kit_api.Ptr("30s"),
		},
	}
}

func getRdsInstanceTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       rdsTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "RDS instance", Other: "RDS instances"},
		Category: discovery_kit_api.Ptr("cloud"),
		Version:  "1.0.0",
		Icon:     discovery_kit_api.Ptr(rdsIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.rds.cluster"},
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
			},
		},
	}
}

func getRdsInstanceDiscoveryResults(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := getAllRdsInstances(r.Context())
	if err != nil {
		utils.WriteError(w, "Failed to collect RDS instance information", err)
	} else {
		utils.WriteBody(w, targets)
	}
}

func getAllRdsInstances(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(utils.AwsConfig)

	result := make([]discovery_kit_api.Target, 0, 20)

	var marker *string = nil
	for {
		output, err := client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
			Marker: marker,
		})
		if err != nil {
			return result, err
		}

		for _, dbInstance := range output.DBInstances {
			result = append(result, toTarget(dbInstance))
		}

		if output.Marker == nil {
			break
		} else {
			marker = output.Marker
		}
	}

	return result, nil
}

func toTarget(dbInstance types.DBInstance) discovery_kit_api.Target {
	arn := aws.ToString(dbInstance.DBInstanceArn)
	label := aws.ToString(dbInstance.DBInstanceIdentifier)

	attributes := make(map[string][]string)
	attributes["steadybit.label"] = []string{label}
	attributes["aws.account"] = []string{utils.AwsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.zone"] = []string{aws.ToString(dbInstance.AvailabilityZone)}
	attributes["aws.rds.engine"] = []string{aws.ToString(dbInstance.Engine)}
	attributes["aws.rds.instance.id"] = []string{label}

	if dbInstance.DBClusterIdentifier != nil {
		attributes["aws.rds.cluster"] = []string{aws.ToString(dbInstance.DBClusterIdentifier)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      label,
		TargetType: "com.github.steadybit.extension_aws.rds",
		Attributes: attributes,
	}
}
