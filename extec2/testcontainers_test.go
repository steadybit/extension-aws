// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extec2

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	aws_middleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go/middleware"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/network"
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
	tcs := setupTestContainers(t, context.Background())
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

func setupTestContainers(t *testing.T, ctx context.Context) *TestContainers {
	localstackImage := "localstack/localstack:3.4.0"

	networkCreated, err := network.New(ctx)
	require.Nil(t, err)
	networkName := networkCreated.Name

	container, err := localstack.Run(
		ctx,
		"localstack/localstack:1.4.0",
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:          localstackImage,
				Env:            map[string]string{"SERVICES": "ec2,imds"},
				Networks:       []string{networkName},
				NetworkAliases: map[string][]string{networkName: {"localstack"}},
			},
		}),
	)
	require.Nil(t, err)
	assert.NotNil(t, container)

	return &TestContainers{
		LocalStackContainer: container,
	}
}

// localstack does not implement api throttling, so we need to simulate it. PermittedApiCalls is a map that contains the number of permitted calls for each API call and can also be used to count the actual calls in a specific test.
var PermittedApiCalls = map[string]int{
	"CreateNetworkAcl":             1000,
	"CreateNetworkAclEntry":        1000,
	"DeleteNetworkAcl":             1000,
	"DescribeNetworkAcls":          1000,
	"DescribeSubnets":              1000,
	"ReplaceNetworkAclAssociation": 1000,
}

var apiThrottlingMiddleware = middleware.InitializeMiddlewareFunc("apiThrottlingMiddleware",
	func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (out middleware.InitializeOutput, metadata middleware.Metadata, err error) {
		operationName := aws_middleware.GetOperationName(ctx)
		permitted, ok := PermittedApiCalls[operationName]
		if !ok {
			return out, metadata, fmt.Errorf("API call not permitted: %s", operationName)
		}
		if permitted == 0 {
			return out, metadata, fmt.Errorf("simulated API Throttling for %s", operationName)
		}
		PermittedApiCalls[operationName]--
		return next.HandleInitialize(ctx, in)
	})

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

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("eu-west-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("accesskey", "secretkey", "token")),
	)

	awsCfg.APIOptions = append(awsCfg.APIOptions, func(stack *middleware.Stack) error {
		return stack.Initialize.Add(apiThrottlingMiddleware, middleware.After)
	})
	if err != nil {
		return nil, err
	}

	awsCfg.BaseEndpoint = aws.String(fmt.Sprintf("http://%s:%d", host, mappedPort.Int()))

	return ec2.NewFromConfig(awsCfg), nil
}

func setupImdsClient(ctx context.Context, l *localstack.LocalStackContainer) (*imds.Client, error) {
	mappedPort, err := l.MappedPort(ctx, "4566/tcp")
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

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("eu-west-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("accesskey", "secretkey", "token")),
	)
	awsCfg.BaseEndpoint = aws.String(fmt.Sprintf("http://%s:%d", host, mappedPort.Int()))
	if err != nil {
		return nil, err
	}

	client := imds.NewFromConfig(awsCfg, func(o *imds.Options) {
	})

	return client, nil
}
