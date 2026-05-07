// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extmq

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	mqIcon         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djJINFY0em0wIDdoMTZ2Mkg0di0yem0wIDdoMTZ2Mkg0di0yek02IDhoMnYySDZWOHptNCAwaDJ2MmgtMlY4em00IDBoMnYyaC0yVjh6bS04IDdoMnYyaC0ydi0yem00IDBoMnYyaC0ydi0yem00IDBoMnYyaC0ydi0yeiIgZmlsbD0iY3VycmVudENvbG9yIi8+PC9zdmc+"
	brokerTargetId = "com.steadybit.extension_aws.mq.broker"
)

type BrokerAttackState struct {
	BrokerID         string
	BrokerName       string
	Account          string
	Region           string
	DiscoveredByRole *string
}

type MqApi interface {
	mq.ListBrokersAPIClient
	DescribeBroker(ctx context.Context, params *mq.DescribeBrokerInput, optFns ...func(*mq.Options)) (*mq.DescribeBrokerOutput, error)
	RebootBroker(ctx context.Context, params *mq.RebootBrokerInput, optFns ...func(*mq.Options)) (*mq.RebootBrokerOutput, error)
}

func defaultMqClientProvider(account string, region string, role *string) (MqApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return mq.NewFromConfig(awsAccess.AwsConfig), nil
}
