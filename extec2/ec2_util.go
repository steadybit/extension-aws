package extec2

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-aws/v2/config"
	"github.com/steadybit/extension-aws/v2/utils"
	"sync"
)

var (
	Util *util
)

type util struct {
	zones sync.Map
	vpcs  sync.Map
}

func InitializeEc2Util() {
	Util = &util{
		zones: sync.Map{},
		vpcs:  sync.Map{},
	}
	_, _ = utils.ForEveryConfiguredAwsAccess(InitEc2UtilForAccount, context.Background(), "availability zone")
}

func InitEc2UtilForAccount(account *utils.AwsAccess, ctx context.Context) ([]discovery_kit_api.Target, error) {
	initZonesCache(ec2.NewFromConfig(account.AwsConfig), account.AccountNumber, account.Region, ctx)
	initVpcCache(ec2.NewFromConfig(account.AwsConfig), account.AccountNumber, account.Region, ctx)
	return nil, nil
}

type AZDescribeAvailabilityZonesApi interface {
	DescribeAvailabilityZones(ctx context.Context, params *ec2.DescribeAvailabilityZonesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
}

func initZonesCache(client AZDescribeAvailabilityZonesApi, awsAccountNumber string, region string, ctx context.Context) {
	output, err := client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		AllAvailabilityZones: aws.Bool(false),
	})
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
			log.Error().Msgf("Not Authorized to discover availability zones for account %s and region %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE=true. Details: %s", awsAccountNumber, region, re.Error())
			Util.zones.Store(awsAccountNumber+"-"+region, []types.AvailabilityZone{})
			return
		}
		log.Error().Err(err).Msgf("Failed to load availability zones for account %s and region %s.", awsAccountNumber, region)
		Util.zones.Store(awsAccountNumber+"-"+region, []types.AvailabilityZone{})
		return
	}
	Util.zones.Store(awsAccountNumber+"-"+region, output.AvailabilityZones)
}

func initVpcCache(client ec2.DescribeVpcsAPIClient, awsAccountNumber string, region string, ctx context.Context) {
	if !config.Config.DiscoveryDisabledVpc {
		output, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
		if err != nil {
			var re *awshttp.ResponseError
			if errors.As(err, &re) && re.HTTPStatusCode() == 403 {
				log.Error().Msgf("Not Authorized to discover vpcs for account %s and region %s. If this is intended, you can disable the discovery by setting STEADYBIT_EXTENSION_DISCOVERY_DISABLED_VPC=true. Details: %s", awsAccountNumber, region, re.Error())
				Util.vpcs.Store(awsAccountNumber+"-"+region, []types.Vpc{})
				return
			}
			log.Error().Err(err).Msgf("Failed to load vpcs for account %s and region %s.", awsAccountNumber, region)
			Util.vpcs.Store(awsAccountNumber+"-"+region, []types.Vpc{})
			return
		}
		Util.vpcs.Store(awsAccountNumber+"-"+region, output.Vpcs)
	} else {
		Util.vpcs.Store(awsAccountNumber+"-"+region, []types.Vpc{})
	}
}

type GetZoneUtil interface {
	GetZone(awsAccountNumber string, region string, awsZone string) *types.AvailabilityZone
}

func (zones *util) GetZones(account *utils.AwsAccess) []types.AvailabilityZone {
	awsAccountNumber := account.AccountNumber
	region := account.Region
	value, ok := zones.zones.Load(awsAccountNumber + "-" + region)
	if !ok {
		return []types.AvailabilityZone{}
	}
	return value.([]types.AvailabilityZone)
}

type GetVpcNameUtil interface {
	GetVpcName(awsAccountNumber string, region string, vpcId string) string
}

func (zones *util) GetVpcName(awsAccountNumber string, region string, vpcId string) string {
	value, ok := zones.vpcs.Load(awsAccountNumber + "-" + region)
	if !ok {
		return vpcId
	}
	for _, vpc := range value.([]types.Vpc) {
		if aws.ToString(vpc.VpcId) == vpcId {
			for _, tag := range vpc.Tags {
				if aws.ToString(tag.Key) == "Name" {
					return aws.ToString(tag.Value) + " (" + vpcId + ")"
				}
			}
		}
	}
	return vpcId
}

type GetZonesUtil interface {
	GetZones(account *utils.AwsAccess) []types.AvailabilityZone
}

func (zones *util) GetZone(awsAccountNumber string, region string, awsZone string) *types.AvailabilityZone {
	value, ok := zones.zones.Load(awsAccountNumber + "-" + region)
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
