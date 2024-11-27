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

	Accounts = &AwsAccounts{
		RootAccountNumber: aws.ToString(identityOutputRoot.Account),
		Accounts:          make(map[string]Regions),
	}

	regions := []string{awsConfigForRootAccount.Region}
	if len(specification.Regions) > 0 {
		regions = specification.Regions
	}

	if len(specification.AssumeRoles) > 0 {
		log.Debug().Msgf("Executing role assumption in other AWS Accounts.")
		for _, roleArn := range specification.AssumeRoles {
			awsConfig := awsConfigForRootAccount.Copy()
			awsConfig.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClientForRootAccount, roleArn, setSessionName))

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
	if _, ok := Accounts.Accounts[account]; !ok {
		Accounts.Accounts[account] = make(map[string]AwsAccess)
	}
	for _, region := range regions {
		regionalConfig := awsConfig.Copy()
		regionalConfig.Region = region
		Accounts.Accounts[account][region] = AwsAccess{
			AccountNumber: account,
			AwsConfig:     regionalConfig,
			Region:        region,
		}
	}
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
			//strings.HasPrefix(operationName, "Assume") ||
			strings.HasPrefix(operationName, "Get") {
			log.Trace().Msgf("AWS-Call: %s - %s - %s", middleware2.GetRegion(ctx), middleware2.GetServiceID(ctx), operationName)
		} else {
			log.Info().Msgf("AWS-Call: %s - %s - %s", middleware2.GetRegion(ctx), middleware2.GetServiceID(ctx), operationName)
		}
		return next.HandleInitialize(ctx, in)
	})

func setSessionName(o *stscreds.AssumeRoleOptions) {
	o.RoleSessionName = "steadybit-extension-aws"
}
