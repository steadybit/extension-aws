// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/rs/zerolog/log"
	"github.com/sosodev/duration"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-aws/utils"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"strings"
	"time"
)

type fisTemplateDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*fisTemplateDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*fisTemplateDiscovery)(nil)
)

func NewFisTemplateDiscovery(ctx context.Context) discovery_kit_sdk.TargetDiscovery {
	discovery := &fisTemplateDiscovery{}
	return discovery_kit_sdk.NewCachedTargetDiscovery(discovery,
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(ctx, time.Duration(config.Config.DiscoveryIntervalFis)*time.Second),
	)
}

func (f *fisTemplateDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: fisTargetId,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr(fmt.Sprintf("%ds", config.Config.DiscoveryIntervalFis)),
		},
	}
}

func (f *fisTemplateDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       fisTargetId,
		Label:    discovery_kit_api.PluralLabel{One: "FIS experiment", Other: "FIS experiments"},
		Category: extutil.Ptr("cloud"),
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(fisIcon),

		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "aws.fis.experiment.template.name"},
				{Attribute: "aws.fis.experiment.template.description"},
				{Attribute: "aws.fis.experiment.template.duration"},
				{Attribute: "aws.account"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "aws.fis.experiment.template.name",
					Direction: "ASC",
				},
			},
		},
	}
}

func (f *fisTemplateDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{
			Attribute: "aws.fis.experiment.template.name",
			Label: discovery_kit_api.PluralLabel{
				One:   "template name",
				Other: "template names",
			},
		}, {
			Attribute: "aws.fis.experiment.template.description",
			Label: discovery_kit_api.PluralLabel{
				One:   "description",
				Other: "descriptions",
			},
		}, {
			Attribute: "aws.fis.experiment.template.duration",
			Label: discovery_kit_api.PluralLabel{
				One:   "duration",
				Other: "durations",
			},
		},
	}
}
func (f *fisTemplateDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	return utils.ForEveryConfiguredAwsAccess(getTargetsForAccount, ctx, "fis-template")
}
func getTargetsForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	client := fis.NewFromConfig(account.AwsConfig)
	result, err := GetAllFisTemplates(ctx, client, account)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover fis experiment templates for account %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_FIS=true. Details: %s", account.AccountNumber, re.Error())
			return []discovery_kit_api.Target{}, nil
		}
		return nil, err
	}
	return result, nil
}

type FisApi interface {
	fis.ListExperimentTemplatesAPIClient
	GetExperimentTemplate(ctx context.Context, params *fis.GetExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error)
}

func GetAllFisTemplates(ctx context.Context, fisApi FisApi, account *utils.AwsAccess) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	paginator := fis.NewListExperimentTemplatesPaginator(fisApi, &fis.ListExperimentTemplatesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return result, err
		}

		for _, template := range output.ExperimentTemplates {
			totalDuration, err := getTotalDuration(ctx, fisApi, template.Id, template.LastUpdateTime)
			if err != nil {
				return result, err
			}
			if matchesTagFilter(template.Tags, account.TagFilters) {
				result = append(result, toTarget(template, account.AccountNumber, account.Region, account.AssumeRole, totalDuration))
			}
		}
	}

	return discovery_kit_commons.ApplyAttributeExcludes(result, config.Config.DiscoveryAttributesExcludesFis), nil
}

