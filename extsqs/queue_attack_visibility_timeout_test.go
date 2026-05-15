// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package extsqs

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type sqsApiMock struct {
	mock.Mock
}

func (m *sqsApiMock) ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.ListQueuesOutput), args.Error(1)
}

func (m *sqsApiMock) GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.GetQueueAttributesOutput), args.Error(1)
}

func (m *sqsApiMock) ListQueueTags(ctx context.Context, params *sqs.ListQueueTagsInput, optFns ...func(*sqs.Options)) (*sqs.ListQueueTagsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.ListQueueTagsOutput), args.Error(1)
}

func (m *sqsApiMock) SetQueueAttributes(ctx context.Context, params *sqs.SetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.SetQueueAttributesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sqs.SetQueueAttributesOutput), args.Error(1)
}

func newVisibilityRequest(target int) action_kit_api.PrepareActionRequestBody {
	return extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
		Config: map[string]interface{}{"visibilityTimeoutSeconds": target},
		Target: extutil.Ptr(action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":                      {"42"},
				"aws.region":                       {"us-east-1"},
				"aws.sqs.queue.url":                {"https://sqs.us-east-1.amazonaws.com/42/test-queue"},
				"aws.sqs.queue.name":               {"test-queue"},
				"extension-aws.discovered-by-role": {"arn:role"},
			},
		}),
	})
}

func newAttack(api *sqsApiMock) queueVisibilityTimeoutAttack {
	return queueVisibilityTimeoutAttack{
		clientProvider: func(account string, region string, role *string) (SqsApi, error) { return api, nil },
	}
}

func TestPrepareCapturesOriginalVisibilityTimeout(t *testing.T) {
	api := new(sqsApiMock)
	api.On("GetQueueAttributes", mock.Anything, mock.Anything).Return(&sqs.GetQueueAttributesOutput{
		Attributes: map[string]string{string(sqstypes.QueueAttributeNameVisibilityTimeout): "60"},
	}, nil)
	a := newAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newVisibilityRequest(0))
	require.NoError(t, err)
	assert.Equal(t, int32(60), state.OriginalVisibilityTimeout)
	assert.Equal(t, int32(0), state.TargetVisibilityTimeout)
}

func TestPrepareFallsBackToSqsDefaultWhenAttributeMissing(t *testing.T) {
	api := new(sqsApiMock)
	api.On("GetQueueAttributes", mock.Anything, mock.Anything).Return(&sqs.GetQueueAttributesOutput{
		Attributes: map[string]string{},
	}, nil)
	a := newAttack(api)
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newVisibilityRequest(10))
	require.NoError(t, err)
	assert.Equal(t, int32(30), state.OriginalVisibilityTimeout, "SQS default when unset is 30s")
}

func TestStartAppliesTargetTimeout(t *testing.T) {
	api := new(sqsApiMock)
	api.On("SetQueueAttributes", mock.Anything, mock.MatchedBy(func(p *sqs.SetQueueAttributesInput) bool {
		v, ok := p.Attributes[string(sqstypes.QueueAttributeNameVisibilityTimeout)]
		require.True(t, ok)
		n, _ := strconv.Atoi(v)
		require.Equal(t, 5, n)
		return true
	})).Return(&sqs.SetQueueAttributesOutput{}, nil)
	a := newAttack(api)
	state := QueueVisibilityTimeoutAttackState{
		QueueUrl: "url", QueueName: "test-queue", Account: "42", Region: "us-east-1",
		OriginalVisibilityTimeout: 60, TargetVisibilityTimeout: 5,
	}
	_, err := a.Start(context.Background(), &state)
	require.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopRestoresOriginalTimeout(t *testing.T) {
	api := new(sqsApiMock)
	api.On("SetQueueAttributes", mock.Anything, mock.MatchedBy(func(p *sqs.SetQueueAttributesInput) bool {
		v := p.Attributes[string(sqstypes.QueueAttributeNameVisibilityTimeout)]
		require.Equal(t, "60", v)
		return true
	})).Return(&sqs.SetQueueAttributesOutput{}, nil)
	a := newAttack(api)
	state := QueueVisibilityTimeoutAttackState{
		QueueUrl: "url", QueueName: "test-queue",
		OriginalVisibilityTimeout: 60, TargetVisibilityTimeout: 5,
	}
	_, err := a.Stop(context.Background(), &state)
	require.NoError(t, err)
	api.AssertExpectations(t)
}

func TestPrepareRejectsOutOfRange(t *testing.T) {
	a := newAttack(new(sqsApiMock))
	state := a.NewEmptyState()
	_, err := a.Prepare(context.Background(), &state, newVisibilityRequest(43201))
	require.Error(t, err)
}

func TestStartForwardsError(t *testing.T) {
	api := new(sqsApiMock)
	api.On("SetQueueAttributes", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))
	a := newAttack(api)
	state := QueueVisibilityTimeoutAttackState{QueueUrl: "url", QueueName: "q", TargetVisibilityTimeout: 0}
	_, err := a.Start(context.Background(), &state)
	assert.Error(t, err)
}
