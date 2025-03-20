// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/steadybit/extension-kit/extutil"
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
		Region:        "eu-central-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
	})

	// Then
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, azTargetType, target.TargetType)
	assert.Equal(t, "eu-central-1b", target.Label)
	assert.Equal(t, 6, len(target.Attributes))
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"eu-central-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"eu-central-1b"}, target.Attributes["aws.zone"])
	assert.Equal(t, []string{"euc1-az3"}, target.Attributes["aws.zone.id"])
	assert.Equal(t, []string{"eu-central-1b@42"}, target.Attributes["aws.zone@account"])
	assert.Equal(t, []string{"arn:aws:iam::42:role/extension-aws-role"}, target.Attributes["extension-aws.discovered-by-role"])
}

func TestGetNoAvailabilityZonesIfTagFilterIsSet(t *testing.T) {
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
		Region:        "eu-central-1",
		AssumeRole:    extutil.Ptr("arn:aws:iam::42:role/extension-aws-role"),
		TagFilters: []extConfig.TagFilter{
			{
				Key:    "application",
				Values: []string{"demo"},
			},
		},
	})

	// Then
	assert.Empty(t, targets)
}
