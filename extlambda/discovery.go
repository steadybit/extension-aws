/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extlambda

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	tagTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/extec2"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strconv"
	"strings"
	"time"
)

type lambdaDiscovery struct{}

var (
	_                                  discovery_kit_sdk.TargetDescriber    = (*lambdaDiscovery)(nil)
	_                                  discovery_kit_sdk.AttributeDescriber = (*lambdaDiscovery)(nil)
	missingPermissionTagsAlreadyLogged                                      = false
)

func NewLambdaDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &lambdaDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalLambda)*time.Second),
	)
}

func (l *lambdaDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: lambdaTargetID,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalLambda)),
		},
	}
}

func (*lambdaDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:      lambdaTargetID,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(lambdaTargetIcon),

		Label: discovery_kit_api.PluralLabel{One: "AWS Lambda", Other: "AWS Lambdas"},

		// Category for the targets to appear in
		Category: extutil.Ptr("cloud"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws.lambda.function-name"},
				{Attribute: "aws.account"},
				{Attribute: "aws.region"},
				{Attribute: "aws.lambda.description"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws.lambda.function-name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (*lambdaDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.lambda.function-name",
			Label: discovery_kit_api.PluralLabel{
				One:   "Function Name",
				Other: "Function Names",
			},
		},
		{
			Attribute: "aws.lambda.runtime",
			Label: discovery_kit_api.PluralLabel{
				One:   "Runtime",
				Other: "Runtimes",
			},
		},
		{
			Attribute: "aws.lambda.handler",
			Label: discovery_kit_api.PluralLabel{
				One:   "Handler",
				Other: "Handlers",
			},
		},
		{
			Attribute: "aws.lambda.codeSize",
			Label: discovery_kit_api.PluralLabel{
				One:   "Code Size",
				Other: "Code Sizes",
			},
		}, {
			Attribute: "aws.lambda.description",
			Label: discovery_kit_api.PluralLabel{
				One:   "Description",
				Other: "Descriptions",
			},
		}, {
			Attribute: "aws.lambda.timeout",
			Label: discovery_kit_api.PluralLabel{
				One:   "Timeout",
				Other: "Timeouts",
			},
		}, {
			Attribute: "aws.lambda.memory-size",
			Label: discovery_kit_api.PluralLabel{
				One:   "Memory Size",
				Other: "Memory Sizes",
			},
		}, {
			Attribute: "aws.lambda.last-modified",
			Label: discovery_kit_api.PluralLabel{
				One:   "Last Modified",
				Other: "Last Modified",
			},
		}, {
			Attribute: "aws.lambda.version",
			Label: discovery_kit_api.PluralLabel{
				One:   "Version",
				Other: "Versions",
			},
		},
		{
			Attribute: "aws.lambda.revision-id",
			Label: discovery_kit_api.PluralLabel{
				One:   "Revision ID",
				Other: "Revision IDs",
			},
		},
		{
			Attribute: "aws.lambda.package-type",
			Label: discovery_kit_api.PluralLabel{
				One:   "Package Type",
				Other: "Package Types",
			},
		}, {
			Attribute: "aws.lambda.architecture",
			Label: discovery_kit_api.PluralLabel{
				One:   "Architecture",
				Other: "Architectures",
			},
		}, {
			Attribute: "aws.lambda.failure-injection-param",
			Label: discovery_kit_api.PluralLabel{
				One:   "Failure Injection SSM Parameter",
				Other: "Failure Injection SSM Parameters",
			},
		}, {
			Attribute: "aws.lambda.master-arn",
			Label: discovery_kit_api.PluralLabel{
				One:   "ARN of the main function.",
				Other: "ARN of the main functions.",
			},
		},
	}
}

func (l *lambdaDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getTargetsForAccount, ctx, "lambda")
}

func getTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	lambdaClient := lambda.NewFromConfig(account.AwsConfig)
	tagsClient := resourcegroupstaggingapi.NewFromConfig(account.AwsConfig)

	result, err := getAllAwsLambdaFunctions(ctx, lambdaClient, tagsClient, extec2.Util, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover lambda functions for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_LAMBDA=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllAwsLambdaFunctions(ctx context.Context, lambdaClient lambda.ListFunctionsAPIClient, tagsClient resourcegroupstaggingapi.GetResourcesAPIClient, ec2Util extec2.GetVpcNameUtil, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 100)
	var marker *string = nil
	for {
		output, err := lambdaClient.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}

		if len(output.Functions) == 0 {
			return result, nil
		}

		tagsResponses, err := getTags(ctx, output, tagsClient, len(account.TagFilters) > 0)
		if err != nil {
			return nil, err
		}

		for _, function := range output.Functions {
			var tags []tagTypes.Tag
			for _, tagsResponse := range tagsResponses {
				if *function.FunctionArn == *tagsResponse.ResourceARN {
					tags = tagsResponse.Tags
				}
			}
			if utils.MatchesTagFilter(tags, account.TagFilters) {
				result = append(result, toTarget(function, ec2Util, account.AccountNumber, account.Region, account.AssumeRole, tags))
			}
		}

		if output.NextMarker == nil {
			break
		} else {
			marker = output.NextMarker
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesLambda), nil
}

