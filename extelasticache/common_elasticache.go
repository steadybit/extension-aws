// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package extelasticache

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/steadybit/extension-aws/utils"
)

const (
	elasticacheNodeGroupTargetId = "com.steadybit.extension_aws.elasticache.node-group"
)

const (
	elasticacheIcon = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aCBmaWxsPSJjdXJyZW50Q29sb3IiCiAgICBkPSJNMTYuMywyMC4zdi0yLjNjLTEsLjYtMi44LjktNC41LjlzLTMuMy0uMy00LjEtLjl2Mi4yYzAsLjcsMS42LDEuNCw0LjEsMS40czQuNS0uNyw0LjUtMS40aDBaTTExLjgsMTUuNGMtMS45LDAtMy4zLS4zLTQuMS0uOXYyLjJjMCwuNywxLjYsMS4zLDQuMSwxLjNzNC41LS43LDQuNS0xLjN2LTIuM2MtMSwuNi0yLjguOS00LjUuOWgwWk0xNi4zLDEzLjN2LTIuN2MtMSwuNi0yLjguOS00LjUuOXMtMy4zLS4zLTQuMS0uOXYyLjZjMCwuNywxLjYsMS4zLDQuMSwxLjNzNC41LS43LDQuNS0xLjNoMFpNNy43LDkuNHMwLDAsMCwwaDBjMCwuNywxLjYsMS40LDQuMSwxLjRzNC41LS44LDQuNS0xLjNoMHMwLDAsMCwwYzAsMCwwLDAsMCwwLDAtLjYtMS42LTEuNC00LjUtMS40cy00LjEuNy00LjEsMS40aDBaTTE3LjEsOS41djMuOGgwczAsMCwwLDB2My41aDBzMCwwLDAsMHYzLjVjMCwxLjUtMi43LDIuMS01LjMsMi4xcy00LjktLjgtNC45LTIuMXYtMy41czAsMCwwLDBoMHYtMy41czAsMCwwLDBoMHYtMy44czAsMCwwLDBjMC0xLjMsMS45LTIuMSw0LjktMi4xczUuMy43LDUuMywyLjEsMCwwLDAsMGgwWk0yMi42LDQuNGMuMiwwLC40LS4yLjQtLjRWMmMwLS4yLS4yLS40LS40LS40SDEuNGMtLjIsMC0uNC4yLS40LjR2MmMwLC4yLjIuNC40LjQuNSwwLC45LjQuOS45cy0uNC45LS45LjktLjQuMi0uNC40djhjMCwuMi4yLjQuNC40aDMuOXYtLjhoLTJ2LTEuMmgydi0uOGgtMi40Yy0uMiwwLS40LjItLjQuNHYxLjZoLS44di03LjNjLjctLjIsMS4zLS44LDEuMy0xLjZzLS41LTEuNC0xLjMtMS42di0xLjNoMjAuNHYxLjNjLS43LjItMS4zLjgtMS4zLDEuNnMuNSwxLjQsMS4zLDEuNnY3LjNoLS44di0xLjZjMC0uMi0uMi0uNC0uNC0uNGgtMi40di44aDJ2MS4yaC0ydi44aDMuOWMuMiwwLC40LS4yLjQtLjRWNi41YzAtLjItLjItLjQtLjQtLjQtLjUsMC0uOS0uNC0uOS0uOXMuNC0uOS45LS45aDBaTTcuMyw3LjF2LTMuMWMwLS4yLS4yLS40LS40LS40aC0yLjRjLS4yLDAtLjQuMi0uNC40djYuN2MwLC4yLjIuNC40LjRoMS4ydi0uOGgtLjh2LTUuOWgxLjZ2Mi44aC44Wk0xOS4xLDEwLjJoLS40di44aC44Yy4yLDAsLjQtLjIuNC0uNFYzLjljMC0uMi0uMi0uNC0uNC0uNGgtMi40Yy0uMiwwLS40LjItLjQuNHYzLjFoLjh2LTIuOGgxLjZ2NS45Wk0xNS45LDYuN3YtMi44YzAtLjItLjItLjQtLjQtLjRoLTIuN2MtLjIsMC0uNC4yLS40LjR2Mi40aC44di0yaDJ2Mi40aC44Wk0xMC44LDYuM3YtMmgtMnYyLjRoLS44di0yLjhjMC0uMi4yLS40LjQtLjRoMi44Yy4yLDAsLjQuMi40LjR2Mi40aC0uOFoiIC8+Cjwvc3ZnPg=="
)

type ElasticacheClusterAttackState struct {
	ReplicationGroupID string
	NodeGroupID        string
	Account            string
	Region             string
}

type ElasticacheApi interface {
	TestFailover(ctx context.Context, params *elasticache.TestFailoverInput, optFns ...func(*elasticache.Options)) (*elasticache.TestFailoverOutput, error)
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

func defaultElasticacheClientProvider(account string, region string) (ElasticacheApi, error) {
	awsAccess, err := utils.GetAwsAccess(account, region)
	if err != nil {
		return nil, err
	}
	return elasticache.NewFromConfig(awsAccess.AwsConfig), nil
}
