// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extConfig "github.com/steadybit/extension-aws/config"
)

type AwsAccess struct {
	AccountNumber string
	Region        string
	AwsConfig     aws.Config
}

type Regions map[string]AwsAccess

var (
	rootAccountNumber string
	accounts          map[string]Regions
)

func InitializeAwsAccess(specification extConfig.Specification) {
	ctx := context.Background()
	awsConfigForRootAccount, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}

	log.Info().Msgf("Starting in region %s", awsConfigForRootAccount.Region)
	awsConfigForRootAccount.Logger = logForwarder{}
	awsConfigForRootAccount.ClientLogMode = aws.LogRequest
	awsConfigForRootAccount.APIOptions = append(awsConfigForRootAccount.APIOptions, func(stack *middleware.Stack) error {
		return stack.Initialize.Add(customLoggerMiddleware, middleware.After)
	})

	if specification.AwsEndpointOverride != "" {
		log.Warn().Msgf("Overriding AWS base endpoint with '%s'", specification.AwsEndpointOverride)
		awsConfigForRootAccount.BaseEndpoint = &specification.AwsEndpointOverride
	}

	stsClientForRootAccount := sts.NewFromConfig(awsConfigForRootAccount)
	identityOutputRoot, err := stsClientForRootAccount.GetCallerIdentity(ctx, nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}

	rootAccountNumber = aws.ToString(identityOutputRoot.Account)
	accounts = make(map[string]Regions)

	regions := []string{awsConfigForRootAccount.Region}
	if len(specification.Regions) > 0 {
		regions = specification.Regions
	}

	if len(specification.AssumeRoles) > 0 {
		log.Debug().Msgf("Executing role assumption in other AWS Accounts.")
		for _, roleArn := range specification.AssumeRoles {
			awsConfig := awsConfigForRootAccount.Copy()
			awsConfig.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClientForRootAccount, roleArn, func(o *stscreds.AssumeRoleOptions) {
				o.RoleSessionName = "steadybit-extension-aws"
			}))

			stsClient := sts.NewFromConfig(awsConfig)
			identityOutput, err := stsClient.GetCallerIdentity(context.Background(), nil)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to identify AWS account number for account assumed via role '%s'. The roleArn will be ignored until the next restart of the extension.", roleArn)
				continue
			}
			assumedAccount := aws.ToString(identityOutput.Account)
			log.Info().Msgf("Successfully assumed role '%s' in account '%s'", roleArn, assumedAccount)
			prepareRegionConfigs(regions, awsConfig, assumedAccount)
		}
	} else {
		prepareRegionConfigs(regions, awsConfigForRootAccount, aws.ToString(identityOutputRoot.Account))
	}
}

func prepareRegionConfigs(regions []string, awsConfig aws.Config, account string) {
	if _, ok := accounts[account]; !ok {
		accounts[account] = make(map[string]AwsAccess)
	}
	for _, region := range regions {
		regionalConfig := awsConfig.Copy()
		regionalConfig.Region = region
		accounts[account][region] = AwsAccess{
			AccountNumber: account,
			AwsConfig:     regionalConfig,
			Region:        region,
		}
	}
}

func GetRootAccountNumber() string {
	return rootAccountNumber
}

func GetAwsAccess(accountNumber string, region string) (*AwsAccess, error) {
	account, ok := accounts[accountNumber]
	if ok {
		if regionAccount, ok := account[region]; ok {
			return &regionAccount, nil
		}
	}
	return nil, fmt.Errorf("AWS Config for account '%s' and region '%s' not found", accountNumber, region)
}

func ForEveryConfiguredAwsAccess(supplier func(account *AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error), ctx context.Context, discovery string) ([]discovery_kit_api.Target, error) {
	count := 0
	for _, regions := range accounts {
		for range regions {
			count++
		}
	}
	if count > 0 {
		accountsChannel := make(chan AwsAccess, count)
		resultsChannel := make(chan []discovery_kit_api.Target, count)
		for w := 1; w <= extConfig.Config.WorkerThreads; w++ {
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
		for _, regions := range accounts {
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
