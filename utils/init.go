// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	middleware2 "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
	"github.com/rs/zerolog/log"
	extConfig "github.com/steadybit/extension-aws/config"
	"strings"
)

var (
	Accounts *AwsAccounts
)

func InitializeAwsAccountAccess(specification extConfig.Specification) {
	ctx := context.Background()
	awsConfigForRootAccount, err := config.LoadDefaultConfig(ctx, config.WithEndpointResolverWithOptions(overridingEndpointResolver(specification)))
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}

	log.Info().Msgf("Starting in region %s", awsConfigForRootAccount.Region)
	awsConfigForRootAccount.Logger = logForwarder{}
	awsConfigForRootAccount.ClientLogMode = aws.LogRequest
	awsConfigForRootAccount.APIOptions = append(awsConfigForRootAccount.APIOptions, func(stack *middleware.Stack) error {
		return stack.Initialize.Add(customLoggerMiddleware, middleware.After)
	})

	stsClientForRootAccount := sts.NewFromConfig(awsConfigForRootAccount)
	identityOutput, err := stsClientForRootAccount.GetCallerIdentity(ctx, nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}

	Accounts = &AwsAccounts{
		RootAccount: AwsAccount{
			AccountNumber: aws.ToString(identityOutput.Account),
			AwsConfig:     awsConfigForRootAccount,
		},
		Accounts: make(map[string]AwsAccount),
	}

	if len(specification.AssumeRoles) > 0 {
		log.Debug().Msgf("Executing role assumption in other AWS Accounts.")

		for _, roleArn := range specification.AssumeRoles {
			assumedAccount := initializeRoleAssumption(stsClientForRootAccount, roleArn, Accounts.RootAccount)
			if assumedAccount != nil {
				Accounts.Accounts[assumedAccount.AccountNumber] = *assumedAccount
			}
		}
	}
}

func overridingEndpointResolver(specification extConfig.Specification) aws.EndpointResolverWithOptions {
	return aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if specification.AwsEndpointOverride != "" {
			log.Warn().Msgf("Overriding AWS endpoint for service '%s' in region '%s' with '%s'", service, region, specification.AwsEndpointOverride)
			return aws.Endpoint{
				URL: specification.AwsEndpointOverride,
			}, nil
		}

		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
}

type logForwarder struct {
}

func (logger logForwarder) Logf(classification logging.Classification, format string, v ...interface{}) {
	switch classification {
	case logging.Debug:
		log.Trace().Msgf(format, v...)
	case logging.Warn:
		log.Warn().Msgf(format, v...)
	}
}

var customLoggerMiddleware = middleware.InitializeMiddlewareFunc("customLoggerMiddleware",
	func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (out middleware.InitializeOutput, metadata middleware.Metadata, err error) {
		operationName := middleware2.GetOperationName(ctx)
		if strings.HasPrefix(operationName, "List") ||
			strings.HasPrefix(operationName, "Describe") ||
			strings.HasPrefix(operationName, "Get") ||
			strings.HasPrefix(operationName, "Assume") {
			log.Trace().Msgf("AWS-Call: %s - %s", middleware2.GetServiceID(ctx), operationName)
		} else {
			log.Info().Msgf("AWS-Call: %s - %s", middleware2.GetServiceID(ctx), operationName)
		}
		return next.HandleInitialize(ctx, in)
	})

func initializeRoleAssumption(stsServiceForRootAccount *sts.Client, roleArn string, rootAccount AwsAccount) *AwsAccount {
	awsConfig := rootAccount.AwsConfig.Copy()
	awsConfig.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsServiceForRootAccount, roleArn, setSessionName))

	stsClient := sts.NewFromConfig(awsConfig)
	identityOutput, err := stsClient.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to identify AWS account number for account assumed via role '%s'. The roleArn will be ignored until the next restart of the extension.", roleArn)
		return nil
	}

	log.Info().Msgf("Successfully assumed role '%s' in account '%s' (region '%s')", roleArn, aws.ToString(identityOutput.Account), awsConfig.Region)

	return &AwsAccount{
		AccountNumber: aws.ToString(identityOutput.Account),
		AwsConfig:     awsConfig,
	}
}

func setSessionName(o *stscreds.AssumeRoleOptions) {
	o.RoleSessionName = "steadybit-extension-aws"
}
