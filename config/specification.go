// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package config

type Specification struct {
	AssumeRoles []string `json:"assumeRoles" split_words:"true" required:"false"`
}
