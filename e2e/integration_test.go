// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package e2e

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	actValidate "github.com/steadybit/action-kit/go/action_kit_test/validate"
	disValidate "github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"strings"
	"testing"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-aws",
		Port: 8085,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=INFO",
				"--set", "extraEnv[0].name=STEADYBIT_EXTENSION_AWS_ENDPOINT_OVERRIDE",
				"--set", "extraEnv[0].value=http://localstack.default.svc.cluster.local:4566",
				"--set", "extraEnv[1].name=AWS_DEFAULT_REGION",
				"--set", "extraEnv[1].value=us-east-1",
				"--set", "extraEnv[2].name=AWS_ACCESS_KEY_ID",
				"--set", "extraEnv[2].value=test",
				"--set", "extraEnv[3].name=AWS_SECRET_ACCESS_KEY",
				"--set", "extraEnv[3].value=test",
				"--set", "aws.discovery.disabled.ecs=false", //disabled by default
				"--set", "aws.discovery.disabled.elb=false", //disabled by default
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts().AfterStart(helmInstallLocalStack), &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
		},
		{
			Name: "validate actions",
			Test: validateActions,
		},
	})
}

func helmInstallLocalStack(minikube *e2e.Minikube) error {
	out, err := exec.Command("helm", "repo", "add", "localstack", "https://localstack.github.io/helm-charts").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install helm chart: %s: %s", err, out)
	}
	out, err = exec.Command("helm",
		"upgrade", "--install",
		"--kube-context", minikube.Profile,
		"--set", "debug=true",
		"--set", "startServices=lambda\\,ec2",
		"--namespace=default",
		"localstack", "localstack/localstack", "--wait").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install helm chart: %s: %s", err, out)
	}
	return nil
}

func validateDiscovery(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	err := disValidate.ValidateEndpointReferences("/", e.Client)
	if err == nil {
		return
	}

	isIgnored := func(s string) bool {
		ignorable := []string{
			//localstack OSS does not support these services, so we ignore the errors from failed discoveries
			"GET /com.steadybit.extension_aws.rds.instance/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.rds.cluster/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.fis-experiment-template/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.ecs-service/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.alb/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.ecs-task/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.elasticache.node-group/discovery/discovered-targets",
			"GET /com.steadybit.extension_aws.msk.cluster.broker/discovery/discovered-targets",
		}
		for _, i := range ignorable {
			if strings.Contains(err.Error(), i) {
				return true
			}
		}
		return false
	}

	errs := err.(interface{ Unwrap() []error }).Unwrap()
	for _, err = range errs {
		if !isIgnored(err.Error()) {
			assert.Fail(t, fmt.Sprintf("Received unexpected error:\n%+v", err))
		}
	}
}

func validateActions(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	assert.NoError(t, actValidate.ValidateEndpointReferences("/", e.Client))
}
