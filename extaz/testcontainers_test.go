// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extaz

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"testing"
)

type TestContainers struct {
	LocalStackContainer *localstack.LocalStackContainer
}

func (tcs *TestContainers) Terminate(t *testing.T, ctx context.Context) {
	log.Info().Msgf("Terminating localstack container")
	localstackContainer := *tcs.LocalStackContainer
	err := localstackContainer.Terminate(ctx)
	require.NoError(t, err)
}

type WithTestContainersCase struct {
	Name string
	Test func(t *testing.T, clientEc2 *ec2.Client, clientImds *imds.Client)
}

func WithTestContainers(t *testing.T, testCases []WithTestContainersCase) {
	tcs, err := setupTestContainers(t, context.Background())
	require.NoError(t, err)
	defer tcs.Terminate(t, context.Background())

	clientEc2, err := setupEc2Client(context.Background(), tcs.LocalStackContainer)
	require.NoError(t, err)
	clientImds, err := setupImdsClient(context.Background(), tcs.LocalStackContainer)
	require.NoError(t, err)
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Test(t, clientEc2, clientImds)
		})
	}
}

func setupTestContainers(t *testing.T, ctx context.Context) (*TestContainers, error) {
	networkName := "localstack-network"
	localstackImage := "localstack/localstack:1.4"

	networkRequest := testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	}
	_, err := testcontainers.GenericNetwork(ctx, networkRequest)
	if err != nil {
		return nil, err
	}

	container, err := localstack.StartContainer(
		ctx,
		localstack.OverrideContainerRequest(testcontainers.ContainerRequest{
			Image:          localstackImage,
			Env:            map[string]string{"SERVICES": "ec2,imds"},
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"localstack"}},
		}),
	)
	require.Nil(t, err)
	assert.NotNil(t, container)

	return &TestContainers{
		LocalStackContainer: container,
	}, nil
}

func setupEc2Client(ctx context.Context, l *localstack.LocalStackContainer) (*ec2.Client, error) {
	mappedPort, err := l.MappedPort(ctx, nat.Port("4566/tcp"))
	if err != nil {
		return nil, err
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return nil, err
	}

	host, err := provider.DaemonHost(ctx)
	if err != nil {
		return nil, err
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           fmt.Sprintf("http://%s:%d", host, mappedPort.Int()),
				SigningRegion: region,
			}, nil
		})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("eu-west-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("accesskey", "secretkey", "token")),
	)
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(awsCfg, func(o *ec2.Options) {
	})

	return client, nil
}

func setupImdsClient(ctx context.Context, l *localstack.LocalStackContainer) (*imds.Client, error) {
	mappedPort, err := l.MappedPort(ctx, nat.Port("4566/tcp"))
	if err != nil {
		return nil, err
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return nil, err
	}

	host, err := provider.DaemonHost(ctx)
	if err != nil {
		return nil, err
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           fmt.Sprintf("http://%s:%d", host, mappedPort.Int()),
				SigningRegion: region,
			}, nil
		})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("eu-west-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("accesskey", "secretkey", "token")),
	)
	if err != nil {
		return nil, err
	}

	client := imds.NewFromConfig(awsCfg, func(o *imds.Options) {
	})

	return client, nil
}
