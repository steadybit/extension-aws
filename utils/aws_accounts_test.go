// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"testing"
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
		rootAccount: AwsAccount{
			AccountNumber: "root",
		},
		accounts: map[string]AwsAccount{
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

	result, err := ForEveryAccount(&accounts, getAccountNumber, reduceAccountNumbers, make([]string, 0, 2), context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"root"}, result)
}

func TestForEachAccountWithRoleAssumption(t *testing.T) {
	accounts := getTestAccountsWithRoleAssumption()

	result, err := ForEveryAccount(&accounts, getAccountNumber, reduceAccountNumbers, make([]string, 0, 2), context.Background())

	require.NoError(t, err)
	require.Equal(t, []string{"assumed1", "assumed2"}, result)
}

func getAccountNumber(account *AwsAccount, _ context.Context) (*string, error) {
	return &account.AccountNumber, nil
}

func reduceAccountNumbers(accountNumbers []string, accountNumber string) ([]string, error) {
	return append(accountNumbers, accountNumber), nil
}

func getTestAccountsWithRoleAssumption() AwsAccounts {
	return AwsAccounts{
		rootAccount: AwsAccount{
			AccountNumber: "root",
		},
		accounts: map[string]AwsAccount{
			"assumed1": {
				AccountNumber: "assumed1",
			},
			"assumed2": {
				AccountNumber: "assumed2",
			},
		},
	}
}

func getTestAccountsWithoutRoleAssumption() AwsAccounts {
	return AwsAccounts{
		rootAccount: AwsAccount{
			AccountNumber: "root",
		},
		accounts: map[string]AwsAccount{},
	}
}
