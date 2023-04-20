// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extfis

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/fis/types"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"testing"
)

type fisClientMock struct {
	mock.Mock
}

func (m *fisClientMock) ListExperimentTemplates(ctx context.Context, params *fis.ListExperimentTemplatesInput, _ ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fis.ListExperimentTemplatesOutput), args.Error(1)
}
func (m *fisClientMock) GetExperimentTemplate(ctx context.Context, params *fis.GetExperimentTemplateInput, _ ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fis.GetExperimentTemplateOutput), args.Error(1)
}

func TestGetAllFisTemplates(t *testing.T) {
	// Given
	mockedApi := new(fisClientMock)
	mockedReturnValue := fis.ListExperimentTemplatesOutput{
		ExperimentTemplates: []types.ExperimentTemplateSummary{
			{
				Id:          extutil.Ptr("EXTT9pS4WzTB4qe"),
				Description: extutil.Ptr("Lorem Ipsum"),
				Tags:        map[string]string{"Name": "FISible Experiment", "SpecialTag": "Great Thing"},
			},
		},
	}
	mockedReturnValueTemplate := fis.GetExperimentTemplateOutput{
		ExperimentTemplate: &types.ExperimentTemplate{
			Id: extutil.Ptr("EXTT9pS4WzTB4qe"),
			Actions: map[string]types.ExperimentTemplateAction{
				"step1":       {Parameters: map[string]string{"duration": "PT1M"}},
				"step2":       {Parameters: map[string]string{"duration": "PT2M"}, StartAfter: []string{"step1"}},
				"step3":       {Parameters: map[string]string{"duration": "PT3M"}, StartAfter: []string{"step2"}},
				"step4":       {Parameters: map[string]string{"duration": "PT3M"}, StartAfter: []string{"step2", "step3", "anotherStep"}},
				"anotherStep": {Parameters: map[string]string{"duration": "PT13M"}},
			},
		},
	}
	mockedApi.On("ListExperimentTemplates", mock.Anything, mock.Anything).Return(&mockedReturnValue, nil)
	mockedApi.On("GetExperimentTemplate", mock.Anything, mock.Anything).Return(&mockedReturnValueTemplate, nil)

	// When
	targets, err := GetAllFisTemplates(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(targets))

	target := targets[0]
	assert.Equal(t, "EXTT9pS4WzTB4qe", target.Id)
	assert.Equal(t, fisTargetId, target.TargetType)
	assert.Equal(t, "FISible Experiment", target.Label)
	assert.Equal(t, []string{"42"}, target.Attributes["aws.account"])
	assert.Equal(t, []string{"us-east-1"}, target.Attributes["aws.region"])
	assert.Equal(t, []string{"EXTT9pS4WzTB4qe"}, target.Attributes["aws.fis.experiment.template.id"])
	assert.Equal(t, []string{"FISible Experiment"}, target.Attributes["aws.fis.experiment.template.name"])
	assert.Equal(t, []string{"Lorem Ipsum"}, target.Attributes["aws.fis.experiment.template.description"])
	assert.Equal(t, []string{"16m0s"}, target.Attributes["aws.fis.experiment.template.duration"])
	assert.Equal(t, []string{"Great Thing"}, target.Attributes["label.specialtag"])
	_, present := target.Attributes["label.name"]
	assert.False(t, present)
}

func TestGetAllFisTemplatesError(t *testing.T) {
	// Given
	mockedApi := new(fisClientMock)

	mockedApi.On("ListExperimentTemplates", mock.Anything, mock.Anything).Return(nil, errors.New("expected"))

	// When
	_, err := GetAllFisTemplates(context.Background(), mockedApi, "42", "us-east-1")

	// Then
	assert.Equal(t, err.Error(), "expected")
}
