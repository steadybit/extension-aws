// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
)

func TestGetAccountSupportsRootAccount(t *testing.T) {
	accounts = getTestAccountsWithoutRoleAssumption()

	account, err := GetAwsAccess("12345678", "us-east-1", nil)

	require.NoError(t, err)
	require.Equal(t, "12345678", account.AccountNumber)
	require.Equal(t, "us-east-1", account.Region)
}

func TestGetAccountSupportsAssumedAccount(t *testing.T) {
	accounts = getTestAccountsWithRoleAssumption()

	account, err := GetAwsAccess("22222222", "eu-central-1", extutil.Ptr("arn:aws:iam::22222222:role/test"))

	require.NoError(t, err)
	require.Equal(t, "22222222", account.AccountNumber)
	require.Equal(t, "eu-central-1", account.Region)
	require.Equal(t, "arn:aws:iam::22222222:role/test", *account.AssumeRole)
}

func TestGetAccountReportsErrorWhenMissing(t *testing.T) {
	accounts = getTestAccountsWithRoleAssumption()

	account, err := GetAwsAccess("unknown-account", "eu-central-1", nil)

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
	require.Equal(t, []string{"12345678@us-east-1"}, values)
}

func TestForEachAccountWithRoleAssumptionAndSingleWorker(t *testing.T) {
	config.Config.WorkerThreads = 1
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0]+"@"+target.Attributes["extension-aws.discovered-by-role"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"11111111@eu-central-1@arn:aws:iam::11111111:role/test", "11111111@us-east-1@arn:aws:iam::11111111:role/test", "22222222@eu-central-1@arn:aws:iam::22222222:role/test", "22222222@us-east-1@arn:aws:iam::22222222:role/test", "33333333@us-east-1@arn:aws:iam::33333333:role/test1", "33333333@us-east-1@arn:aws:iam::33333333:role/test2"}, values)
}

func TestForEachAccountWithRoleAssumptionAndMultipleWorkers(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0]+"@"+target.Attributes["extension-aws.discovered-by-role"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"11111111@eu-central-1@arn:aws:iam::11111111:role/test", "11111111@us-east-1@arn:aws:iam::11111111:role/test", "22222222@eu-central-1@arn:aws:iam::22222222:role/test", "22222222@us-east-1@arn:aws:iam::22222222:role/test", "33333333@us-east-1@arn:aws:iam::33333333:role/test1", "33333333@us-east-1@arn:aws:iam::33333333:role/test2"}, values)
}

func TestForEachAccountWithRoleAssumptionAndError(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(extutil.Ptr("22222222"), nil), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0]+"@"+target.Attributes["extension-aws.discovered-by-role"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"11111111@eu-central-1@arn:aws:iam::11111111:role/test", "11111111@us-east-1@arn:aws:iam::11111111:role/test", "33333333@us-east-1@arn:aws:iam::33333333:role/test1", "33333333@us-east-1@arn:aws:iam::33333333:role/test2"}, values)
}

func TestForEachAccountWithRoleAssumptionAndEmptyLists(t *testing.T) {
	config.Config.WorkerThreads = 4
	accounts = getTestAccountsWithRoleAssumption()

	result, err := ForEveryConfiguredAwsAccess(getTestFunction(nil, extutil.Ptr("22222222")), context.Background(), "discovery")

	require.NoError(t, err)
	// for stable test execution
	var values []string
	for _, target := range result {
		values = append(values, target.Attributes["aws.account"][0]+"@"+target.Attributes["aws.region"][0]+"@"+target.Attributes["extension-aws.discovered-by-role"][0])
	}
	sort.Strings(values)
	require.Equal(t, []string{"11111111@eu-central-1@arn:aws:iam::11111111:role/test", "11111111@us-east-1@arn:aws:iam::11111111:role/test", "33333333@us-east-1@arn:aws:iam::33333333:role/test1", "33333333@us-east-1@arn:aws:iam::33333333:role/test2"}, values)
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
				"aws.account":                      {account.AccountNumber},
				"aws.region":                       {account.Region},
				"extension-aws.discovered-by-role": {aws.ToString(account.AssumeRole)},
			},
		})
		time.Sleep(100 * time.Millisecond)
		return targets, nil
	}
}

func getTestAccountsWithRoleAssumption() map[string]AwsAccess {
	return map[string]AwsAccess{
		//2 accounts with 2 regions each
		"11111111-us-east-1-arn:aws:iam::11111111:role/test": {
			AccountNumber: "11111111",
			Region:        "us-east-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::11111111:role/test"),
		},
		"11111111-eu-central-1-arn:aws:iam::11111111:role/test": {
			AccountNumber: "11111111",
			Region:        "eu-central-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::11111111:role/test"),
		},
		"22222222-us-east-1-arn:aws:iam::22222222:role/test": {
			AccountNumber: "22222222",
			Region:        "us-east-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::22222222:role/test"),
		},
		"22222222-eu-central-1-arn:aws:iam::22222222:role/test": {
			AccountNumber: "22222222",
			Region:        "eu-central-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::22222222:role/test"),
		},
		// 1 account with 1 region but 2 roles separated by tag filer
		"33333333-us-east-1-arn:aws:iam::33333333:role/test": {
			AccountNumber: "33333333",
			Region:        "us-east-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::33333333:role/test1"),
			TagFilters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
			},
		},
		"33333333-eu-central-1-arn:aws:iam::33333333:role/test": {
			AccountNumber: "33333333",
			Region:        "us-east-1",
			AssumeRole:    extutil.Ptr("arn:aws:iam::33333333:role/test2"),
			TagFilters: []config.TagFilter{
				{Key: "env", Values: []string{"dev"}},
			},
		},
	}
}

func getTestAccountsWithoutRoleAssumption() map[string]AwsAccess {
	return map[string]AwsAccess{
		"12345678-us-east-1": {
			AccountNumber: "12345678",
			Region:        "us-east-1",
		},
	}
}