func getTags(ctx context.Context, output *lambda.ListFunctionsOutput, tagsClient resourcegroupstaggingapi.GetResourcesAPIClient, tagsRequired bool) ([]tagTypes.ResourceTagMapping, error) {
	arns := make([]string, 0, len(output.Functions))
	for _, function := range output.Functions {
		arns = append(arns, *function.FunctionArn)
	}
	tagsResult, tagsErr := tagsClient.GetResources(ctx, &resourcegroupstaggingapi.GetResourcesInput{
		ResourceARNList: arns,
	})
	if tagsErr != nil {
		if tagsRequired {
			return nil, tagsErr
		} else if !missingPermissionTagsAlreadyLogged {
			log.Warn().Err(tagsErr).Msg("Error fetching tags for lambda functions. Tags will be missing in the discovery.")
			missingPermissionTagsAlreadyLogged = true
		}
	}
	var tags []tagTypes.ResourceTagMapping
	if tagsResult != nil && tagsResult.ResourceTagMappingList != nil {
		tags = tagsResult.ResourceTagMappingList
	}
	return tags, nil
}

func toTarget(function types.FunctionConfiguration, ec2Util extec2.GetVpcNameUtil, awsAccountNumber string, awsAccountRegion string, role *string, tags []tagTypes.Tag) discovery_kit_api.Target {
	arn := aws.ToString(function.FunctionArn)
	name := aws.ToString(function.FunctionName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsAccountRegion}
	attributes["aws.arn"] = []string{arn}
	if function.VpcConfig != nil && function.VpcConfig.VpcId != nil {
		attributes["aws.vpc.id"] = []string{aws.ToString(function.VpcConfig.VpcId)}
		attributes["aws.vpc.name"] = []string{ec2Util.GetVpcName(awsAccountNumber, awsAccountRegion, aws.ToString(function.VpcConfig.VpcId))}
	}
	attributes["aws.lambda.function-name"] = []string{name}
	attributes["aws.lambda.runtime"] = []string{string(function.Runtime)}
	attributes["aws.role"] = []string{aws.ToString(function.Role)}
	attributes["aws.lambda.handler"] = []string{aws.ToString(function.Handler)}
	attributes["aws.lambda.code-size"] = []string{strconv.FormatInt(function.CodeSize, 10)}
	attributes["aws.lambda.description"] = []string{aws.ToString(function.Description)}
	if function.Timeout != nil {
		attributes["aws.lambda.timeout"] = []string{strconv.FormatInt(int64(*function.Timeout), 10)}
	}
	if function.MemorySize != nil {
		attributes["aws.lambda.memory-size"] = []string{strconv.FormatInt(int64(*function.MemorySize), 10)}
	}
	attributes["aws.lambda.last-modified"] = []string{aws.ToString(function.LastModified)}
	attributes["aws.lambda.version"] = []string{aws.ToString(function.Version)}
	attributes["aws.lambda.revision-id"] = []string{aws.ToString(function.RevisionId)}
	attributes["aws.lambda.package-type"] = []string{string(function.PackageType)}
	if function.Environment != nil && function.Environment.Variables != nil {
		attributes["aws.lambda.failure-injection-param"] = []string{function.Environment.Variables["FAILURE_INJECTION_PARAM"]}
	}

	architectures := make([]string, len(function.Architectures))
	for i, architecture := range function.Architectures {
		architectures[i] = string(architecture)
	}
	attributes["aws.lambda.architecture"] = architectures
	if function.MasterArn != nil {
		attributes["aws.lambda.master-arn"] = []string{aws.ToString(function.MasterArn)}
	}

	for _, tag := range tags {
		attributes[fmt.Sprintf("aws.lambda.label.%s", strings.ToLower(aws.ToString(tag.Key)))] = []string{aws.ToString(tag.Value)}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: lambdaTargetID,
		Attributes: attributes,
	}
}
