// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-kit/exthttp"
)

var (
	AwsAccountNumber string
	AwsConfig        aws.Config
)

func InitializeAwsAccountAccess() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}
	AwsConfig = cfg

	stsService := sts.NewFromConfig(cfg)
	identityOutput, err := stsService.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}
	AwsAccountNumber = aws.ToString(identityOutput.Account)
}

func RegisterCommonDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/common/discovery/attribute-descriptions", exthttp.GetterAsHandler(getCommonAttributeDescriptions))
}

func getCommonAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
			{
				Attribute: "aws.account",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS account",
					Other: "AWS accounts",
				},
			}, {
				Attribute: "aws.region",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS region",
					Other: "AWS regions",
				},
			}, {
				Attribute: "aws.zone",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS zone",
					Other: "AWS zones",
				},
			}, {
				Attribute: "aws.zone.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS zone ID",
					Other: "AWS zone IDs",
				},
			}, {
				Attribute: "aws.arn",
				Label: discovery_kit_api.PluralLabel{
					One:   "AWS ARN",
					Other: "AWS ARNs",
				},
			},
		},
	}
}
