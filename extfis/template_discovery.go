// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/rs/zerolog/log"
	"github.com/sosodev/duration"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"strings"
	"time"
)

func RegisterFisInstanceDiscoveryHandlers() {
	exthttp.RegisterHttpHandler("/fis/template/discovery", exthttp.GetterAsHandler(getFisTemplateDiscoveryDescription))
	exthttp.RegisterHttpHandler("/fis/template/discovery/target-description", exthttp.GetterAsHandler(getFisTemplateTargetDescription))
	exthttp.RegisterHttpHandler("/fis/template/discovery/attribute-descriptions", exthttp.GetterAsHandler(getFisTemplateAttributeDescriptions))
	exthttp.RegisterHttpHandler("/fis/template/discovery/discovered-targets", getFisTemplateTargets)
}

func getFisTemplateDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         fisTargetId,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         "/fis/template/discovery/discovered-targets",
			CallInterval: extutil.Ptr("300s"),
		},
	}
}

func getFisTemplateTargetDescription() discovery_kit_api.TargetDescription {
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

func getFisTemplateAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
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
		},
	}
}

func getFisTemplateTargets(w http.ResponseWriter, r *http.Request, _ []byte) {
	targets, err := utils.ForEveryAccount(utils.Accounts, getTargetsForAccount, mergeTargets, make([]discovery_kit_api.Target, 0, 100), r.Context())
	if err != nil {
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect FIS Template information", err))
	} else {
		exthttp.WriteBody(w, discovery_kit_api.DiscoveredTargets{Targets: targets})
	}
}

func getTargetsForAccount(account *utils.AwsAccount, ctx context.Context) (*[]discovery_kit_api.Target, error) {
	client := fis.NewFromConfig(account.AwsConfig)
	targets, err := GetAllFisTemplates(ctx, client, account.AccountNumber, account.AwsConfig.Region)
	if err != nil {
		return nil, err
	}
	return &targets, nil
}

func mergeTargets(merged []discovery_kit_api.Target, eachResult []discovery_kit_api.Target) ([]discovery_kit_api.Target, error) {
	return append(merged, eachResult...), nil
}

type FisApi interface {
	ListExperimentTemplates(ctx context.Context, params *fis.ListExperimentTemplatesInput, optFns ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error)
	GetExperimentTemplate(ctx context.Context, params *fis.GetExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error)
}

func GetAllFisTemplates(ctx context.Context, fisApi FisApi, awsAccountNumber string, awsRegion string) ([]discovery_kit_api.Target, error) {
	result := make([]discovery_kit_api.Target, 0, 20)

	var nextToken *string = nil
	for {
		output, err := fisApi.ListExperimentTemplates(ctx, &fis.ListExperimentTemplatesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return result, err
		}

		for _, template := range output.ExperimentTemplates {
			totalDuration, err := getTotalDuration(ctx, fisApi, template.Id, template.LastUpdateTime)
			if err != nil {
				return result, err
			}
			result = append(result, toTarget(template, awsAccountNumber, awsRegion, totalDuration))
		}

		if output.NextToken == nil {
			break
		} else {
			nextToken = output.NextToken
		}
	}

	return result, nil
}

func toTarget(template types.ExperimentTemplateSummary, awsAccountNumber string, awsRegion string, totalDuration *time.Duration) discovery_kit_api.Target {

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
	for _, duration := range totalDurations {
		if duration > longestDuration {
			longestDuration = duration
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
