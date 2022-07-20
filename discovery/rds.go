// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package discovery

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
	utils.RegisterHttpHandler("/discovery/rds", utils.GetterAsHandler(getRdsDiscoveryDescription))
	utils.RegisterHttpHandler("/discovery/rds/target-description", utils.GetterAsHandler(getRdsTargetDescription))
	utils.RegisterHttpHandler("/discovery/rds/attribute-descriptions", utils.GetterAsHandler(getRdsAttributeDescriptions))
	utils.RegisterHttpHandler("/discovery/rds/discovered-targets", getRdsDiscoveryResults)
}

func getRdsDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         "com.github.steadybit.extension_aws.rds",
		RestrictTo: discovery_kit_api.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/discovery/rds/discovered-targets",
			CallInterval: discovery_kit_api.Ptr("30s"),
		},
	}
}

func getRdsTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       "com.github.steadybit.extension_aws.rds",
		Label:    discovery_kit_api.PluralLabel{One: "RDS instance", Other: "RDS instances"},
		Category: discovery_kit_api.Ptr("cloud"),
		Version:  "1.0.0",
		Icon:     discovery_kit_api.Ptr("data:image/svg+xml,%3Csvg%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%3E%3Cpath%20fill-rule%3D%22evenodd%22%20clip-rule%3D%22evenodd%22%20d%3D%22M6.764%2010.212c0%20.296.031.535.087.71.064.176.144.368.256.575.04.064.056.128.056.184%200%20.08-.048.16-.152.24l-.503.335a.383.383%200%2001-.207.072c-.08%200-.16-.04-.24-.112a2.474%202.474%200%2001-.287-.375%206.198%206.198%200%2001-.248-.471c-.622.734-1.405%201.101-2.346%201.101-.671%200-1.206-.191-1.597-.574-.391-.384-.59-.894-.59-1.533%200-.678.239-1.23.726-1.644.487-.415%201.133-.623%201.955-.623.272%200%20.551.024.847.064.295.04.598.104.917.176v-.583c0-.607-.127-1.03-.375-1.277-.255-.248-.686-.367-1.3-.367-.28%200-.567.031-.863.103a6.36%206.36%200%2000-.862.272%202.292%202.292%200%2001-.28.103.49.49%200%2001-.127.024c-.112%200-.168-.08-.168-.247v-.391c0-.128.016-.224.056-.28a.598.598%200%2001.224-.167c.28-.144.614-.264%201.005-.36a4.84%204.84%200%20011.246-.151c.95%200%201.644.215%202.091.647.44.43.663%201.085.663%201.963v2.586h.016zm-3.241%201.214c.263%200%20.535-.048.822-.144a1.78%201.78%200%2000.758-.51c.128-.153.224-.32.272-.512.048-.191.08-.423.08-.694V9.23a6.665%206.665%200%2000-.735-.136%206.014%206.014%200%2000-.75-.048c-.535%200-.926.104-1.19.32-.263.215-.39.518-.39.917%200%20.375.095.655.295.846.191.2.47.296.838.296zm6.41.862c-.144%200-.24-.024-.304-.08-.064-.048-.12-.16-.168-.311l-1.875-6.17a1.398%201.398%200%2001-.072-.32c0-.128.064-.2.191-.2h.783c.151%200%20.255.024.311.08.064.048.112.16.16.312l1.34%205.284%201.246-5.284c.04-.16.088-.264.152-.312a.549.549%200%2001.319-.08h.638c.152%200%20.256.024.32.08.063.048.12.16.151.312l1.261%205.348%201.381-5.348c.048-.16.104-.264.16-.312a.521.521%200%2001.311-.08h.743c.127%200%20.2.064.2.2a.64.64%200%2001-.013.107l-.004.02a1.14%201.14%200%2001-.056.2l-1.923%206.17c-.048.16-.104.264-.168.312a.511.511%200%2001-.303.08h-.687c-.151%200-.255-.024-.319-.08s-.12-.16-.152-.32L12.32%206.749l-1.23%205.14c-.04.16-.087.264-.15.32-.065.056-.176.08-.32.08h-.687zm10.256.215c-.415%200-.83-.048-1.229-.143-.399-.096-.71-.2-.918-.32-.127-.072-.215-.151-.247-.223a.563.563%200%2001-.048-.224v-.407c0-.168.064-.247.183-.247.048%200%20.096.008.144.024.031.01.072.027.119.046l.08.034c.272.12.567.215.879.279.32.064.63.096.95.096.503%200%20.894-.088%201.165-.264a.86.86%200%2000.415-.758.777.777%200%2000-.215-.559c-.144-.151-.415-.287-.807-.415l-1.157-.36c-.583-.183-1.014-.454-1.277-.813a1.902%201.902%200%2001-.4-1.158c0-.335.073-.63.216-.886.144-.255.336-.479.575-.654.24-.184.51-.32.83-.415.32-.096.655-.136%201.006-.136.175%200%20.36.008.535.032.183.024.35.056.519.088.16.04.31.08.454.127.144.048.256.096.336.144a.69.69%200%2001.24.2.43.43%200%2001.071.263v.375c0%20.168-.064.256-.184.256a.83.83%200%2001-.303-.096%203.652%203.652%200%2000-1.532-.311c-.455%200-.815.071-1.062.223-.248.152-.375.383-.375.71%200%20.224.08.416.24.567.159.152.454.304.877.44l1.134.358c.575.184.99.44%201.237.767.247.327.367.702.367%201.117%200%20.344-.072.655-.207.926a2.157%202.157%200%2001-.583.703c-.247.2-.543.343-.886.447-.36.111-.734.167-1.142.167zm1.509%203.88c-2.626%201.94-6.442%202.969-9.722%202.969-4.598%200-8.74-1.7-11.87-4.526-.247-.224-.024-.527.272-.351%203.384%201.963%207.559%203.153%2011.877%203.153%202.914%200%206.114-.607%209.06-1.852.439-.2.814.287.383.607zm-1.98-1.35c.855-.103%202.738-.327%203.074.104.335.423-.376%202.203-.695%202.994-.096.24.112.335.327.151%201.405-1.181%201.773-3.648%201.485-4.007-.287-.351-2.754-.654-4.254.4-.232.167-.192.39.063.358z%22%20fill%3D%22currentColor%22%2F%3E%3C%2Fsvg%3E"),
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

func getRdsAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
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
			},
		},
	}
}

func getRdsDiscoveryResults(w http.ResponseWriter, r *http.Request, body []byte) {
	targets, err := getAllRdsInstances(r.Context())
	if err != nil {
		utils.WriteError(w, "Failed to collect RDS instance information", err)
	} else {
		utils.WriteBody(w, targets)
	}
}

func getAllRdsInstances(context context.Context) ([]discovery_kit_api.Target, error) {
	client := rds.NewFromConfig(awsConfig)

	result := make([]discovery_kit_api.Target, 0, 20)

	var marker *string = nil
	for {
		output, err := client.DescribeDBInstances(context, &rds.DescribeDBInstancesInput{
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
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.arn"] = []string{arn}
	attributes["aws.zone"] = []string{aws.ToString(dbInstance.AvailabilityZone)}
	attributes["aws.rds.engine"] = []string{aws.ToString(dbInstance.Engine)}

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
