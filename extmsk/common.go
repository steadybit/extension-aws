// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extmsk

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/steadybit/extension-aws/v2/utils"
)

const (
	mskIcon           = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aAogICAgZD0iTTIzLDE1LjhsLS4zLjJjLTEuMSwxLTMuMSwxLTQuMiwwbC0uNi0uNWMtLjgtLjctMi4zLS43LTMuMSwwbC0uNi41Yy0uNi41LTEuMy44LTIuMS44cy0xLjUtLjMtMi4xLS44bC0uNi0uNWMtLjgtLjctMi4zLS43LTMuMSwwbC0uNi41Yy0xLjEsMS0zLjEsMS00LjIsMGwtLjMtLjIuNi0uNi4zLjJjLjguNywyLjMuNywzLjEsMGwuNi0uNWMxLjEtMSwzLjEtMSw0LjIsMGwuNi41Yy44LjcsMi4zLjcsMy4xLDBsLjYtLjVjMS4xLTEsMy4xLTEsNC4yLDBsLjYuNWMuOC43LDIuMy43LDMuMSwwbC4zLS4yTTExLjIsOC41bDEuMiwxLjhoMWwtMS40LTIsMS4zLTEuOGgtLjlsLTEuMSwxLjd2LTEuN2gtLjh2My44aC44di0xLjhoLS4xWk0yMS44LDIxLjFjLS44LjctMi4zLjctMy4xLDBsLS42LS41Yy0xLjEtMS0zLjEtMS00LjIsMGwtLjYuNWMtLjguNy0yLjMuNy0zLjEsMGwtLjYtLjVjLTEuMS0xLTMuMS0xLTQuMiwwbC0uNi41Yy0uOC43LTIuMy43LTMuMSwwbC0uMy0uMi0uNi42LjMuMmMxLjEsMSwzLjEsMSw0LjIsMGwuNi0uNWMuOC0uNywyLjMtLjcsMy4xLDBsLjYuNWMxLjEsMSwzLjEsMSw0LjIsMGwuNi0uNWMuOC0uNywyLjMtLjcsMy4xLDBsLjYuNWMuNi41LDEuMy44LDIuMS44czEuNS0uMiwyLjEtLjdsLjMtLjItLjYtLjZoME0yMywxNS45aDBaTTQsMTEuNmMwLS45LjgtMS43LDEuNy0xLjdzMCwwLC4yLDBsNC4zLTUuOWMtLjItLjMtLjMtLjYtLjMtLjksMC0uOS44LTEuNywxLjctMS43czEuNy44LDEuNywxLjctLjEuNi0uMy45bDQuMyw1LjloLjJjLjksMCwxLjcuOCwxLjcsMS43cy0uOCwxLjctMS43LDEuNy0xLjQtLjUtMS42LTEuM0g3LjNjLS4yLjctLjgsMS4zLTEuNiwxLjNoMGMtLjksMC0xLjctLjgtMS43LTEuN1pNMTYuNiwxMS42YzAsLjUuNC44LjguOHMuOC0uNC44LS44aDBjMC0uNS0uNC0uOC0uOC0uOHMtLjguNC0uOC44Wk0xMC43LDMuMWMwLC41LjQuOC44LjhzLjgtLjQuOC0uOC0uNC0uOC0uOC0uOC0uOC40LS44LjhaTTYuNywxMC4yYy4zLjIuNS42LjYuOWg4LjZjMC0uNC4zLS43LjYtLjlsLTQuMS01LjZjLS4yLDAtLjUuMi0uOC4ycy0uNiwwLS44LS4yaDBzLTQuMSw1LjYtNC4xLDUuNlpNNC44LDExLjZjMCwuNS40LjguOC44cy44LS40LjgtLjgtLjQtLjgtLjgtLjgtLjguNC0uOC44Wk0yMS44LDE4LjVjLS44LjctMi4zLjctMy4xLDBsLS42LS41Yy0xLjEtMS0zLjEtMS00LjIsMGwtLjYuNWMtLjguNy0yLjMuNy0zLjEsMGwtLjYtLjVjLTEuMS0xLTMuMS0xLTQuMiwwbC0uNi41Yy0uOC43LTIuMy43LTMuMSwwbC0uMy0uMi0uNi42LjMuMmMxLjEsMSwzLjEsMSw0LjIsMGwuNi0uNWMuOC0uNywyLjMtLjcsMy4xLDBsLjYuNWMuNi41LDEuMy44LDIuMS44czEuNS0uMiwyLjEtLjdsLjYtLjVjLjgtLjcsMi4zLS43LDMuMSwwbC42LjVjMS4xLDEsMy4xLDEsNC4yLDBsLjMtLjItLjYtLjZoMCIKICAgIGZpbGw9ImN1cnJlbnRDb2xvciIgLz4KPC9zdmc+"
	mskBrokerTargetId = "com.steadybit.extension_aws.msk.cluster.broker"
)

type KafkaAttackState struct {
	BrokerID         string
	BrokerARN        string
	ClusterARN       string
	ClusterName      string
	Account          string
	Region           string
	DiscoveredByRole *string
}

type MskApi interface {
	RebootBroker(ctx context.Context, params *kafka.RebootBrokerInput, optFns ...func(*kafka.Options)) (*kafka.RebootBrokerOutput, error)
	kafka.ListClustersV2APIClient
	kafka.ListNodesAPIClient
}

func defaultMskClientProvider(account string, region string, role *string) (MskApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region, role)
	if err != nil {
		return nil, err
	}
	return kafka.NewFromConfig(awsAccess.AwsConfig), nil
}
