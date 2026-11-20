// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	extConfig "github.com/steadybit/extension-aws/v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const callerIdentityResponse = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/test</Arn>
    <UserId>AIDATEST</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`

const describeRegionsResponse = `<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <regionInfo/>
</DescribeRegionsResponse>`

// recordUserAgents initializes the AWS access against a fake AWS endpoint, makes an STS and an EC2 call with a
// regional config, and returns the User-Agent header of every request, including the one made during initialization.
func recordUserAgents(t *testing.T, specification extConfig.Specification) []string {
	var mu sync.Mutex
	var userAgents []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
		mu.Unlock()

		// Runs on the server's goroutine, so assert (not require) to keep FailNow on the test goroutine.
		assert.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "text/xml")
		if r.Form.Get("Action") == "DescribeRegions" {
			_, _ = w.Write([]byte(describeRegionsResponse))
		} else {
			_, _ = w.Write([]byte(callerIdentityResponse))
		}
	}))
	t.Cleanup(server.Close)

	previousAccounts, previousRoot := accounts, rootAccountNumber
	t.Cleanup(func() { accounts, rootAccountNumber = previousAccounts, previousRoot })

	specification.AwsEndpointOverride = server.URL
	specification.Regions = []string{"us-east-1", "eu-central-1"}
	InitializeAwsAccess(specification, aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
	})

	// Clients are created per account and region from the configs InitializeAwsAccess prepared.
	access, err := GetAwsAccess("123456789012", "eu-central-1", nil)
	require.NoError(t, err)
	_, err = sts.NewFromConfig(access.AwsConfig).GetCallerIdentity(context.Background(), nil)
	require.NoError(t, err)
	_, err = ec2.NewFromConfig(access.AwsConfig).DescribeRegions(context.Background(), nil)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, userAgents, 3, "the call during initialization and both calls of the test")
	return userAgents
}

// The identifier must reach AWS exactly as AWS documents it, on every call of every client the extension creates.
func TestEveryAwsCallCarriesThePartnerRevenueMeasurementUserAgent(t *testing.T) {
	for _, userAgent := range recordUserAgents(t, extConfig.Specification{}) {
		require.True(t, strings.HasSuffix(userAgent, " APN_1.1/pc_egaiyzlc2fa84pb03ml3c4hnk$"), "got %q", userAgent)
		require.Equal(t, 1, strings.Count(userAgent, "APN_1.1/"), "got %q", userAgent)
		require.Contains(t, userAgent, "aws-sdk-go-v2/", "the SDK's own user agent must be kept")
	}
}

func TestThePartnerRevenueMeasurementUserAgentCanBeDisabled(t *testing.T) {
	for _, userAgent := range recordUserAgents(t, extConfig.Specification{PartnerRevenueMeasurementDisabled: true}) {
		require.NotContains(t, userAgent, "APN_1.1", "got %q", userAgent)
		require.Contains(t, userAgent, "aws-sdk-go-v2/")
	}
}

func TestAddingThePartnerRevenueMeasurementUserAgentTwiceIsHarmless(t *testing.T) {
	stack := middleware.NewStack("test", nil)

	require.NoError(t, addPartnerRevenueMeasurementUserAgent(stack))
	require.NoError(t, addPartnerRevenueMeasurementUserAgent(stack))

	require.Len(t, stack.Build.List(), 1)
}
