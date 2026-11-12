// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	colfeaturegate "go.opentelemetry.io/collector/featuregate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/featuregate"
)

const chartProfilesEndpoint = "http://cloudwatch-agent.observability:14318/v1development/profiles"

// setJavaProfiling sets the operator environment main.go derives from --auto-instrumentation-config java.profiling:
// enabled "true" and the other keys as JSON.
func setJavaProfiling(t *testing.T, settings map[string]string) {
	t.Helper()
	for _, gate := range []*colfeaturegate.Gate{featuregate.EnableMultiInstrumentationSupport, featuregate.SkipMultiInstrumentationContainerValidation} {
		was := gate.IsEnabled()
		require.NoError(t, colfeaturegate.GlobalRegistry().Set(gate.ID(), true))
		t.Cleanup(func() { require.NoError(t, colfeaturegate.GlobalRegistry().Set(gate.ID(), was)) })
	}
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	t.Setenv(envJavaProfilingEnabled, "true")
	t.Setenv(envJavaProfilingSettings, string(raw))
	for _, name := range []string{"AUTO_INSTRUMENTATION_JAVA", "AUTO_INSTRUMENTATION_PYTHON", "AUTO_INSTRUMENTATION_DOTNET", "AUTO_INSTRUMENTATION_NODEJS"} {
		t.Setenv(name, "it-image")
	}
	javaProfilingConfigOnce = sync.Once{}
	t.Cleanup(func() { javaProfilingConfigOnce = sync.Once{} })
}

// mutateJavaPod runs a Java-annotated pod with the given container through the pod mutator (envtest) and returns the
// mutated container.
func mutateJavaPod(t *testing.T, container corev1.Container) corev1.Container {
	return mutateJavaPodWithVolumes(t, container, nil).Spec.Containers[0]
}

// mutateJavaPodWithVolumes is mutateJavaPod with pod volumes, returning the whole mutated pod.
func mutateJavaPodWithVolumes(t *testing.T, container corev1.Container, volumes []corev1.Volume) corev1.Pod {
	t.Helper()
	ctx := context.Background()
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "java-profiling-chart-endpoint"}}
	if err := k8sClient.Create(ctx, &ns); err != nil && !apierrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "java-pod", Namespace: ns.Name,
			Annotations: map[string]string{"instrumentation.opentelemetry.io/inject-java": "true"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{container}, Volumes: volumes},
	}
	mutated, err := NewMutator(logr.Discard(), k8sClient, record.NewFakeRecorder(100)).Mutate(ctx, ns, pod)
	require.NoError(t, err)
	require.NotEmpty(t, getEnvValue(mutated.Spec.Containers[0].Env, envJavaToolsOptions), "the javaagent must be injected")
	return mutated
}

// The profiler is pointed at the endpoint the chart passes, with the chart's payload cap.
func TestJavaProfilingUsesChartEndpoint(t *testing.T) {
	setJavaProfiling(t, map[string]string{"endpoint": chartProfilesEndpoint, "max_payload_bytes": "20971520"})
	c := mutateJavaPod(t, corev1.Container{Name: "app", Image: "app"})
	assert.Equal(t, "true", getEnvValue(c.Env, envProfilerEnabled))
	assert.Equal(t, chartProfilesEndpoint, getEnvValue(c.Env, envProfilerEndpoint))
	assert.Equal(t, "20971520", getEnvValue(c.Env, "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES"))
}

// Without an endpoint from the chart there is no receiver to send to, so nothing is injected (no fallback).
func TestJavaProfilingWithoutEndpointInjectsNothing(t *testing.T) {
	setJavaProfiling(t, map[string]string{"max_payload_bytes": "20971520"})
	c := mutateJavaPod(t, corev1.Container{Name: "app", Image: "app"})
	for _, env := range c.Env {
		assert.NotContains(t, env.Name, "OTEL_AWS_PROFILER_", "no profiler variable without an endpoint")
	}
	for _, m := range c.VolumeMounts {
		assert.NotEqual(t, javaProfilerVolumeName, m.Name)
	}
}

// envFrom Secrets and prefixed sources no longer skip the container: as for the javaagent, the injected env wins
// over envFrom values the operator cannot read.
func TestJavaProfilingProfilesEnvFromSecretAndPrefix(t *testing.T) {
	for name, from := range map[string]corev1.EnvFromSource{
		"secret":              {SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-credentials"}}},
		"prefixed config map": {Prefix: "APP_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-settings"}}},
		"prefixed secret":     {Prefix: "DB_", SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "db-credentials"}}},
	} {
		t.Run(name, func(t *testing.T) {
			setJavaProfiling(t, map[string]string{"endpoint": chartProfilesEndpoint})
			if from.ConfigMapRef != nil {
				// The ConfigMap exists, so getAllEnvVars resolves it (with an unrelated key).
				cm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: from.ConfigMapRef.Name, Namespace: "java-profiling-chart-endpoint"},
					Data: map[string]string{"LOG_LEVEL": "debug"}}
				ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cm.Namespace}}
				if err := k8sClient.Create(context.Background(), &ns); err != nil && !apierrors.IsAlreadyExists(err) {
					require.NoError(t, err)
				}
				require.NoError(t, k8sClient.Create(context.Background(), &cm))
				t.Cleanup(func() { _ = k8sClient.Delete(context.Background(), &cm) })
			}
			c := mutateJavaPod(t, corev1.Container{Name: "app", Image: "app", EnvFrom: []corev1.EnvFromSource{from}})
			assert.Equal(t, "true", getEnvValue(c.Env, envProfilerEnabled))
			assert.Equal(t, chartProfilesEndpoint, getEnvValue(c.Env, envProfilerEndpoint))
		})
	}
}

