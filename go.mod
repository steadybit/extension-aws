// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

module github.com/steadybit/extension-aws

go 1.18

require (
	github.com/aws/aws-sdk-go-v2 v1.17.7
	github.com/aws/aws-sdk-go-v2/config v1.18.19
	github.com/aws/aws-sdk-go-v2/credentials v1.13.18
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.91.0
	github.com/aws/aws-sdk-go-v2/service/fis v1.14.6
	github.com/aws/aws-sdk-go-v2/service/rds v1.40.7
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.7
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/rs/zerolog v1.29.0
	github.com/steadybit/action-kit/go/action_kit_api/v2 v2.3.0
	github.com/steadybit/discovery-kit/go/discovery_kit_api v1.2.0
	github.com/steadybit/extension-kit v1.7.2
	github.com/stretchr/testify v1.8.0
	github.com/sosodev/duration v1.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.32 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.6 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.4.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
