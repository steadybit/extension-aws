package utils

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"sync"
)

var (
	Zones *AwsZones
)

type AwsZones struct {
	zones sync.Map
}

func InitializeAwsZones() {
	Zones = &AwsZones{
		zones: sync.Map{},
	}
	_, _ = ForEveryAccount(Accounts, initAwsZonesForAccount, context.Background(), "availability zone")
}

func initAwsZonesForAccount(account *AwsAccount, ctx context.Context) ([]discovery_kit_api.Target, error) {
	return initAwsZonesForAccountWithClient(ec2.NewFromConfig(account.AwsConfig), account.AccountNumber, ctx)
}

func initAwsZonesForAccountWithClient(client AZDescribeAvailabilityZonesApi, awsAccountNumber string, ctx context.Context) ([]discovery_kit_api.Target, error) {
	result := getAllAvailabilityZones(ctx, client, awsAccountNumber)
	Zones.zones.Store(awsAccountNumber, result)
	return nil, nil
}

type AZDescribeAvailabilityZonesApi interface {
	DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
}

func getAllAvailabilityZones(ctx context.Context, ec2Api AZDescribeAvailabilityZonesApi, awsAccountNumber string) []types.AvailabilityZone {
	output, err := ec2Api.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		AllAvailabilityZones: aws.Bool(false),
	})
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover availability cache for account %s. If this intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE=true. Details: %s", awsAccountNumber, re.Error())
			return []types.AvailabilityZone{}
		}
		log.Fatal().Err(err).Msgf("Failed to load availability cache for account %s.", awsAccountNumber)
		return []types.AvailabilityZone{}
	}
	return output.AvailabilityZones
}

type GetZoneUtil interface {
	GetZone(awsAccountNumber string, awsZone string) *types.AvailabilityZone
}
type GetZonesUtil interface {
	GetZones(awsAccountNumber string) []types.AvailabilityZone
}

func (zones AwsZones) GetZones(awsAccountNumber string) []types.AvailabilityZone {
	value, ok := zones.zones.Load(awsAccountNumber)
	if !ok {
		return []types.AvailabilityZone{}
	}
	return value.([]types.AvailabilityZone)
}

func (zones AwsZones) GetZone(awsAccountNumber string, awsZone string) *types.AvailabilityZone {
	value, ok := zones.zones.Load(awsAccountNumber)
	if !ok {
		return nil
	}
	for _, zone := range value.([]types.AvailabilityZone) {
		if aws.ToString(zone.ZoneName) == awsZone {
			return &zone
		}
	}
	return nil
}
