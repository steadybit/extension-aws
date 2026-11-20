// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"context"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// partnerRevenueMeasurementUserAgent identifies the AWS API calls of this extension as made by the Steadybit
// AWS Marketplace product, so AWS Partner Revenue Measurement can attribute them. The format is fixed by AWS:
// APN_1.1/pc_<marketplace product code>$
// https://docs.aws.amazon.com/PRM/latest/aws-prm-onboarding-guide/user-agent-string.html
const partnerRevenueMeasurementUserAgent = "APN_1.1/pc_egaiyzlc2fa84pb03ml3c4hnk$"

// addPartnerRevenueMeasurementUserAgent appends the identifier to the User-Agent header of every request.
//
// The SDK's application ID setting (aws.Config.AppID, AWS_SDK_UA_APP_ID) cannot be used for this: the SDK replaces
// the "/" of the identifier with "-", which no longer matches the format AWS expects.
func addPartnerRevenueMeasurementUserAgent(stack *middleware.Stack) error {
	// Added after the SDK's own user agent middleware, so the header it builds is already there to append to.
	return stack.Build.Add(middleware.BuildMiddlewareFunc("SteadybitPartnerRevenueMeasurementUserAgent",
		func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
			if request, ok := in.Request.(*smithyhttp.Request); ok {
				const header = "User-Agent"
				if current := request.Header.Get(header); current != "" {
					request.Header.Set(header, current+" "+partnerRevenueMeasurementUserAgent)
				} else {
					request.Header.Set(header, partnerRevenueMeasurementUserAgent)
				}
			}
			return next.HandleBuild(ctx, in)
		}), middleware.After)
}
