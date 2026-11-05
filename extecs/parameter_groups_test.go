// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extecs

import (
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
)

func Test_parameterGroups(t *testing.T) {
	for _, description := range []action_kit_api.ActionDescription{
		NewEcsTaskNetworkDelayAction().Describe(),
		NewEcsTaskNetworkLossAction().Describe(),
	} {
		t.Run(description.Id, func(t *testing.T) {
			assertParameterGroups(t, description)
			assert.NotEmpty(t, description.Parameters)

			// The IP/CIDR filter is the action's traffic filter; it stays advanced for platforms without groups.
			found := false
			for _, p := range description.Parameters {
				if p.Name == "ip" {
					found = true
					assert.Equal(t, groupTrafficFilter, *p.Group)
					assert.True(t, *p.Advanced)
				}
			}
			assert.True(t, found, "ip parameter")
			assert.Equal(t, []action_kit_api.ParameterGroup{trafficFilterGroup}, *description.ParameterGroups)
		})
	}
}

func assertParameterGroups(t *testing.T, description action_kit_api.ActionDescription) {
	t.Helper()
	usesOrder := false
	for _, p := range description.Parameters {
		if p.Order != nil && *p.Order != 0 {
			usesOrder = true
		}
	}
	sortKey := func(i int, p action_kit_api.ActionParameter) int {
		if !usesOrder {
			return i
		}
		if p.Order == nil {
			return 0
		}
		return *p.Order
	}
	inAdditionalSettings := func(p action_kit_api.ActionParameter) bool {
		return p.Group != nil && *p.Group == action_kit_api.ParameterGroupAdditionalSettings
	}

	maxOther := -1 << 31
	for i, p := range description.Parameters {
		if p.Advanced != nil && *p.Advanced {
			assert.NotNil(t, p.Group, "%s: advanced parameter %q has no group", description.Id, p.Name)
		}
		if p.Name == "failEarly" {
			assert.False(t, p.Advanced != nil && *p.Advanced, "%s: failEarly must not be advanced", description.Id)
			assert.Nil(t, p.Group, "%s: failEarly must not be in a group", description.Id)
		}
		if !inAdditionalSettings(p) {
			maxOther = max(maxOther, sortKey(i, p))
		}
	}
	for i, p := range description.Parameters {
		if inAdditionalSettings(p) {
			assert.Greater(t, sortKey(i, p), maxOther, "%s: %q must sort after all other parameters", description.Id, p.Name)
		}
	}
}
