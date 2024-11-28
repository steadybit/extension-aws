package utils

import (
	"context"
	middleware2 "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
	"github.com/rs/zerolog/log"
	"strings"
)

type logForwarder struct {
}

func (logger logForwarder) Logf(classification logging.Classification, format string, v ...interface{}) {
	switch classification {
	case logging.Debug:
		log.Trace().Msgf(format, v...)
	case logging.Warn:
		log.Warn().Msgf(format, v...)
	}
}

var customLoggerMiddleware = middleware.InitializeMiddlewareFunc("customLoggerMiddleware",
	func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (out middleware.InitializeOutput, metadata middleware.Metadata, err error) {
		operationName := middleware2.GetOperationName(ctx)
		if strings.HasPrefix(operationName, "List") ||
			strings.HasPrefix(operationName, "Describe") ||
			strings.HasPrefix(operationName, "Assume") ||
			strings.HasPrefix(operationName, "Get") {
			log.Trace().Msgf("AWS-Call: %s - %s - %s", middleware2.GetRegion(ctx), middleware2.GetServiceID(ctx), operationName)
		} else {
			log.Info().Msgf("AWS-Call: %s - %s - %s", middleware2.GetRegion(ctx), middleware2.GetServiceID(ctx), operationName)
		}
		return next.HandleInitialize(ctx, in)
	})
