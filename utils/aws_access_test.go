// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"errors"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
)

func TestGetAccountSupportsRootAccount(t *testing.T) {
	accounts = getTestAccountsWithoutRoleAssumption()

	account, err := GetAwsAccess("root", "us-east-1")

	require.NoError(t, err)
	require.Equal(t, "root", account.AccountNumber)
	require.Equal(t, "us-east-1", account.Region)
}

func TestGetAccountSupportsAssumedAccount(t *testing.T) {
	accounts = getTestAccountsWithRoleAssumption()

	account, err := GetAwsAccess("assumed2", "eu-central-1")

	require.NoError(t, err)
	require.Equal(t, "assumed2", account.AccountNumber)
	require.Equal(t, "eu-central-1", account.Region)
}

func TestGetAccountReportsErrorWhenMissing(t *testing.T) {
	accounts = getTestAccountsWithRoleAssumption()

	account, err := GetAwsAccess("unknown-account", "eu-central-1")

	require.ErrorContains(t, err, "unknown-account")
	require.Nil(t, account)
}

func TestForEachAccountWithoutRoleAssumption(t *testing.T) {
	config.Config.WorkerThreads = 1
	accounts = getTestAccountsWithoutRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, nil), context.Background(), "discovery")

	require.NoError(t, err)
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0])
	}
	require.Equal(t, []string{"root@us-east-1"}, values)
}

func TestForEachAccountWithRoleAssumptionAndSingleWorker(t *testing.T) {
	config.Config.WorkerThreads = 1
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1@eu-central-1", "assumed1@us-east-1", "assumed2@eu-central-1", "assumed2@us-east-1", "assumed3@eu-central-1", "assumed3@us-east-1", "assumed4@eu-central-1", "assumed4@us-east-1"}, values)
}

func TestForEachAccountWithRoleAssumptionAndMultipleWorkers(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1@eu-central-1", "assumed1@us-east-1", "assumed2@eu-central-1", "assumed2@us-east-1", "assumed3@eu-central-1", "assumed3@us-east-1", "assumed4@eu-central-1", "assumed4@us-east-1"}, values)
}

func TestForEachAccountWithRoleAssumptionAndError(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(extutil.Ptr("assumed2"), nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1@eu-central-1", "assumed1@us-east-1", "assumed3@eu-central-1", "assumed3@us-east-1", "assumed4@eu-central-1", "assumed4@us-east-1"}, values)
}

func TestForEachAccountWithRoleAssumptionAndEmptyLists(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, extutil.Ptr("assumed2")), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1@eu-central-1", "assumed1@us-east-1", "assumed3@eu-central-1", "assumed3@us-east-1", "assumed4@eu-central-1", "assumed4@us-east-1"}, values)
}

func getTestFunction(errorForAccount *string, emptyForAccount *string) func(account *AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	return func(account *AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
		if (errorForAccount != nil) && (*errorForAccount == account.AccountNumber) {
			return nil, errors.New("damn broken discovery")
		}
		if (emptyForAccount != nil) && (*emptyForAccount == account.AccountNumber) {
			return []discovery_kit_api.Target{}, nil
		}
		var targets []discovery_kit_api.Target
		targets = append(targets, discovery_kit_api.Target{
			TargetType: "example",
			Label:      "label",
			Attributes: map[string][]string{
				"aws.account": {account.AccountNumber},
				"aws.region":  {account.Region},
			},
		})
		time.Sleep(100 * time.Millisecond)
		return targets, nil
	}
}

func getTestAccountsWithRoleAssumption() map[string]Regions {
	return map[string]Regions{
		"assumed1": {
			"us-east-1": {
				AccountNumber: "assumed1",
				Region:        "us-east-1",
			},
			"eu-central-1": {
				AccountNumber: "assumed1",
				Region:        "eu-central-1",
			},
		},
		"assumed2": {
			"us-east-1": {
				AccountNumber: "assumed2",
				Region:        "us-east-1",
			},
			"eu-central-1": {
				AccountNumber: "assumed2",
				Region:        "eu-central-1",
			},
		},
		"assumed3": {
			"us-east-1": {
				AccountNumber: "assumed3",
				Region:        "us-east-1",
			},
			"eu-central-1": {
				AccountNumber: "assumed3",
				Region:        "eu-central-1",
			},
		},
		"assumed4": {
			"us-east-1": {
				AccountNumber: "assumed4",
				Region:        "us-east-1",
			},
			"eu-central-1": {
				AccountNumber: "assumed4",
				Region:        "eu-central-1",
			},
		},
	}
}

func getTestAccountsWithoutRoleAssumption() map[string]Regions {
	return map[string]Regions{
		"root": {
			"us-east-1": {
				AccountNumber: "root",
				Region:        "us-east-1",
			},
		},
	}
}
