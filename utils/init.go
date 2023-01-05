// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog/log"
	extConfig "github.com/steadybit/extension-aws/config"
)

var (
	Accounts *AwsAccounts
)

func InitializeAwsAccountAccess(specification extConfig.Specification) {
	awsConfigForRootAccount, err := config.LoadDefaultConfig(context.Background())

	log.Info().Msgf("Starting in region %s", awsConfigForRootAccount.Region)

	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}

	stsClientForRootAccount := sts.NewFromConfig(awsConfigForRootAccount)
	identityOutput, err := stsClientForRootAccount.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}

	Accounts = &AwsAccounts{
		rootAccount: AwsAccount{
			AccountNumber: aws.ToString(identityOutput.Account),
			AwsConfig:     awsConfigForRootAccount,
		},
		accounts: make(map[string]AwsAccount),
	}

	if len(specification.AssumeRoles) > 0 {
		log.Debug().Msgf("Executing role assumption in other AWS Accounts.")

		for _, roleArn := range specification.AssumeRoles {
			assumedAccount := initializeRoleAssumption(stsClientForRootAccount, roleArn, Accounts.rootAccount)
			Accounts.accounts[assumedAccount.AccountNumber] = assumedAccount
		}
	}
}

func initializeRoleAssumption(stsServiceForRootAccount *sts.Client, roleArn string, rootAccount AwsAccount) AwsAccount {
	awsConfig := rootAccount.AwsConfig.Copy()
	awsConfig.Credentials = stscreds.NewAssumeRoleProvider(stsServiceForRootAccount, roleArn, setSessionName)

	stsClient := sts.NewFromConfig(awsConfig)
	identityOutput, err := stsClient.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number for account assumed via role '%s'", roleArn)
	}

	log.Info().Msgf("Successfully assumed role '%s' in account '%s' (region '%s')", roleArn, aws.ToString(identityOutput.Account), awsConfig.Region)

	return AwsAccount{
		AccountNumber: aws.ToString(identityOutput.Account),
		AwsConfig:     awsConfig,
	}
}

func setSessionName(o *stscreds.AssumeRoleOptions) {
	o.RoleSessionName = "steadybit-extension-aws"
}
