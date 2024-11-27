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

type AwsAccess struct {
	AccountNumber string
	Region        string
	AwsConfig     aws.Config
}

type Regions map[string]AwsAccess

type AwsAccounts struct {
	RootAccountNumber string
	Accounts          map[string]Regions
}

type GetAccountApi interface {
	GetAccount(accountNumber string) (*AwsAccess, error)
}

func (accounts *AwsAccounts) GetRootAccountNumber() string {
	return accounts.RootAccountNumber
}

func (accounts *AwsAccounts) GetAccount(accountNumber string, region string) (*AwsAccess, error) {
	account, ok := accounts.Accounts[accountNumber]
	if ok {
		if regionAccount, ok := account[region]; ok {
			return &regionAccount, nil
		}
	}
	return nil, fmt.Errorf("AWS Config for account '%s' and region '%s' not found", accountNumber, region)
}

func ForEveryAccount(
	accounts *AwsAccounts,
	supplier func(account *AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error),
	ctx context.Context,
	discovery string,
) ([]discovery_kit_api.Target, error) {
	count := 0
	for _, regions := range accounts.Accounts {
		for range regions {
			count++
		}
	}
	if count > 0 {
		accountsChannel := make(chan AwsAccess, count)
		resultsChannel := make(chan []discovery_kit_api.Target, count)
		for w := 1; w <= config.Config.WorkerThreads; w++ {
			go func(w int, accounts <-chan AwsAccess, result <-chan []discovery_kit_api.Target) {
				for account := range accounts {
					log.Trace().Int("worker", w).Msgf("Collecting %s for account %s in region %s", discovery, account.AccountNumber, account.Region)
					eachResult, eachErr := supplier(&account, ctx)
					if eachErr != nil {
						log.Err(eachErr).Msgf("Failed to collect %s for account %s in region %s", discovery, account.AccountNumber, account.Region)
					}
					resultsChannel <- eachResult
				}
			}(w, accountsChannel, resultsChannel)
		}
		for _, regions := range accounts.Accounts {
			for _, account := range regions {
				accountsChannel <- account
			}
		}
		close(accountsChannel)
		resultTargets := make([]discovery_kit_api.Target, 0)
		for a := 1; a <= count; a++ {
			targets := <-resultsChannel
			if targets != nil {
				resultTargets = append(resultTargets, targets...)
			}
		}
		return resultTargets, nil
	}
	return []discovery_kit_api.Target{}, nil
}
