package config

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestTranslateToAssumeRolesAdvanced(t *testing.T) {
	tests := []struct {
		name           string
		initialConfig  Specification
		expectedConfig Specification
	}{
		{
			name: "Populate empty TagFilters and Regions in AssumeRolesAdvanced",
			initialConfig: Specification{
				AssumeRolesAdvanced: AssumeRoles{
					{RoleArn: "arn:aws:iam::123456789012:role/test"},
				},
				Regions:    []string{"us-east-1"},
				TagFilters: TagFilters{{Key: "env", Values: []string{"prod"}}},
			},
			expectedConfig: Specification{
				AssumeRolesAdvanced: AssumeRoles{
					{
						RoleArn:    "arn:aws:iam::123456789012:role/test",
						Regions:    []string{"us-east-1"},
						TagFilters: TagFilters{{Key: "env", Values: []string{"prod"}}},
					},
				},
				Regions:    []string{"us-east-1"},
				TagFilters: TagFilters{{Key: "env", Values: []string{"prod"}}},
			},
		},
		{
			name: "Convert AssumeRoles to AssumeRolesAdvanced",
			initialConfig: Specification{
				AssumeRoles: []string{"arn:aws:iam::123456789012:role/test"},
				Regions:     []string{"us-west-2"},
				TagFilters:  TagFilters{{Key: "team", Values: []string{"dev"}}},
			},
			expectedConfig: Specification{
				AssumeRoles: []string{"arn:aws:iam::123456789012:role/test"},
				AssumeRolesAdvanced: AssumeRoles{
					{
						RoleArn:    "arn:aws:iam::123456789012:role/test",
						Regions:    []string{"us-west-2"},
						TagFilters: TagFilters{{Key: "team", Values: []string{"dev"}}},
					},
				},
				Regions:    []string{"us-west-2"},
				TagFilters: TagFilters{{Key: "team", Values: []string{"dev"}}},
			},
		},
		{
			name: "Do not override non-empty AssumeRolesAdvanced values",
			initialConfig: Specification{
				AssumeRolesAdvanced: AssumeRoles{
					{
						RoleArn:    "arn:aws:iam::123456789012:role/test",
						Regions:    []string{"us-east-2"},
						TagFilters: TagFilters{{Key: "custom", Values: []string{"yes"}}},
					},
				},
				Regions:    []string{"us-west-1"},
				TagFilters: TagFilters{{Key: "general", Values: []string{"true"}}},
			},
			expectedConfig: Specification{
				AssumeRolesAdvanced: AssumeRoles{
					{
						RoleArn:    "arn:aws:iam::123456789012:role/test",
						Regions:    []string{"us-east-2"},                                // Should not be overridden
						TagFilters: TagFilters{{Key: "custom", Values: []string{"yes"}}}, // Should not be overridden
					},
				},
				Regions:    []string{"us-west-1"},
				TagFilters: TagFilters{{Key: "general", Values: []string{"true"}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global Config variable
			Config = tt.initialConfig

			// Call the function under test
			translateToAssumeRolesAdvanced()

			// Verify the results
			if !reflect.DeepEqual(Config, tt.expectedConfig) {
				t.Errorf("translateToAssumeRolesAdvanced() failed for %s.\nExpected: %+v\nGot: %+v",
					tt.name, tt.expectedConfig, Config)
			}
		})
	}
}

func TestVerifyAssumeRolesAdvanced(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		Config.AssumeRolesAdvanced = []AssumeRole{
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole1", Regions: []string{"us-east-1"}, TagFilters: TagFilters{{Key: "team", Values: []string{"foo"}}}},
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole2", Regions: []string{"us-east-1"}, TagFilters: TagFilters{{Key: "team", Values: []string{"bar"}}}},
			{RoleArn: "arn:aws:iam::234567890123:role/TestRole2", Regions: []string{"us-west-2"}},
		}
		err := verifyAssumeRolesAdvanced()
		assert.Nil(t, err)
	})

	t.Run("duplicate account without tag filters (should fail)", func(t *testing.T) {
		Config.AssumeRolesAdvanced = []AssumeRole{
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole1", Regions: []string{"us-east-1"}},
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole1", Regions: []string{"eu-central-1"}},
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole2", Regions: []string{"us-east-1"}},
		}
		err := verifyAssumeRolesAdvanced()
		assert.EqualError(t, err, "you have configured multiple role-arn for the same account '123456789012'. you need to set up tag filters to separate the discovered targets by each role")
	})

	t.Run("duplicate role in same region (should fail)", func(t *testing.T) {
		Config.AssumeRolesAdvanced = []AssumeRole{
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole1", Regions: []string{"us-east-1"}},
			{RoleArn: "arn:aws:iam::123456789999:role/TestRole1", Regions: []string{"us-east-1"}},
			{RoleArn: "arn:aws:iam::123456789012:role/TestRole1", Regions: []string{"us-east-1"}},
		}
		err := verifyAssumeRolesAdvanced()
		assert.EqualError(t, err, "you have configured the same role-arn for the same region twice. (arn: 'arn:aws:iam::123456789012:role/TestRole1', region: 'us-east-1')")
	})
}

func Test_getAccountNumberFromArn(t *testing.T) {
	type args struct {
		arn string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Regular ARN",
			args: args{arn: "arn:aws:iam::123456789012:role/TestRole"},
			want: "123456789012",
		},
		{
			name: "GovCloud ARN",
			args: args{arn: "arn:aws-us-gov:iam::123456789012:role/TestRole"},
			want: "123456789012",
		},
		{
			name: "Invalid ARN",
			args: args{arn: "arn:this-is-not-a-valid-arn"},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getAccountNumberFromArn(tt.args.arn), "getAccountNumberFromArn(%v)", tt.args.arn)
		})
	}
}
