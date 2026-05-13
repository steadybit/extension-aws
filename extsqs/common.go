// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package extsqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

const (
	sqsIcon         = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMyA3aDE0djJIM3YxMmgxOFY3aC00VjVoNHYyaDFWMjFoLTIwVjVoMnYyem01LTRoOHYySDhWM3oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPjwvc3ZnPg=="
	queueTargetType = "com.steadybit.extension_aws.sqs.queue"
)

type SqsApi interface {
	sqs.ListQueuesAPIClient
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	ListQueueTags(ctx context.Context, params *sqs.ListQueueTagsInput, optFns ...func(*sqs.Options)) (*sqs.ListQueueTagsOutput, error)
	SetQueueAttributes(ctx context.Context, params *sqs.SetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.SetQueueAttributesOutput, error)
}

// QueueVisibilityTimeoutAttackState captures the original VisibilityTimeout so we can restore on Stop.
type QueueVisibilityTimeoutAttackState struct {
	QueueUrl                  string
	QueueName                 string
	Account                   string
	Region                    string
	DiscoveredByRole          *string
	OriginalVisibilityTimeout int32
	TargetVisibilityTimeout   int32
}
