// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

func TestGetAllAvailabilityZones(t *testing.T) {
	// Given
	mockedApi := new(ec2UtilMock)
	mockedReturnValue := []types.AvailabilityZone{
		{
			ZoneName:   discovery_kit_api.Ptr("eu-central-1b"),
			RegionName: discovery_kit_api.Ptr("eu-central-1"),
			ZoneId:     discovery_kit_api.Ptr("euc1-az3"),
		},
	}
	mockedApi.On("GetZones", mock.Anything).Return(mockedReturnValue)

	// When
	targets := getAllAvailabilityZonesFromCache(mockedApi, &utils.AwsAccess{
		AccountNumber: "42",
	})

	// Then
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, azTargetType, target.TargetType)
	assert.Equal(t, "eu-central-1b", target.Label)
	assert.Equal(t, 5, len(target.Attributes))
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"eu-central-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"eu-central-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"euc1-az3"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"eu-central-1b@42"}, target.Attributes["aws.zone@account"])
}
