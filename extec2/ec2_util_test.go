package extec2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/v2/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sync"
	"testing"
)

type ec2UtilsClientMock struct {
	mock.Mock
}

func (m *ec2UtilsClientMock) DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ec2.DescribeAvailabilityZonesOutput), args.Error(1)
}

func TestAwsZones(t *testing.T) {
	// Given
	Util = &util{
		zones: sync.Map{},
	}
	mockedApi42 := new(ec2UtilsClientMock)
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

	mockedApi4711 := new(ec2UtilsClientMock)
	mockedReturnValue4711 := ec2.DescribeAvailabilityZonesOutput{
		AvailabilityZones: []types.AvailabilityZone{},
	}
	mockedApi4711.On("DescribeAvailabilityZones", mock.Anything, mock.Anything, mock.Anything).Return(&mockedReturnValue4711, nil)

	// When
	initZonesCache(mockedApi42, "42", "eu-central-1", context.Background())
	initZonesCache(mockedApi4711, "4711", "eu-central-1", context.Background())

	// Then
	assert.Equal(t, &mockedReturnValue42.AvailabilityZones[0], Util.GetZone("42", "eu-central-1", "eu-central-1a"))
	assert.Nil(t, Util.GetZone("42", "eu-central-1", "eu-central-1c"))
	assert.Nil(t, Util.GetZone("4711", "eu-central-1", "eu-central-1a"))

	assert.Equal(t, mockedReturnValue42.AvailabilityZones, Util.GetZones(&utils.AwsAccess{AccountNumber: "42", Region: "eu-central-1"}))
	assert.Equal(t, []types.AvailabilityZone{}, Util.GetZones(&utils.AwsAccess{AccountNumber: "4711", Region: "eu-central-1"}))
	assert.Equal(t, []types.AvailabilityZone{}, Util.GetZones(&utils.AwsAccess{AccountNumber: "0815", Region: "eu-central-1"}))
}
