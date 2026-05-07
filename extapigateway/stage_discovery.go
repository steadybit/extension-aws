// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extapigateway

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extbuild"
)

const (
	protocolREST      = "REST"
	protocolHTTP      = "HTTP"
	protocolWEBSOCKET = "WEBSOCKET"
)

type stageDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*stageDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*stageDiscovery)(nil)
)

func NewStageDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&stageDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalApigateway)*time.Second),
	)
}

func (d *stageDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: stageTargetType,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: new(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalApigateway)),
		},
	}
}

func (d *stageDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       stageTargetType,
		Label:    discovery_kit_api.PluralLabel{One: "API Gateway stage", Other: "API Gateway stages"},
		Category: new("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(apiGatewayIcon),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "aws.apigateway.api.protocol-type"},
				{Attribute: "aws.apigateway.stage.throttle.rate-limit"},
				{Attribute: "aws.apigateway.stage.tracing-enabled"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *stageDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "aws.apigateway.api.id", Label: discovery_kit_api.PluralLabel{One: "API Gateway API ID", Other: "API Gateway API IDs"}},
		{Attribute: "aws.apigateway.api.name", Label: discovery_kit_api.PluralLabel{One: "API Gateway API name", Other: "API Gateway API names"}},
		{Attribute: "aws.apigateway.api.protocol-type", Label: discovery_kit_api.PluralLabel{One: "API Gateway protocol type", Other: "API Gateway protocol types"}},
		{Attribute: "aws.apigateway.api.endpoint-type", Label: discovery_kit_api.PluralLabel{One: "API Gateway endpoint type", Other: "API Gateway endpoint types"}},
		{Attribute: "aws.apigateway.api.disable-execute-api-endpoint", Label: discovery_kit_api.PluralLabel{One: "API Gateway disable execute-api endpoint", Other: "API Gateway disable execute-api endpoint"}},
		{Attribute: "aws.apigateway.stage.name", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage name", Other: "API Gateway stage names"}},
		{Attribute: "aws.apigateway.stage.throttle.rate-limit", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage throttle rate", Other: "API Gateway stage throttle rates"}},
		{Attribute: "aws.apigateway.stage.throttle.burst-limit", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage throttle burst", Other: "API Gateway stage throttle bursts"}},
		{Attribute: "aws.apigateway.stage.cache.enabled", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage cache", Other: "API Gateway stage cache"}},
		{Attribute: "aws.apigateway.stage.cache.size", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage cache size", Other: "API Gateway stage cache sizes"}},
		{Attribute: "aws.apigateway.stage.tracing-enabled", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage X-Ray tracing", Other: "API Gateway stage X-Ray tracing"}},
		{Attribute: "aws.apigateway.stage.logging-level", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage logging level", Other: "API Gateway stage logging levels"}},
		{Attribute: "aws.apigateway.stage.access-log.configured", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage access log", Other: "API Gateway stage access log"}},
		{Attribute: "aws.apigateway.stage.waf-arn", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage WAF ARN", Other: "API Gateway stage WAF ARNs"}},
		{Attribute: "aws.apigateway.stage.client-certificate-id", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage client certificate", Other: "API Gateway stage client certificates"}},
		{Attribute: "aws.apigateway.stage.auto-deploy", Label: discovery_kit_api.PluralLabel{One: "API Gateway stage auto-deploy", Other: "API Gateway stage auto-deploy"}},
	}
}

func (d *stageDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getStageTargets, ctx, "apigateway-stage")
}

func getStageTargets(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	rest := apigateway.NewFromConfig(account.AwsConfig)
	httpApi := apigatewayv2.NewFromConfig(account.AwsConfig)
	result, err := getAllStages(ctx, rest, httpApi, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover API Gateway stages for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_APIGATEWAY=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

func getAllStages(ctx context.Context, rest RestApiGatewayApi, httpApi HttpApiGatewayApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 10)

	restTargets, err := getRestStages(ctx, rest, account)
	if err != nil {
		return nil, err
	}
	result = append(result, restTargets...)

	httpTargets, err := getHttpStages(ctx, httpApi, account)
	if err != nil {
		// Treat HTTP API errors as non-fatal: a permission gap in v2 shouldn't kill the v1 discovery.
		log.Warn().Err(err).Msg("Failed to discover HTTP API Gateway stages; v2 stages will be missing.")
	} else {
		result = append(result, httpTargets...)
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesApigateway), nil
}

func getRestStages(ctx context.Context, client RestApiGatewayApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0)
	paginator := apigateway.NewGetRestApisPaginator(client, &apigateway.GetRestApisInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, api := range page.Items {
			if api.Id == nil {
				continue
			}
			if !matchesApigwTagFilter(api.Tags, account.TagFilters) {
				continue
			}
			stagesOut, err := client.GetStages(ctx, &apigateway.GetStagesInput{RestApiId: api.Id})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to list stages for REST API %s", aws.ToString(api.Id))
				continue
			}
			for _, stage := range stagesOut.Item {
				result = append(result, toRestStageTarget(api, stage, account.AccountNumber, account.Region, account.AssumeRole))
			}
		}
	}
	return result, nil
}

func getHttpStages(ctx context.Context, client HttpApiGatewayApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0)
	var nextToken *string
	for {
		out, err := client.GetApis(ctx, &apigatewayv2.GetApisInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, api := range out.Items {
			if api.ApiId == nil {
				continue
			}
			if !matchesApigwTagFilter(api.Tags, account.TagFilters) {
				continue
			}
			stagesOut, err := client.GetStages(ctx, &apigatewayv2.GetStagesInput{ApiId: api.ApiId})
			if err != nil {
				log.Warn().Err(err).Msgf("Failed to list stages for HTTP API %s", aws.ToString(api.ApiId))
				continue
			}
			for _, stage := range stagesOut.Items {
				result = append(result, toHttpStageTarget(api, stage, account.AccountNumber, account.Region, account.AssumeRole))
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return result, nil
}

func matchesApigwTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		matched := false
		if v, ok := tags[filter.Key]; ok {
			for _, want := range filter.Values {
				if v == want {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func toRestStageTarget(api apigwtypes.RestApi, stage apigwtypes.Stage, account string, region string, role *string) discovery_kit_api.Target {
	apiId := aws.ToString(api.Id)
	apiName := aws.ToString(api.Name)
	stageName := aws.ToString(stage.StageName)
	id := fmt.Sprintf("rest:%s:%s:%s/%s", account, region, apiId, stageName)
	label := fmt.Sprintf("%s/%s", apiName, stageName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.apigateway.api.id"] = []string{apiId}
	attributes["aws.apigateway.api.name"] = []string{apiName}
	attributes["aws.apigateway.api.protocol-type"] = []string{protocolREST}
	attributes["aws.apigateway.stage.name"] = []string{stageName}

	endpointType := "REGIONAL"
	if api.EndpointConfiguration != nil && len(api.EndpointConfiguration.Types) > 0 {
		endpointType = string(api.EndpointConfiguration.Types[0])
	}
	attributes["aws.apigateway.api.endpoint-type"] = []string{endpointType}

	attributes["aws.apigateway.api.disable-execute-api-endpoint"] = []string{strconv.FormatBool(api.DisableExecuteApiEndpoint)}

	rate, burst := readRestStageThrottle(stage)
	if rate >= 0 {
		attributes["aws.apigateway.stage.throttle.rate-limit"] = []string{strconv.FormatFloat(rate, 'f', -1, 64)}
	}
	if burst >= 0 {
		attributes["aws.apigateway.stage.throttle.burst-limit"] = []string{strconv.Itoa(int(burst))}
	}

	attributes["aws.apigateway.stage.cache.enabled"] = []string{strconv.FormatBool(stage.CacheClusterEnabled)}
	if stage.CacheClusterEnabled && stage.CacheClusterSize != "" {
		attributes["aws.apigateway.stage.cache.size"] = []string{string(stage.CacheClusterSize)}
	}
	attributes["aws.apigateway.stage.tracing-enabled"] = []string{strconv.FormatBool(stage.TracingEnabled)}

	if loggingLevel := readRestStageLoggingLevel(stage); loggingLevel != "" {
		attributes["aws.apigateway.stage.logging-level"] = []string{loggingLevel}
	}
	attributes["aws.apigateway.stage.access-log.configured"] = []string{strconv.FormatBool(stage.AccessLogSettings != nil && stage.AccessLogSettings.DestinationArn != nil && *stage.AccessLogSettings.DestinationArn != "")}

	if stage.WebAclArn != nil && *stage.WebAclArn != "" {
		attributes["aws.apigateway.stage.waf-arn"] = []string{*stage.WebAclArn}
	}
	if stage.ClientCertificateId != nil && *stage.ClientCertificateId != "" {
		attributes["aws.apigateway.stage.client-certificate-id"] = []string{*stage.ClientCertificateId}
	}

	for k, v := range stage.Tags {
		attributes[fmt.Sprintf("aws.apigateway.stage.label.%s", strings.ToLower(k))] = []string{v}
	}
	for k, v := range api.Tags {
		// API-level tags inherited (avoid clobbering stage tags by using a different prefix).
		key := fmt.Sprintf("aws.apigateway.api.label.%s", strings.ToLower(k))
		if _, exists := attributes[key]; !exists {
			attributes[key] = []string{v}
		}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      label,
		TargetType: stageTargetType,
		Attributes: attributes,
	}
}

func readRestStageThrottle(stage apigwtypes.Stage) (rate float64, burst int32) {
	rate, burst = -1, -1
	if ms, ok := stage.MethodSettings["*/*"]; ok {
		if ms.ThrottlingRateLimit != 0 {
			rate = ms.ThrottlingRateLimit
		}
		if ms.ThrottlingBurstLimit != 0 {
			burst = ms.ThrottlingBurstLimit
		}
	}
	return rate, burst
}

func readRestStageLoggingLevel(stage apigwtypes.Stage) string {
	if ms, ok := stage.MethodSettings["*/*"]; ok && ms.LoggingLevel != nil {
		return *ms.LoggingLevel
	}
	return ""
}

func toHttpStageTarget(api apigwv2types.Api, stage apigwv2types.Stage, account string, region string, role *string) discovery_kit_api.Target {
	apiId := aws.ToString(api.ApiId)
	apiName := aws.ToString(api.Name)
	stageName := aws.ToString(stage.StageName)
	id := fmt.Sprintf("http:%s:%s:%s/%s", account, region, apiId, stageName)
	label := fmt.Sprintf("%s/%s", apiName, stageName)

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{account}
	attributes["aws.region"] = []string{region}
	attributes["aws.apigateway.api.id"] = []string{apiId}
	attributes["aws.apigateway.api.name"] = []string{apiName}
	attributes["aws.apigateway.api.protocol-type"] = []string{string(api.ProtocolType)}
	attributes["aws.apigateway.stage.name"] = []string{stageName}

	if api.DisableExecuteApiEndpoint != nil {
		attributes["aws.apigateway.api.disable-execute-api-endpoint"] = []string{strconv.FormatBool(*api.DisableExecuteApiEndpoint)}
	}

	if stage.DefaultRouteSettings != nil {
		if stage.DefaultRouteSettings.ThrottlingRateLimit != nil {
			attributes["aws.apigateway.stage.throttle.rate-limit"] = []string{strconv.FormatFloat(*stage.DefaultRouteSettings.ThrottlingRateLimit, 'f', -1, 64)}
		}
		if stage.DefaultRouteSettings.ThrottlingBurstLimit != nil {
			attributes["aws.apigateway.stage.throttle.burst-limit"] = []string{strconv.Itoa(int(*stage.DefaultRouteSettings.ThrottlingBurstLimit))}
		}
		if stage.DefaultRouteSettings.LoggingLevel != "" {
			attributes["aws.apigateway.stage.logging-level"] = []string{string(stage.DefaultRouteSettings.LoggingLevel)}
		}
	}

	attributes["aws.apigateway.stage.access-log.configured"] = []string{strconv.FormatBool(stage.AccessLogSettings != nil && stage.AccessLogSettings.DestinationArn != nil && *stage.AccessLogSettings.DestinationArn != "")}

	if stage.AutoDeploy != nil {
		attributes["aws.apigateway.stage.auto-deploy"] = []string{strconv.FormatBool(*stage.AutoDeploy)}
	}
	if stage.ClientCertificateId != nil && *stage.ClientCertificateId != "" {
		attributes["aws.apigateway.stage.client-certificate-id"] = []string{*stage.ClientCertificateId}
	}

	for k, v := range stage.Tags {
		attributes[fmt.Sprintf("aws.apigateway.stage.label.%s", strings.ToLower(k))] = []string{v}
	}
	for k, v := range api.Tags {
		key := fmt.Sprintf("aws.apigateway.api.label.%s", strings.ToLower(k))
		if _, exists := attributes[key]; !exists {
			attributes[key] = []string{v}
		}
	}

	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		Label:      label,
		TargetType: stageTargetType,
		Attributes: attributes,
	}
}
