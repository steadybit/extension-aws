// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extaz

import (
	"testing"
)

func TestWithTestContainers(t *testing.T) {
	WithTestContainers(t,
		[]WithTestContainersCase{
			{
				Name: "prepare & start & stop blackhole attack",
				Test: testPrepareAndStartAndStopBlackholeLocalStack,
			},
			{
				Name: "API Throttling during start while creating the second NACL",
				Test: testApiThrottlingDuringStartWhileCreatingTheSecondNACL,
			},
			{
				Name: "API Throttling during start while assigning the first NACL",
				Test: testApiThrottlingDuringStartWhileAssigningTheFirstNACL,
			},
			{
				Name: "API Throttling during stop while re-assigning the old NACLs",
				Test: testApiThrottlingDuringStopWhileReassigningTheOldNACLs,
			},
			{
				Name: "API Throttling during stop while deleting NACLs",
				Test: testApiThrottlingDuringStopWhileDeletingNACLs,
			},
		},
	)
}
