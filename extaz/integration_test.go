// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extaz

import "testing"

func TestWithTestContainers(t *testing.T) {
	WithTestContainers(t, []WithTestContainersCase{
		{
			Name: "prepare & start & stop blackhole attack with localstack",
			Test: testPrepareAndStartAndStopBlackholeLocalStack,
		},
	})
}