func matchesTagFilter(tags map[string]string, filters []config.TagFilter) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		matched := false
		if value, exists := tags[filter.Key]; exists {
			for _, filterValue := range filter.Values {
				if value == filterValue {
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

func toTarget(template types.ExperimentTemplateSummary, awsAccountNumber string, awsRegion string, role *string, totalDuration *time.Duration) discovery_kit_api.Target {

	name := template.Id
	nameTag, nameTagPresent := template.Tags["Name"]
	if nameTagPresent {
		name = &nameTag
	}

	attributes := make(map[string][]string)
	attributes["aws.account"] = []string{awsAccountNumber}
	attributes["aws.region"] = []string{awsRegion}
	attributes["aws.fis.experiment.template.id"] = []string{aws.ToString(template.Id)}
	attributes["aws.fis.experiment.template.name"] = []string{aws.ToString(name)}
	attributes["aws.fis.experiment.template.description"] = []string{aws.ToString(template.Description)}
	attributes["aws.fis.experiment.template.duration"] = []string{totalDuration.String()}
	for key, value := range template.Tags {
		if key == "Name" {
			continue
		}
		attributes[fmt.Sprintf("label.%s", strings.ToLower(key))] = []string{value}
	}
	if role != nil {
		attributes["extension-aws.discovered-by-role"] = []string{aws.ToString(role)}
	}

	return discovery_kit_api.Target{
		Id:         *template.Id,
		Label:      *name,
		TargetType: fisTargetId,
		Attributes: attributes,
	}
}

func getTotalDuration(ctx context.Context, fisApi FisApi, templateId *string, lastUpdateTime *time.Time) (*time.Duration, error) {
	cachedValue, cachedValuePresent := durationCache[*templateId]
	if !cachedValuePresent || cachedValue.created.Before(*lastUpdateTime) {
		result, err := fisApi.GetExperimentTemplate(ctx, &fis.GetExperimentTemplateInput{Id: templateId})
		if err != nil {
			return nil, err
		}
		totalDuration := calculateTotalDuration(result.ExperimentTemplate)
		durationCache[*templateId] = templateDurationCacheEntry{
			created:  time.Now(),
			duration: *totalDuration,
		}
		return totalDuration, nil
	}
	return &cachedValue.duration, nil
}

func calculateTotalDuration(experimentTemplate *types.ExperimentTemplate) *time.Duration {
	actions := experimentTemplate.Actions
	totalDurations := make(map[string]time.Duration)

	iterationCount := 1
	for {
		if len(totalDurations) < len(actions) {
			for actionName, action := range actions {
				// if we have already calculated the duration for this action, skip it
				_, alreadyCalculated := totalDurations[actionName]
				if alreadyCalculated {
					continue
				}
				if len(action.StartAfter) > 0 {
					// if the action starts after other actions, check if we have total times for all actions that start before it
					allRequiredDurationsCalculated := true
					for _, startAfterAction := range action.StartAfter {
						_, present := totalDurations[startAfterAction]
						if !present {
							allRequiredDurationsCalculated = false
						}
					}
					if allRequiredDurationsCalculated {
						for _, startAfterAction := range action.StartAfter {
							startAfterActionDuration := totalDurations[startAfterAction]
							currentTotalDuration := startAfterActionDuration + getDuration(&action)
							// save the longest duration
							existingTotalDuration, existingTotalDurationPresent := totalDurations[actionName]
							if !existingTotalDurationPresent || currentTotalDuration > existingTotalDuration {
								totalDurations[actionName] = currentTotalDuration
							}
						}
					}
				} else {
					// if the action doesn't start after other actions, just calculate the duration
					totalDurations[actionName] = getDuration(&action)
				}
			}
		} else {
			break
		}
		iterationCount = iterationCount + 1
		log.Trace().Msgf("Iteration %d: Bubble calculate durations for FIS Experiment %s: %v", iterationCount, *experimentTemplate.Id, totalDurations)
	}

	log.Debug().Msgf("Calculated total durations for FIS Experiment %s: %v", *experimentTemplate.Id, totalDurations)
	longestDuration := time.Duration(0)
	for _, d := range totalDurations {
		if d > longestDuration {
			longestDuration = d
		}
	}
	return &longestDuration
}

func getDuration(action *types.ExperimentTemplateAction) time.Duration {
	durationString, durationStringPresent := action.Parameters["duration"]
	if durationStringPresent {
		parsed, err := duration.Parse(durationString)
		if err == nil {
			return parsed.ToTimeDuration()
		} else {
			log.Error().Msgf("Could not parse duration %s", durationString)
		}
	}
	return time.Duration(0)
}

var (
	durationCache = make(map[string]templateDurationCacheEntry)
)

type templateDurationCacheEntry struct {
	created  time.Time
	duration time.Duration
}
