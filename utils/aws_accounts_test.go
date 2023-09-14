// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
)

func TestGetAccountSupportsRootAccount(t *testing.T) {
	accounts := getTestAccountsWithRoleAssumption()

	account, err := accounts.GetAccount("root")

	require.NoError(t, err)
	require.Equal(t, "root", account.AccountNumber)
}

func TestGetAccountSupportsAssumedAccount(t *testing.T) {
	accounts := getTestAccountsWithRoleAssumption()

	account, err := accounts.GetAccount("assumed2")

	require.NoError(t, err)
	require.Equal(t, "assumed2", account.AccountNumber)
}

func TestMustPreferAssumedAccount(t *testing.T) {
	accounts := AwsAccounts{
		RootAccount: AwsAccount{
			AccountNumber: "root",
		},
		Accounts: map[string]AwsAccount{
			"assumed1": {
				AccountNumber: "assumed1",
			},
			"root": {
				AccountNumber: "root",
				AwsConfig:     aws.Config{},
			},
		},
	}

	account, err := accounts.GetAccount("root")

	require.NoError(t, err)
	require.NotNil(t, account.AwsConfig)
}

func TestGetAccountReportsErrorWhenMissing(t *testing.T) {
	accounts := getTestAccountsWithRoleAssumption()

	account, err := accounts.GetAccount("unknown-account")

	require.ErrorContains(t, err, "unknown-account")
	require.Nil(t, account)
}

func TestForEachAccountWithoutRoleAssumption(t *testing.T) {
	accounts := getTestAccountsWithoutRoleAssumption()

	result, err := ForEveryAccount(&accounts, getTestFunction(nil), context.Background(), "discovery")

	require.NoError(t, err)
	var values []string
	for _, target := range *result {
		values = append(values, target.Attributes["aws.account"][0])
	}
	require.Equal(t, []string{"root"}, values)
}

func TestForEachAccountWithRoleAssumptionAndSingleWorker(t *testing.T) {
	config.Config.WorkerThreads = 1
	accounts := getTestAccountsWithRoleAssumption()

	result, err := ForEveryAccount(&accounts, getTestFunction(nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range *result {
		values = append(values, target.Attributes["aws.account"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1", "assumed2", "assumed3", "assumed4", "assumed5", "assumed6", "assumed7", "assumed8", "assumed9"}, values)
}

func TestForEachAccountWithRoleAssumptionAndMultipleWorkers(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts := getTestAccountsWithRoleAssumption()

	result, err := ForEveryAccount(&accounts, getTestFunction(nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range *result {
		values = append(values, target.Attributes["aws.account"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1", "assumed2", "assumed3", "assumed4", "assumed5", "assumed6", "assumed7", "assumed8", "assumed9"}, values)
}

func TestForEachAccountWithRoleAssumptionAndError(t *testing.T) {
	accounts := getTestAccountsWithRoleAssumption()

	result, err := ForEveryAccount(&accounts, getTestFunction(extutil.Ptr("assumed2")), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range *result {
		values = append(values, target.Attributes["aws.account"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"assumed1", "assumed3", "assumed4", "assumed5", "assumed6", "assumed7", "assumed8", "assumed9"}, values)
}

func getTestFunction(errorForAccount *string) func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	return func(account *AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
		if (errorForAccount != nil) && (*errorForAccount == account.AccountNumber) {
			return nil, errors.New("damn broken discovery")
		}
		var targets []discovery_kit_api.Target
		targets = append(targets, discovery_kit_api.Target{
			TargetType: "example",
			Label:      "label",
			Attributes: map[string][]string{
				"aws.account": {account.AccountNumber},
			},
		})
		time.Sleep(100 * time.Millisecond)
		return &targets, nil
	}
}

func getTestAccountsWithRoleAssumption() AwsAccounts {
	return AwsAccounts{
		RootAccount: AwsAccount{
			AccountNumber: "root",
		},
		Accounts: map[string]AwsAccount{
			"assumed1": {
				AccountNumber: "assumed1",
			},
			"assumed2": {
				AccountNumber: "assumed2",
			},
			"assumed3": {
				AccountNumber: "assumed3",
			},
			"assumed4": {
				AccountNumber: "assumed4",
			},
			"assumed5": {
				AccountNumber: "assumed5",
			},
			"assumed6": {
				AccountNumber: "assumed6",
			},
			"assumed7": {
				AccountNumber: "assumed7",
			},
			"assumed8": {
				AccountNumber: "assumed8",
			},
			"assumed9": {
				AccountNumber: "assumed9",
			},
		},
	}
}

func getTestAccountsWithoutRoleAssumption() AwsAccounts {
	return AwsAccounts{
		RootAccount: AwsAccount{
			AccountNumber: "root",
		},
		Accounts: map[string]AwsAccount{},
	}
}
