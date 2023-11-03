// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/config"
)

type AwsAccount struct {
	AccountNumber string
	AwsConfig     aws.Config
}

type AwsAccounts struct {
	RootAccount AwsAccount

	// accounts is a map of AWS account numbers to AwsAccount for which roles are to be assumed.
	Accounts map[string]AwsAccount
}

type GetAccountApi interface {
	GetAccount(accountNumber string) (*AwsAccount, error)
}

func (accounts *AwsAccounts) GetRootAccount() *AwsAccount {
	return &accounts.RootAccount
}

func (accounts *AwsAccounts) GetAccount(accountNumber string) (*AwsAccount, error) {
	account, ok := accounts.Accounts[accountNumber]
	if ok {
		return &account, nil
	}

	if accountNumber == accounts.RootAccount.AccountNumber {
		return &accounts.RootAccount, nil
	}

	return nil, fmt.Errorf("AWS account '%s' not found", accountNumber)
}

func ForEveryAccount(
	accounts *AwsAccounts,
	supplier func(account *AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error),
	ctx context.Context,
	discovery string,
) ([]discovery_kit_api.Target, error) {
	numAccounts := len(accounts.Accounts)
	if numAccounts > 0 {
		accountsChannel := make(chan AwsAccount, numAccounts)
		resultsChannel := make(chan []discovery_kit_api.Target, numAccounts)
		for w := 1; w <= config.Config.WorkerThreads; w++ {
			go func(w int, accounts <-chan AwsAccount, result <-chan []discovery_kit_api.Target) {
				for account := range accounts {
					log.Trace().Int("worker", w).Msgf("Collecting %s for account %s", discovery, account.AccountNumber)
					eachResult, eachErr := supplier(&account, ctx)
					if eachErr != nil {
						log.Err(eachErr).Msgf("Failed to collect %s for account %s", discovery, account.AccountNumber)
					}
					resultsChannel <- eachResult
				}
			}(w, accountsChannel, resultsChannel)
		}
		for _, account := range accounts.Accounts {
			accountsChannel <- account
		}
		close(accountsChannel)
		var resultTargets []discovery_kit_api.Target
		for a := 1; a <= numAccounts; a++ {
			targets := <-resultsChannel
			if targets != nil {
				resultTargets = append(resultTargets, targets...)
			}
		}
		return resultTargets, nil
	} else {
		return supplier(&accounts.RootAccount, ctx)
	}
}
