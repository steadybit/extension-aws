// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extecs

import "github.com/steadybit/action-kit/go/action_kit_api/v2"

const groupTrafficFilter = "Traffic Filter"

// An empty IP/CIDR filter is sent as 0.0.0.0/0.
var trafficFilterGroup = action_kit_api.ParameterGroup{
	Name:        groupTrafficFilter,
	Description: "Leave unchanged to affect all outgoing traffic.",
}
