// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
)

type AwsAccount struct {
	AccountNumber string
	AwsConfig     aws.Config
}

type AwsAccounts struct {
	rootAccount AwsAccount

	// accounts is a map of AWS account numbers to AwsAccount for which roles are to be assumed.
	accounts map[string]AwsAccount
}

type GetAccountApi interface {
	GetAccount(accountNumber string) (*AwsAccount, error)
}

func (accounts *AwsAccounts) GetAccount(accountNumber string) (*AwsAccount, error) {
	account, ok := accounts.accounts[accountNumber]
	if ok {
		return &account, nil
	}

	if accountNumber == accounts.rootAccount.AccountNumber {
		return &accounts.rootAccount, nil
	}

	return nil, fmt.Errorf("AWS account '%s' not found", accountNumber)
}

// ForEveryAccount cannot be turned into an interface method because of generics restrictions.
func ForEveryAccount[EachResult any, MergedResult any](
	accounts *AwsAccounts,
	mapper func(account *AwsAccount, ctx context.Context) (*EachResult, error),
	reducer func(merged MergedResult, eachResult EachResult) (MergedResult, error),
	startValue MergedResult,
	ctx context.Context,
) (MergedResult, error) {
	var err error
	result := startValue

	execute := func(account AwsAccount) error {
		eachResult, eachErr := mapper(&account, ctx)
		if eachErr != nil {
			return eachErr
		}

		if eachResult != nil {
			reduceResult, reduceErr := reducer(result, *eachResult)
			if reduceErr != nil {
				return reduceErr
			} else {
				result = reduceResult
			}
		}

		return nil
	}

	if len(accounts.accounts) > 0 {
		for _, account := range accounts.accounts {
			eachErr := execute(account)
			if eachErr != nil {
				err = eachErr
			}
		}
	} else {
		eachErr := execute(accounts.rootAccount)
		if eachErr != nil {
			err = eachErr
		}
	}

	return result, err
}
