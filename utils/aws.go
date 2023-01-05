// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog/log"
)

var (
	AwsAccountNumber string
	AwsConfig        aws.Config
)

func InitializeAwsAccountAccess() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to load AWS configuration")
	}
	AwsConfig = cfg

	stsService := sts.NewFromConfig(cfg)
	identityOutput, err := stsService.GetCallerIdentity(context.Background(), nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to identify AWS account number")
	}
	AwsAccountNumber = aws.ToString(identityOutput.Account)
}
