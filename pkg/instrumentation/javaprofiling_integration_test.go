// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/yaml"

	"github.com/aws/amazon-cloudwatch-agent-operator/integration-tests/java-profiling/validate"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
)

// The Java profiling integration case (integration-tests/java-profiling, run in minikube by
// operator-integration-test.yml) checked here against pods the pod mutator renders in envtest: the same
// deployments, operator config and expectations files, and the same validator.
const javaProfilingITDir = "../../integration-tests/java-profiling"

// setJavaProfilingFromITConfig sets what main.go derives from the case's --auto-instrumentation-config, and resets
// the once-parsed settings so that this config is the one injected.
func setJavaProfilingFromITConfig(t *testing.T, enabled bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(javaProfilingITDir, "auto-instrumentation-config.json"))
	require.NoError(t, err)
	var cfg map[string]map[string]map[string]string
	require.NoError(t, json.Unmarshal(data, &cfg))
	settings := map[string]string{}
	for key, value := range cfg["java"]["profiling"] {
		if key != "enabled" {
			settings[key] = value
		}
	}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	if enabled {
		t.Setenv(envJavaProfilingEnabled, cfg["java"]["profiling"]["enabled"])
	} else {
		t.Setenv(envJavaProfilingEnabled, "")
	}
	t.Setenv(envJavaProfilingSettings, string(raw))
	// The default Instrumentation needs the images the deployed operator gets from its environment.
	for _, name := range []string{"AUTO_INSTRUMENTATION_JAVA", "AUTO_INSTRUMENTATION_PYTHON", "AUTO_INSTRUMENTATION_DOTNET", "AUTO_INSTRUMENTATION_NODEJS"} {
		t.Setenv(name, "it-image")
	}
	javaProfilingConfigOnce = sync.Once{}
	t.Cleanup(func() { javaProfilingConfigOnce = sync.Once{} })
}

func renderITPod(t *testing.T, deploymentFile string, ns corev1.Namespace) corev1.Pod {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(javaProfilingITDir, deploymentFile))
	require.NoError(t, err)
	var deployment appsv1.Deployment
	require.NoError(t, yaml.Unmarshal(data, &deployment))
	pod := corev1.Pod{ObjectMeta: deployment.Spec.Template.ObjectMeta, Spec: deployment.Spec.Template.Spec}
	pod.Name, pod.Namespace = deployment.Name+"-pod", ns.Name
	mutated, err := NewMutator(logr.Discard(), k8sClient, record.NewFakeRecorder(100)).Mutate(context.Background(), ns, pod)
	require.NoError(t, err)
	return mutated
}

func loadITExpectations(t *testing.T, file string) validate.Expectations {
	t.Helper()
	exp, err := validate.Load(filepath.Join(javaProfilingITDir, file))
	require.NoError(t, err)
	return exp
}

func TestJavaProfilingIntegrationCase(t *testing.T) {
	// The deployed operator runs with multi-instrumentation on (config/manager), which java-container-names needs;
	// other tests in this package may leave the gate off.
	for _, gate := range []*colfeaturegate.Gate{featuregate.EnableMultiInstrumentationSupport, featuregate.SkipMultiInstrumentationContainerValidation} {
		was := gate.IsEnabled()
		require.NoError(t, colfeaturegate.GlobalRegistry().Set(gate.ID(), true))
		t.Cleanup(func() { require.NoError(t, colfeaturegate.GlobalRegistry().Set(gate.ID(), was)) })
	}
	// Mutate uses this object as given; the namespace need not exist (envtest cannot delete one).
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "java-profiling-it"}}
	profiled := loadITExpectations(t, "java_profiling_expectations.json")
	optOut := loadITExpectations(t, "java_profiling_opt_out_expectations.json")

	t.Run("profiled pod matches", func(t *testing.T) {
		setJavaProfilingFromITConfig(t, true)
		assert.Empty(t, validate.Pod(renderITPod(t, "sample-deployment-java-profiling.yaml", ns), profiled))
	})
	t.Run("opt-out pod matches", func(t *testing.T) {
		setJavaProfilingFromITConfig(t, true)
		assert.Empty(t, validate.Pod(renderITPod(t, "sample-deployment-java-profiling-opt-out.yaml", ns), optOut))
	})
	// Controls: the case must fail when the operator does not inject, and the opt-out expectations must fail
	// on a profiled pod, so neither check can pass vacuously.
	t.Run("profiling off fails the profiled expectations", func(t *testing.T) {
		setJavaProfilingFromITConfig(t, false)
		pod := renderITPod(t, "sample-deployment-java-profiling.yaml", ns)
		problems := strings.Join(validate.Pod(pod, profiled), "\n")
		assert.Contains(t, problems, "OTEL_AWS_PROFILER_ENABLED is not set")
		assert.Contains(t, problems, "want exactly one volume "+validate.ProfilerVolumeName)
		assert.Empty(t, validate.Pod(pod, validate.Expectations{JavaInstrumentedContainers: []string{"app", "worker"}}),
			"the pod is still Java-instrumented, so only the profiler checks fail")
	})
	t.Run("a profiled pod fails the opt-out expectations", func(t *testing.T) {
		setJavaProfilingFromITConfig(t, true)
		assert.NotEmpty(t, validate.Pod(renderITPod(t, "sample-deployment-java-profiling.yaml", ns), optOut))
	})
}
