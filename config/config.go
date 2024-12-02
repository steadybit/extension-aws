// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"strings"
)

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	Config.AssumeRoles = trimSpaces(Config.AssumeRoles)
	Config.Regions = trimSpaces(Config.Regions)
	Config.EnrichEc2DataForTargetTypes = trimSpaces(Config.EnrichEc2DataForTargetTypes)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func trimSpaces(orig []string) []string {
	var trimmed []string
	for _, s := range orig {
		trimmed = append(trimmed, strings.TrimSpace(s))
	}
	return trimmed
}
