package extecs

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestServiceEventLog_Lifecycle(t *testing.T) {
	const account = "awsAccount"
	const cluster = "cluster"
	const service = "service"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pollerMock := new(ServiceDescriptionPollerMock)
	pollerMock.On("Register", account, cluster, service)
	pollerMock.On("Unregister", account, cluster, service)
	pollerMock.On("Latest", account, cluster, service).Return(nil, nil)
	action := EcsServiceEventLogAction{
		poller: pollerMock,
	}
	state := &EcsServiceEventLogState{}
	request := action_kit_api.PrepareActionRequestBody{
		Target: &action_kit_api.Target{
			Attributes: map[string][]string{
				"aws.account":         {account},
				"aws-ecs.cluster.arn": {cluster},
				"aws-ecs.service.arn": {service},
			},
		},
	}

	prepare, err := action.Prepare(ctx, state, request)
	assert.NoError(t, err)
	assert.Nil(t, prepare)
	assert.Equal(t, state.AwsAccount, account)
	assert.Equal(t, state.ClusterArn, cluster)
	assert.Equal(t, state.ServiceArn, service)
	pollerMock.AssertCalled(t, "Register", account, cluster, service)

	start, err := action.Start(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, start)

	stop, err := action.Stop(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, stop)
	pollerMock.AssertCalled(t, "Unregister", account, cluster, service)
}

func TestServiceEventLog_Status(t *testing.T) {
	now := time.Now()
	before := now.Add(-1 * time.Millisecond)
	after := now.Add(1 * time.Millisecond)
	moreAfter := now.Add(1 * time.Millisecond)

	tests := []struct {
		name      string
		responses []*PollService
		state     EcsServiceEventLogState
		mode      string
		wanted    func(t *testing.T, result *action_kit_api.StatusResult, invocation int)
	}{
		{
			name:      "no event",
			responses: []*PollService{},
			state: EcsServiceEventLogState{
				LatestEventTimestamp: now,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult, invocation int) {
				assert.Empty(t, result.Messages)
			},
		},
		{
			name: "no new event",
			responses: []*PollService{
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &before,
							},
						},
					},
				},
			},
			state: EcsServiceEventLogState{
				LatestEventTimestamp: now,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult, invocation int) {
				assert.Empty(t, result.Messages)
			},
		},
		{
			name: "one new event",
			responses: []*PollService{
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &after,
								Id:        extutil.Ptr("Id"),
								Message:   extutil.Ptr("Message"),
							},
						},
					},
				},
			},
			state: EcsServiceEventLogState{
				LatestEventTimestamp: now,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult, invocation int) {
				assert.Len(t, *result.Messages, 1)
				assert.Contains(t, *result.Messages, action_kit_api.Message{
					Message:         "Message",
					Timestamp:       &after,
					TimestampSource: extutil.Ptr(action_kit_api.TimestampSourceExternal),
					Type:            extutil.Ptr(LogType),
					Fields: extutil.Ptr(action_kit_api.MessageFields{
						"Id": "Id",
					}),
				})
			},
		},
		{
			name: "one new event after an old one",
			responses: []*PollService{
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &now,
								Id:        extutil.Ptr("Old"),
								Message:   extutil.Ptr("Message1"),
							},
						},
					},
				},
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &after,
								Id:        extutil.Ptr("New"),
								Message:   extutil.Ptr("Message2"),
							},
						},
					},
				},
			},
			state: EcsServiceEventLogState{
				LatestEventTimestamp: now,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult, invocation int) {
				if invocation == 0 {
					assert.Len(t, *result.Messages, 0)
				} else {
					assert.Len(t, *result.Messages, 1)
					assert.Contains(t, *result.Messages, action_kit_api.Message{
						Message:         "Message2",
						Timestamp:       &after,
						TimestampSource: extutil.Ptr(action_kit_api.TimestampSourceExternal),
						Type:            extutil.Ptr(LogType),
						Fields: extutil.Ptr(action_kit_api.MessageFields{
							"Id": "New",
						}),
					})
				}
			},
		},
		{
			name: "one new event after the other",
			responses: []*PollService{
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &after,
								Id:        extutil.Ptr("New1"),
								Message:   extutil.Ptr("Message1"),
							},
						},
					},
				},
				{
					service: &types.Service{
						Events: []types.ServiceEvent{
							{
								CreatedAt: &moreAfter,
								Id:        extutil.Ptr("New2"),
								Message:   extutil.Ptr("Message2"),
							},
						},
					},
				},
			},
			state: EcsServiceEventLogState{
				LatestEventTimestamp: now,
			},
			wanted: func(t *testing.T, result *action_kit_api.StatusResult, invocation int) {
				if invocation == 0 {
					assert.Len(t, *result.Messages, 1)
					assert.Contains(t, *result.Messages, action_kit_api.Message{
						Message:         "Message1",
						Timestamp:       &after,
						TimestampSource: extutil.Ptr(action_kit_api.TimestampSourceExternal),
						Type:            extutil.Ptr(LogType),
						Fields: extutil.Ptr(action_kit_api.MessageFields{
							"Id": "New1",
						}),
					})
				} else {
					assert.Len(t, *result.Messages, 1)
					assert.Contains(t, *result.Messages, action_kit_api.Message{
						Message:         "Message2",
						Timestamp:       &after,
						TimestampSource: extutil.Ptr(action_kit_api.TimestampSourceExternal),
						Type:            extutil.Ptr(LogType),
						Fields: extutil.Ptr(action_kit_api.MessageFields{
							"Id": "New2",
						}),
					})
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := EcsServiceEventLogAction{}
			if len(test.responses) == 0 {
				pollerMock := new(ServiceDescriptionPollerMock)
				pollerMock.On("Latest", test.state.AwsAccount, test.state.ClusterArn, test.state.ServiceArn).Return(nil, nil)
				action.poller = pollerMock
				runWithPoller(t, action, test.state, test.wanted, 0)
			}
			for i := range test.responses {
				// Setting different return values for multiple calls with the same parameters does not seem to work.
				// This little workaround sets a new poller mock for every response, resulting in the same behavior.
				pollerMock := new(ServiceDescriptionPollerMock)
				pollerMock.On("Latest", test.state.AwsAccount, test.state.ClusterArn, test.state.ServiceArn).Return(test.responses[i], nil)
				action.poller = pollerMock
				runWithPoller(t, action, test.state, test.wanted, i)
			}
		})
	}
}

func runWithPoller(t *testing.T, action EcsServiceEventLogAction, state EcsServiceEventLogState, wanted func(t *testing.T, result *action_kit_api.StatusResult, invocation int), i int) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())

	// When
	result, err := action.Status(ctx, &state)
	cancel()

	// Then
	assert.NoError(t, err)
	wanted(t, result, i)
}
