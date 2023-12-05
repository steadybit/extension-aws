package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sync"
	"testing"
)

type ec2ClientMock struct {
	mock.Mock
}

func (m *ec2ClientMock) DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeAvailabilityZonesOutput), args.Error(1)
}

func TestAwsZones(t *testing.T) {
	// Given
	Zones = &AwsZones{
		zones: sync.Map{},
	}
	mockedApi42 := new(ec2ClientMock)
	mockedReturnValue42 := ec2.DescribeAvailabilityZonesOutput{
		AvailabilityZones: []types.AvailabilityZone{
			{
				ZoneName:   discovery_kit_api.Ptr("eu-central-1a"),
				RegionName: discovery_kit_api.Ptr("eu-central-1"),
				ZoneId:     discovery_kit_api.Ptr("euc1-az1"),
			},
			{
				ZoneName:   discovery_kit_api.Ptr("eu-central-1b"),
				RegionName: discovery_kit_api.Ptr("eu-central-1"),
				ZoneId:     discovery_kit_api.Ptr("euc1-az3"),
			},
		},
	}
	mockedApi42.On("DescribeAvailabilityZones", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue42, nil)

	mockedApi4711 := new(ec2ClientMock)
	mockedReturnValue4711 := ec2.DescribeAvailabilityZonesOutput{
		AvailabilityZones: []types.AvailabilityZone{},
	}
	mockedApi4711.On("DescribeAvailabilityZones", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue4711, nil)

	// When
	result, err := initAwsZonesForAccountWithClient(mockedApi42, "42", context.Background())
	assert.Nil(t, result)
	assert.Nil(t, err)
	result, err = initAwsZonesForAccountWithClient(mockedApi4711, "4711", context.Background())
	assert.Nil(t, result)
	assert.Nil(t, err)

	// Then
	assert.Equal(t, &mockedReturnValue42.AvailabilityZones[0], Zones.GetZone("42", "eu-central-1a"))
	assert.Nil(t, Zones.GetZone("42", "eu-central-1c"))
	assert.Nil(t, Zones.GetZone("4711", "eu-central-1a"))

	assert.Equal(t, mockedReturnValue42.AvailabilityZones, Zones.GetZones("42"))
	assert.Equal(t, []types.AvailabilityZone{}, Zones.GetZones("4711"))
	assert.Equal(t, []types.AvailabilityZone{}, Zones.GetZones("0815"))
}
