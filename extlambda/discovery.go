/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extlambda

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"strconv"
)

const discoveryBasePath = "/lambda/discovery"

func RegisterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler(discoveryBasePath, exthttp.GetterAsHandler(getDiscoveryDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/target-description", exthttp.GetterAsHandler(getTargetDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/attribute-descriptions", exthttp.GetterAsHandler(getAttributeDescriptions))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/discovered-targets", getDiscoveredTargets)
}

func GetDiscoveryEndpoints() discovery_kit_api.DiscoveryList {
	return discovery_kit_api.DiscoveryList{
		Discoveries: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath,
			},
		},
		TargetTypes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/target-description",
			},
		},
		TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/attribute-descriptions",
			},
		},
	}
}

func getDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         lambdaTargetID,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         discoveryBasePath + "/discovered-targets",
			CallInterval: extutil.Ptr("1m"),
		},
	}
}

func getTargetDescription() discovery_kit_api.TargetDescription {
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

func getAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
			{
				Attribute: "aws.lambda.function-name",
				Label: discovery_kit_api.PluralLabel{
					One:   "Function Name",
					Other: "Function Names",
				},
			},
			{
				Attribute: "aws.arn",
				Label: discovery_kit_api.PluralLabel{
					One:   "ARN",
					Other: "ARNs",
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
				Attribute: "aws.role",
				Label: discovery_kit_api.PluralLabel{
					One:   "Role",
					Other: "Roles",
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
				Attribute: "aws.lambda.memorySize",
				Label: discovery_kit_api.PluralLabel{
					One:   "Memory Size",
					Other: "Memory Sizes",
				},
			}, {
				Attribute: "aws.lambda.lastModified",
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
				Attribute: "aws.lambda.revisionId",
				Label: discovery_kit_api.PluralLabel{
					One:   "Revision ID",
					Other: "Revision IDs",
				},
			},
			{
				Attribute: "aws.lambda.packageType",
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
		},
	}
}

func getDiscoveredTargets(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect lambda function information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := lambda.NewFromConfig(account.AwsConfig)
	result, err := getAllAwsLambdaFunctions(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover lambda functions for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_LAMBDA=true. Details: %s", account.AccountNumber, re.Error())
			return extutil.Ptr([]discovery_kit_api.Target{}), nil
		}
		return nil, err
	}
	return result, nil
}

func getAllAwsLambdaFunctions(ctx context.Context, client lambda.ListFunctionsAPIClient, awsAccountNumber string, awsAccountRegion string) (*[]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 100)
	var marker *string = nil
	for {
		output, err := client.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker: marker,
		})
		if err != nil {
			return nil, err
		}

		for _, function := range output.Functions {
			result = append(result, toTarget(function, awsAccountNumber, awsAccountRegion))
		}

		if output.NextMarker == nil {
			break
		} else {
			marker = output.NextMarker
		}
	}
	return &result, nil
}

func mergeTargets(merged []discovery_kit_api.Target, eachResult []discovery_kit_api.Target) ([]discovery_kit_api.Target, error) {
	return append(merged, eachResult...), nil
}

func toTarget(function types.FunctionConfiguration, awsAccountNumber string, awsAccountRegion string) discovery_kit_api.Target {
	arn := aws.ToString(function.FunctionArn)
	name := aws.ToString(function.FunctionName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsAccountRegion}
	attributes["aws.arn"] = []string{arn}
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

	return discovery_kit_api.Target{
		Id:         arn,
		Label:      name,
		TargetType: lambdaTargetID,
		Attributes: attributes,
	}
}
