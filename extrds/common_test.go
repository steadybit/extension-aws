package extrds

import (
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/steadybit/extension-aws/config"
	"github.com/steadybit/extension-kit/extutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesTagFilter(t *testing.T) {
	tests := []struct {
		name    string
		tags    []types.Tag
		filters []config.TagFilter
		want    bool
	}{
		{
			name: "No filters - should match",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("prod")},
				{Key: extutil.Ptr("team"), Value: extutil.Ptr("devops")},
			},
			filters: []config.TagFilter{},
			want:    true,
		},
		{
			name: "Single filter - matching key and value",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("prod")},
				{Key: extutil.Ptr("team"), Value: extutil.Ptr("devops")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
			},
			want: true,
		},
		{
			name: "Single filter - matching key but no matching value",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("staging")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
			},
			want: false,
		},
		{
			name: "Multiple filters - all match",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("prod")},
				{Key: extutil.Ptr("team"), Value: extutil.Ptr("devops")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
				{Key: "team", Values: []string{"devops"}},
			},
			want: true,
		},
		{
			name: "Multiple filters - one does not match",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("prod")},
				{Key: extutil.Ptr("team"), Value: extutil.Ptr("engineering")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
				{Key: "team", Values: []string{"devops"}},
			},
			want: false,
		},
		{
			name: "Filter with multiple values - one matches",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("staging")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"dev", "staging", "prod"}},
			},
			want: true,
		},
		{
			name: "Filter with multiple values - none match",
			tags: []types.Tag{
				{Key: extutil.Ptr("env"), Value: extutil.Ptr("test")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"dev", "staging", "prod"}},
			},
			want: false,
		},
		{
			name: "Filter key not present in tags",
			tags: []types.Tag{
				{Key: extutil.Ptr("region"), Value: extutil.Ptr("us-east-1")},
			},
			filters: []config.TagFilter{
				{Key: "env", Values: []string{"prod"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesTagFilter(tt.tags, tt.filters)
			assert.Equal(t, tt.want, got)
		})
	}
}
