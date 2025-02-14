// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"regexp"
	"strings"
)

var (
	Config Specification
)

func ParseConfiguration(rootRegion string) {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}

	Config.AssumeRoles = trimSpaces(Config.AssumeRoles)
	Config.Regions = trimSpaces(Config.Regions)
	Config.EnrichEc2DataForTargetTypes = trimSpaces(Config.EnrichEc2DataForTargetTypes)

	if len(Config.Regions) == 0 {
		Config.Regions = []string{rootRegion}
	}

	if Config.DisableDiscoveryExcludes {
		log.Info().Msg("Discovery excludes are disabled. Will also discover containers labeled with steadybit.com/discovery-disabled.")
	}
	if Config.AssumeRoles != nil && Config.AssumeRolesAdvanced != nil {
		log.Fatal().Msg("You can only specify either `assumeRoles` or `assumeRolesAdvanced`.")
	}
	translateToAssumeRolesAdvanced()
	err = verifyAssumeRolesAdvanced()
	if err != nil {
		log.Fatal().Err(err)
	}

	if Config.AssumeRolesAdvanced != nil {
		log.Info().Msgf("Using assume role configuration: %v", Config.AssumeRolesAdvanced)
	}
}

func trimSpaces(orig []string) []string {
	var trimmed []string
	for _, s := range orig {
		trimmed = append(trimmed, strings.TrimSpace(s))
	}
	return trimmed
}

func verifyAssumeRolesAdvanced() error {
	if Config.AssumeRolesAdvanced != nil {
		existingAccount := make(map[string]bool)
		existingRoles := make(map[string]bool)
		for _, role := range Config.AssumeRolesAdvanced {
			account := getAccountNumberFromArn(role.AssumeRole)
			for _, region := range role.Regions {
				_, roleAndRegionAlreadyConfigured := existingRoles[role.AssumeRole+"/"+region]
				if roleAndRegionAlreadyConfigured {
					return fmt.Errorf("you have configured the same role-arn for the same region twice. (arn: '%s', region: '%s')", role.AssumeRole, region)
				} else {
					existingRoles[role.AssumeRole+"/"+region] = true
				}

				_, accountAndRegionAlreadyConfigured := existingAccount[account+"/"+region]
				if accountAndRegionAlreadyConfigured {
					if role.TagFilters == nil {
						return fmt.Errorf("you have configured multiple role-arn for the same account '%s'. you need to set up tag filters to separate the discovered targets by each role", account)
					}
				} else {
					existingAccount[account+"/"+region] = true
				}
			}
		}
	}
	return nil
}

func getAccountNumberFromArn(arn string) string {
	re := regexp.MustCompile(`^arn:aws:iam::(\d{12}):role/[\w+=,.@-]+$`)
	matches := re.FindStringSubmatch(arn)
	if len(matches) < 2 {
		return "unknown"
	}
	return matches[1]
}

func translateToAssumeRolesAdvanced() {
	// if advanced assume roles are used, apply simple tag filters and regions to all roles that do not specify an own filter
	if Config.AssumeRolesAdvanced != nil {
		for i, assumeRole := range Config.AssumeRolesAdvanced {
			if assumeRole.TagFilters == nil {
				Config.AssumeRolesAdvanced[i].TagFilters = Config.TagFilters
			}
			if assumeRole.Regions == nil {
				Config.AssumeRolesAdvanced[i].Regions = Config.Regions
			}
		}
	}
	// create advanced assume roles from simple assume roles
	if Config.AssumeRoles != nil {
		for _, assumeRole := range Config.AssumeRoles {
			Config.AssumeRolesAdvanced = append(Config.AssumeRolesAdvanced, AssumeRole{assumeRole, Config.Regions, Config.TagFilters})
		}
	}
}