// A container's own OTEL_AWS_PROFILER_ENABLED in a ConfigMap it loads with envFrom still opts it out.
func TestJavaProfilingConfigMapOptOutWins(t *testing.T) {
	setJavaProfiling(t, map[string]string{"endpoint": chartProfilesEndpoint})
	ctx := context.Background()
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "java-profiling-chart-endpoint"}}
	if err := k8sClient.Create(ctx, &ns); err != nil && !apierrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
	cm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "profiler-off", Namespace: ns.Name},
		Data: map[string]string{envProfilerEnabled: "false"}}
	require.NoError(t, k8sClient.Create(ctx, &cm))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, &cm) })
	c := mutateJavaPod(t, corev1.Container{Name: "app", Image: "app", EnvFrom: []corev1.EnvFromSource{
		{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "profiler-off"}}},
	}})
	assert.Equal(t, -1, getIndexOfEnv(c.Env, envProfilerEnabled), "the ConfigMap value wins, nothing is injected")
	assert.Equal(t, -1, getIndexOfEnv(c.Env, envProfilerEndpoint))
}

// The chart's interface is only enabled: data_dir, volume_size_limit and ADOT options under java.profiling are not
// passed through. The data dir is fixed on the profiler volume, and the volume is 160Mi per profiled container.
func TestJavaProfilingOnlyChartKeys(t *testing.T) {
	setJavaProfiling(t, map[string]string{"endpoint": chartProfilesEndpoint, "data_dir": "/base", "volume_size_limit": "64Mi",
		"mode": "cpu", "window_seconds": "300"})
	pod := mutateJavaPodWithVolumes(t, corev1.Container{Name: "app", Image: "app"}, nil)
	c := pod.Spec.Containers[0]
	assert.Equal(t, "/otel-profiler-data-java/app", getEnvValue(c.Env, envProfilerDataDir))
	assert.Equal(t, -1, getIndexOfEnv(c.Env, "OTEL_AWS_PROFILER_MODE"))
	assert.Equal(t, -1, getIndexOfEnv(c.Env, "OTEL_AWS_PROFILER_WINDOW_SECONDS"))
	var size string
	for _, v := range pod.Spec.Volumes {
		if v.Name == javaProfilerVolumeName {
			size = v.EmptyDir.SizeLimit.String()
		}
	}
	assert.Equal(t, "160Mi", size)
}

// Escape hatch: a container that needs more space mounts its own volume and sets OTEL_AWS_PROFILER_DATA_DIR on it;
// that value is kept verbatim, with its own ADOT settings.
func TestJavaProfilingOwnVolumeDataDir(t *testing.T) {
	setJavaProfiling(t, map[string]string{"endpoint": chartProfilesEndpoint})
	pod := mutateJavaPodWithVolumes(t, corev1.Container{Name: "app", Image: "app",
		Env:          []corev1.EnvVar{{Name: envProfilerDataDir, Value: "/profiler-own/app"}, {Name: "OTEL_AWS_PROFILER_WINDOW_SECONDS", Value: "300"}},
		VolumeMounts: []corev1.VolumeMount{{Name: "profiler-own", MountPath: "/profiler-own"}},
	}, []corev1.Volume{{Name: "profiler-own", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}})
	c := pod.Spec.Containers[0]
	assert.Equal(t, "true", getEnvValue(c.Env, envProfilerEnabled))
	assert.Equal(t, 1, countEnv(c.Env, envProfilerDataDir), "the operator adds no second data dir")
	assert.Equal(t, "/profiler-own/app", getEnvValue(c.Env, envProfilerDataDir))
	assert.Equal(t, 1, countEnv(c.Env, "OTEL_AWS_PROFILER_WINDOW_SECONDS"))
	mounts := map[string]string{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m.MountPath
	}
	assert.Equal(t, "/profiler-own", mounts["profiler-own"], "the container keeps its own mount")
	assert.Equal(t, javaProfilerMountPath, mounts[javaProfilerVolumeName], "the operator volume is still mounted, unused")
	for _, v := range pod.Spec.Volumes {
		if v.Name == javaProfilerVolumeName {
			assert.Equal(t, "160Mi", v.EmptyDir.SizeLimit.String())
		}
	}
}
