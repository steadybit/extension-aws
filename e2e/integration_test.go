// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-aws",
		Port: 8085,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=trace",
				"--set", "extraEnv[0].name=STEADYBIT_EXTENSION_AWS_ENDPOINT_OVERRIDE",
				"--set", "extraEnv[0].value=http://localstack.default.svc.cluster.local:4566",
				"--set", "extraEnv[1].name=AWS_DEFAULT_REGION",
				"--set", "extraEnv[1].value=us-east-1",
				"--set", "extraEnv[2].name=AWS_ACCESS_KEY_ID",
				"--set", "extraEnv[2].value=test",
				"--set", "extraEnv[3].name=AWS_SECRET_ACCESS_KEY",
				"--set", "extraEnv[3].value=test",
				"--set", "extraEnv[4].name=STEADYBIT_EXTENSION_DISCOVERY_DISABLED_RDS", //missing localstack support
				"--set-string", "extraEnv[4].value=true",
				"--set", "extraEnv[5].name=STEADYBIT_EXTENSION_DISCOVERY_DISABLED_FIS", //missing localstack support
				"--set-string", "extraEnv[5].value=true",
				"--set", "extraEnv[6].name=STEADYBIT_EXTENSION_DISCOVERY_DISABLED_EC2", //server 500 from localstack
				"--set-string", "extraEnv[6].value=true",
				"--set", "extraEnv[7].name=STEADYBIT_EXTENSION_DISCOVERY_DISABLED_ZONE", //server 500 from localstack
				"--set-string", "extraEnv[7].value=true",
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts().AfterStart(helmInstallLocalStack), &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
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

func validateDiscovery(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	assert.NoError(t, validate.ValidateEndpointReferences("/", e.Client))
}
