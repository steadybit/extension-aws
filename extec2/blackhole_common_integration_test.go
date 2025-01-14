// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extec2

import (
	"testing"
)

func TestWithTestContainers(t *testing.T) {
	WithTestContainers(t,
		[]WithTestContainersCase{
			{
				Name: "AZ Blackhole - prepare & start & stop blackhole attack",
				Test: testPrepareAndStartAndStopBlackholeLocalStack,
			},
			{
				Name: "AZ Blackhole - API Throttling during start while creating the second NACL",
				Test: testApiThrottlingDuringStartWhileCreatingTheSecondNACL,
			},
			{
				Name: "AZ Blackhole - API Throttling during start while assigning the first NACL",
				Test: testApiThrottlingDuringStartWhileAssigningTheFirstNACL,
			},
			{
				Name: "AZ Blackhole - API Throttling during stop while re-assigning the old NACLs",
				Test: testApiThrottlingDuringStopWhileReassigningTheOldNACLs,
			},
			{
				Name: "AZ Blackhole - API Throttling during stop while deleting NACLs",
				Test: testApiThrottlingDuringStopWhileDeletingNACLs,
			},
			{
				Name: "Subnet Blackhole - prepare & start & stop blackhole attack",
				Test: testPrepareAndStartAndStopBlackholeSubnetLocalStack,
			},
		},
	)
}
