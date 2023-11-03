// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
)

type commonAttributes struct {
}

func NewCommonAttributeDescriber() discovery_kit_sdk.AttributeDescriber {
	return &commonAttributes{}
}

func (c *commonAttributes) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
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
		}, {
			Attribute: "aws.role",
			Label: discovery_kit_api.PluralLabel{
				One:   "AWS Role",
				Other: "AWS Roles",
			},
		},
	}
}
