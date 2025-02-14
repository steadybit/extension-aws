// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
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
	AssumeRole    *string
	TagFilters    []extConfig.TagFilter
	AwsConfig     aws.Config
}

var (
	rootAccountNumber string
	accounts          map[string]AwsAccess
)

func InitializeAwsAccess(specification extConfig.Specification, awsConfigForRootAccount aws.Config) {
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
	identityOutputRoot, err := stsClientForRootAccount.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}

	rootAccountNumber = aws.ToString(identityOutputRoot.Account)
	accounts = make(map[string]AwsAccess)

	if len(specification.AssumeRolesAdvanced) > 0 {
		log.Debug().Msgf("Executing role assumption in other AWS Accounts.")
		for _, assumeRoleConfig := range specification.AssumeRolesAdvanced {
			awsConfig := awsConfigForRootAccount.Copy()
			awsConfig.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClientForRootAccount, assumeRoleConfig.AssumeRole, func(o *stscreds.AssumeRoleOptions) {
				o.RoleSessionName = "steadybit-extension-aws"
			}))

			stsClient := sts.NewFromConfig(awsConfig)
			identityOutput, err := stsClient.GetCallerIdentity(context.Background(), nil)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to identify AWS account number for account assumed via role '%s'. The roleArn will be ignored until the next restart of the extension.", assumeRoleConfig.AssumeRole)
				continue
			}
			assumedAccount := aws.ToString(identityOutput.Account)
			log.Info().Msgf("Successfully assumed role '%s' in account '%s'", assumeRoleConfig.AssumeRole, assumedAccount)
			prepareRegionConfigs(awsConfig, &assumeRoleConfig.AssumeRole, assumedAccount, assumeRoleConfig.Regions, assumeRoleConfig.TagFilters)
		}
	} else {
		prepareRegionConfigs(awsConfigForRootAccount, nil, aws.ToString(identityOutputRoot.Account), specification.Regions, specification.TagFilters)
	}
}

func prepareRegionConfigs(awsConfig aws.Config, assumedRole *string, account string, regions []string, tagFilters []extConfig.TagFilter) {
	for _, region := range regions {
		regionalConfig := awsConfig.Copy()
		regionalConfig.Region = region
		accounts[getMapKey(account, region, assumedRole)] = AwsAccess{
			AccountNumber: account,
			AwsConfig:     regionalConfig,
			Region:        region,
			AssumeRole:    assumedRole,
			TagFilters:    tagFilters,
		}
	}
}

func getMapKey(account string, region string, assumedRole *string) string {
	if assumedRole == nil {
		return fmt.Sprintf("%s-%s", account, region)
	} else {
		return fmt.Sprintf("%s-%s-%s", account, region, aws.ToString(assumedRole))
	}
}

func GetRootAccountNumber() string {
	return rootAccountNumber
}

func GetAwsAccess(accountNumber string, region string, assumedRole *string) (*AwsAccess, error) {
	mapKey := getMapKey(accountNumber, region, assumedRole)
	account, ok := accounts[mapKey]
	if ok {
		return &account, nil
	}
	if assumedRole != nil {
		return nil, fmt.Errorf("AWS Config for account '%s' and region '%s' and role '%s' not found", accountNumber, region, aws.ToString(assumedRole))
	} else {
		return nil, fmt.Errorf("AWS Config for account '%s' and region '%s' not found", accountNumber, region)
	}
}

func ForEveryConfiguredAwsAccess(supplier func(account *AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error), ctx context.Context, discovery string) ([]discovery_kit_api.Target, error) {
	count := len(accounts)
	if count > 0 {
		accountsChannel := make(chan AwsAccess, count)
		resultsChannel := make(chan []discovery_kit_api.Target, count)
		for w := 1; w <= extConfig.Config.WorkerThreads; w++ {
			go func(w int, accounts <-chan AwsAccess, result <-chan []discovery_kit_api.Target) {
				for account := range accounts {
					log.Trace().Str("role", aws.ToString(account.AssumeRole)).Str("account", account.AccountNumber).Str("region", account.Region).Int("worker", w).Msgf("Collecting %s", discovery)
					eachResult, eachErr := supplier(&account, ctx)
					if eachErr != nil {
						log.Err(eachErr).Str("role", aws.ToString(account.AssumeRole)).Str("account", account.AccountNumber).Str("region", account.Region).Msgf("Failed to collect %s", discovery)
					}
					resultsChannel <- eachResult
				}
			}(w, accountsChannel, resultsChannel)
		}
		for _, account := range accounts {
			accountsChannel <- account
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
