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
	extConfig "github.com/steadybit/extension-aws/v2/config"
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

// The identifier must reach AWS exactly as AWS documents it, on every call of every client the extension creates.
func TestEveryAwsCallCarriesThePartnerRevenueMeasurementUserAgent(t *testing.T) {
	var mu sync.Mutex
	var userAgents []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgents = append(userAgents, r.Header.Get("User-Agent"))
		mu.Unlock()

		require.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "text/xml")
		if r.Form.Get("Action") == "DescribeRegions" {
			_, _ = w.Write([]byte(describeRegionsResponse))
		} else {
			_, _ = w.Write([]byte(callerIdentityResponse))
		}
	}))
	defer server.Close()

	previousAccounts, previousRoot := accounts, rootAccountNumber
	defer func() { accounts, rootAccountNumber = previousAccounts, previousRoot }()

	InitializeAwsAccess(extConfig.Specification{
		AwsEndpointOverride: server.URL,
		Regions:             []string{"us-east-1", "eu-central-1"},
	}, aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
	})

	// Clients are created per account and region from the config InitializeAwsAccess prepared.
	access, err := GetAwsAccess("123456789012", "eu-central-1", nil)
	require.NoError(t, err)
	_, err = sts.NewFromConfig(access.AwsConfig).GetCallerIdentity(context.Background(), nil)
	require.NoError(t, err)
	_, err = ec2.NewFromConfig(access.AwsConfig).DescribeRegions(context.Background(), nil)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, userAgents, 3, "the call during initialization and both calls of the test")
	for _, userAgent := range userAgents {
		require.True(t, strings.HasSuffix(userAgent, " APN_1.1/pc_egaiyzlc2fa84pb03ml3c4hnk$"), "got %q", userAgent)
		require.Equal(t, 1, strings.Count(userAgent, "APN_1.1/"), "got %q", userAgent)
		require.Contains(t, userAgent, "aws-sdk-go-v2/", "the SDK's own user agent must be kept")
	}
}

// Guards the reason for the custom middleware: if the SDK ever stops rewriting the identifier, the application ID
// setting becomes the simpler way to send it.
func TestTheSdkApplicationIdSettingStillManglesTheIdentifier(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(callerIdentityResponse))
	}))
	defer server.Close()

	_, err := sts.NewFromConfig(aws.Config{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
		BaseEndpoint: aws.String(server.URL),
		AppID:        partnerRevenueMeasurementUserAgent,
	}).GetCallerIdentity(context.Background(), nil)

	require.NoError(t, err)
	require.NotContains(t, userAgent, partnerRevenueMeasurementUserAgent)
	require.Contains(t, userAgent, "app/APN_1.1-pc_egaiyzlc2fa84pb03ml3c4hnk$")
}
