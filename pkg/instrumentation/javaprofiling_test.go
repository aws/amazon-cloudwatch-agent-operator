// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

const agentEndpoint = "http://cloudwatch-agent.amazon-cloudwatch:4318/v1development/profiles"

func profilingPod(env ...corev1.EnvVar) corev1.Pod {
	return corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Env: env}}}}
}

func defaultCfg() javaProfilingConfig { return parseJavaProfilingConfig("") }

func cfgFrom(t *testing.T, values map[string]string) javaProfilingConfig {
	raw, err := json.Marshal(values)
	require.NoError(t, err)
	return parseJavaProfilingConfig(string(raw))
}

func profilerVolume(pod corev1.Pod) *corev1.Volume {
	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].Name == "opentelemetry-profiler-data-java" {
			return &pod.Spec.Volumes[i]
		}
	}
	return nil
}

func profilerMounts(c corev1.Container) int {
	n := 0
	for _, m := range c.VolumeMounts {
		if m.Name == "opentelemetry-profiler-data-java" && m.MountPath == "/otel-profiler-data-java" {
			n++
		}
	}
	return n
}

func countEnv(envs []corev1.EnvVar, name string) int {
	n := 0
	for _, e := range envs {
		if e.Name == name {
			n++
		}
	}
	return n
}

func TestInjectJavaProfilingDefaults(t *testing.T) {
	pod, injected, err := injectJavaProfiling(true, defaultCfg(), false, corev1.Namespace{}, profilingPod(), 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, []corev1.EnvVar{
		{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "true"},
		{Name: "OTEL_AWS_PROFILER_ENDPOINT", Value: agentEndpoint},
		{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/otel-profiler-data-java/app"},
	}, pod.Spec.Containers[0].Env)
	v := profilerVolume(pod)
	require.NotNil(t, v, "one pod-level profiler volume")
	require.NotNil(t, v.EmptyDir)
	assert.Equal(t, corev1.StorageMedium(""), v.EmptyDir.Medium, "disk-backed, not memory")
	assert.Equal(t, "160Mi", v.EmptyDir.SizeLimit.String())
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]))
}

func TestJavaProfilingVolumeSize(t *testing.T) {
	cases := map[string]string{"64Mi": "64Mi", "1Gi": "1Gi", "500M": "500M", "16Mi": "16Mi", "abc": "160Mi", "-1Mi": "160Mi", "0": "160Mi", "": "160Mi",
		"160m": "160Mi", "1": "160Mi", "15Mi": "160Mi", "1.5": "160Mi", "20000000.5": "160Mi"}
	for in, want := range cases {
		cfg := cfgFrom(t, map[string]string{"volume_size_limit": in})
		pod, _, err := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, nil)
		require.NoError(t, err)
		assert.Equal(t, want, profilerVolume(pod).EmptyDir.SizeLimit.String(), in)
		assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_VOLUME_SIZE_LIMIT"), "never an env var")
	}
	cfg := cfgFrom(t, map[string]string{"volume_size_limit": "abc"})
	assert.Len(t, cfg.notes, 1, "an invalid size is logged")
	want := resource.MustParse("160Mi")
	assert.Equal(t, 0, want.Cmp(defaultProfilerVolumeSize), "the default is 160Mi")
}

func TestJavaProfilingDataDir(t *testing.T) {
	// The chart base gets the container name appended; the volume is still mounted when the base is outside it.
	cfg := cfgFrom(t, map[string]string{"data_dir": "/var/profiler/"})
	pod, _, err := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, nil)
	require.NoError(t, err)
	assert.Equal(t, "/var/profiler/app", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]))
	assert.NotNil(t, profilerVolume(pod))

	// A relative or ".." base falls back to the default and is logged.
	for _, bad := range []string{"relative/dir", "/a/../etc", "/a/.."} {
		cfg := cfgFrom(t, map[string]string{"data_dir": bad})
		assert.Len(t, cfg.notes, 1, bad)
		pod, _, _ := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, nil)
		assert.Equal(t, "/otel-profiler-data-java/app", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"), bad)
	}

	// ".." inside a segment is a legitimate name.
	cfg = cfgFrom(t, map[string]string{"data_dir": "/data/a..b"})
	assert.Empty(t, cfg.notes)
	pod, _, _ = injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, nil)
	assert.Equal(t, "/data/a..b/app", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))

	// The pod's own DATA_DIR wins, verbatim, with no container suffix; direct env, valueFrom or envFrom.
	own := profilingPod(corev1.EnvVar{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/mine"})
	pod, injected, err := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, own, 0, own.Spec.Containers[0].Env)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, 1, countEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, "/mine", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	pod, _, _ = injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/from-configmap"}})
	assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"), "a ConfigMap envFrom value wins")
}

func TestInjectJavaProfilingTwoContainers(t *testing.T) {
	pod := corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "api"}, {Name: "worker_2"}}}}
	pod, injected, err := injectJavaProfiling(true, defaultCfg(), false, corev1.Namespace{}, pod, 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	pod, injected, err = injectJavaProfiling(true, defaultCfg(), false, corev1.Namespace{}, pod, 1, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, "/otel-profiler-data-java/api", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, "/otel-profiler-data-java/worker-2", getEnvValue(pod.Spec.Containers[1].Env, "OTEL_AWS_PROFILER_DATA_DIR"), "sanitised to one segment")
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]))
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[1]))
	n := 0
	for _, v := range pod.Spec.Volumes {
		if v.Name == "opentelemetry-profiler-data-java" {
			n++
		}
	}
	assert.Equal(t, 1, n, "one pod-level volume for both containers")
	assert.Equal(t, "320Mi", profilerVolume(pod).EmptyDir.SizeLimit.String(), "the size is per profiled container")
}

func TestInjectJavaProfilingSkipsAddNoVolume(t *testing.T) {
	optedOut := profilingPod()
	optedOut.Annotations = map[string]string{"cloudwatch.aws.amazon.com/java-profiling": " FALSE "}
	windows := profilingPod()
	windows.Spec.NodeSelector = map[string]string{"kubernetes.io/os": "windows"}
	windowsOS := profilingPod()
	windowsOS.Spec.OS = &corev1.PodOS{Name: corev1.Windows}
	grpc := profilingPod(corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "grpc"})
	grpcValueFrom := profilingPod(corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", ValueFrom: &corev1.EnvVarSource{
		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}, Key: "p"}}})
	grpcSysprop := profilingPod(corev1.EnvVar{Name: "JAVA_TOOL_OPTIONS", Value: "-Dotel.exporter.otlp.protocol=grpc"})
	secretFrom := profilingPod()
	secretFrom.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "s"}}}}
	prefixedFrom := profilingPod()
	prefixedFrom.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{Prefix: "OTEL_AWS_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}}}}
	ownOff := profilingPod(corev1.EnvVar{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"})
	grpcSyspropFrom := []corev1.EnvVar{{Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx1g -Dotel.exporter.otlp.protocol=GRPC"}}
	nsOptOut := corev1.Namespace{}
	nsOptOut.Annotations = map[string]string{"cloudwatch.aws.amazon.com/java-profiling": "false"}

	tests := []struct {
		name    string
		enabled bool
		ns      corev1.Namespace
		pod     corev1.Pod
		allEnvs []corev1.EnvVar
		wantErr bool
	}{
		{name: "disabled", pod: profilingPod()},
		{name: "pod opt-out", enabled: true, pod: optedOut},
		{name: "namespace opt-out", enabled: true, ns: nsOptOut, pod: profilingPod()},
		{name: "windows", enabled: true, pod: windows, wantErr: true},
		{name: "windows from spec.os", enabled: true, pod: windowsOS, wantErr: true},
		{name: "grpc on the container", enabled: true, pod: grpc, wantErr: true},
		{name: "grpc from envFrom", enabled: true, pod: profilingPod(), allEnvs: []corev1.EnvVar{{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "GRPC"}}, wantErr: true},
		{name: "protocol from valueFrom", enabled: true, pod: grpcValueFrom, wantErr: true},
		{name: "grpc system property", enabled: true, pod: grpcSysprop, wantErr: true},
		{name: "grpc system property from envFrom, any case", enabled: true, pod: profilingPod(), allEnvs: grpcSyspropFrom, wantErr: true},
		{name: "container turns the profiler off", enabled: true, pod: ownOff, allEnvs: ownOff.Spec.Containers[0].Env},
		{name: "envFrom turns the profiler off", enabled: true, pod: profilingPod(), allEnvs: []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"}}},
		{name: "secret envFrom", enabled: true, pod: secretFrom, wantErr: true},
		{name: "prefixed envFrom", enabled: true, pod: prefixedFrom, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(tt.pod.Spec.Containers[0].Env)
			pod, injected, err := injectJavaProfiling(tt.enabled, defaultCfg(), false, tt.ns, tt.pod, 0, tt.allEnvs)
			assert.Equal(t, tt.wantErr, err != nil, err)
			assert.False(t, injected)
			assert.Len(t, pod.Spec.Containers[0].Env, before, "env unchanged")
			assert.Nil(t, profilerVolume(pod), "no volume")
			assert.Equal(t, 0, profilerMounts(pod.Spec.Containers[0]), "no mount")
		})
	}
}

func TestInjectJavaProfilingClashes(t *testing.T) {
	// A customer volume with the profiler's name: skipped, not rejected by the API server.
	pod := profilingPod()
	pod.Spec.Volumes = []corev1.Volume{{Name: "opentelemetry-profiler-data-java", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	out, injected, err := injectJavaProfiling(true, defaultCfg(), javaProfilerVolumePresent(pod), corev1.Namespace{}, pod, 0, nil)
	assert.Error(t, err)
	assert.False(t, injected)
	assert.Len(t, out.Spec.Volumes, 1)
	assert.Empty(t, out.Spec.Containers[0].Env)

	// A customer mount at the profiler's path, or of a volume with its name.
	for _, mount := range []corev1.VolumeMount{{Name: "mine", MountPath: "/otel-profiler-data-java/"}, {Name: "opentelemetry-profiler-data-java", MountPath: "/elsewhere"}} {
		pod := profilingPod()
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{mount}
		out, injected, err := injectJavaProfiling(true, defaultCfg(), false, corev1.Namespace{}, pod, 0, nil)
		assert.Error(t, err, mount.MountPath)
		assert.False(t, injected)
		assert.Nil(t, profilerVolume(out))
		assert.Len(t, out.Spec.Containers[0].VolumeMounts, 1)
	}
}

func TestJavaProfilingReservedKeys(t *testing.T) {
	cfg := cfgFrom(t, map[string]string{
		"enabled": "true", "Enabled": "false", "endpoint": "http://evil:4318", "export_endpoint": "x", "_ENDPOINT": "x",
		"data_dir": "/base", "volume_size_limit": "64Mi", "mode": "cpu",
	})
	assert.Equal(t, []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_MODE", Value: "cpu"}}, cfg.settings, "reserved keys never become generic env vars")
	assert.Equal(t, "/base", cfg.dataDirBase)
	assert.Equal(t, "64Mi", cfg.volumeSize.String())
	notes := strings.Join(cfg.notes, "\n")
	assert.Contains(t, notes, `"endpoint"`)
	assert.Contains(t, notes, `"export_endpoint"`)
	assert.Contains(t, notes, `"Enabled": ignored`, "only the lower-case key is the switch")
	assert.NotContains(t, notes, "http://evil", "values are never logged")

	pod, _, err := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(), 0, nil)
	require.NoError(t, err)
	env := pod.Spec.Containers[0].Env
	assert.Equal(t, "true", getEnvValue(env, "OTEL_AWS_PROFILER_ENABLED"))
	assert.Equal(t, agentEndpoint, getEnvValue(env, "OTEL_AWS_PROFILER_ENDPOINT"))
	assert.Equal(t, "/base/app", getEnvValue(env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, -1, getIndexOfEnv(env, "OTEL_AWS_PROFILER_VOLUME_SIZE_LIMIT"))
	assert.Equal(t, -1, getIndexOfEnv(env, "OTEL_AWS_PROFILER_EXPORT_ENDPOINT"))
	assert.Equal(t, "cpu", getEnvValue(env, "OTEL_AWS_PROFILER_MODE"))
}

func TestJavaProfilingGenericKeys(t *testing.T) {
	keys := []string{"mode", "wall_interval_ms", "cpu_interval_ms", "memory_enabled", "alloc_interval_bytes",
		"window_seconds", "aggregation_mode", "export_compression", "export_timeout_ms", "max_payload_bytes"}
	values := map[string]string{}
	for _, k := range keys {
		values[k] = "v-" + k
	}
	cfg := cfgFrom(t, values)
	assert.Empty(t, cfg.notes, "every settable ADOT v2.32.0 option is known")
	require.Len(t, cfg.settings, len(keys))
	for _, k := range keys {
		assert.Equal(t, "v-"+k, getEnvValue(cfg.settings, "OTEL_AWS_PROFILER_"+strings.ToUpper(k)), k)
	}
	assert.Len(t, adotProfilerSettings, len(keys), "the known-name list matches the chart's generic list")

	// Strict names, control characters, empty values, duplicates and unknown names.
	cfg = cfgFrom(t, map[string]string{"mode-x": "cpu", "a.b": "1", "empty": "", "MODE": "wall", "mode": "cpu", "window_seconds": "30\nforged", "windows_seconds": "30"})
	assert.Equal(t, []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_MODE", Value: "wall"}, {Name: "OTEL_AWS_PROFILER_WINDOWS_SECONDS", Value: "30"}}, cfg.settings)
	assert.Len(t, cfg.notes, 6)
	assert.NotContains(t, strings.Join(cfg.notes, "\n"), "forged")
	assert.Len(t, parseJavaProfilingConfig("not json").notes, 1)
}

func TestInjectJavaProfilingCustomerWins(t *testing.T) {
	cfg := cfgFrom(t, map[string]string{"mode": "cpu", "memory_enabled": "true"})
	ref := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}, Key: "mode"}}
	own := []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_MODE", ValueFrom: ref}}
	allEnvs := append(append([]corev1.EnvVar{}, own...), corev1.EnvVar{Name: "OTEL_AWS_PROFILER_MEMORY_ENABLED", Value: ""})
	pod, _, err := injectJavaProfiling(true, cfg, false, corev1.Namespace{}, profilingPod(own...), 0, allEnvs)
	require.NoError(t, err)
	env := pod.Spec.Containers[0].Env
	assert.Equal(t, 1, countEnv(env, "OTEL_AWS_PROFILER_MODE"), "a valueFrom on the pod wins")
	assert.NotNil(t, env[0].ValueFrom)
	assert.Equal(t, -1, getIndexOfEnv(env, "OTEL_AWS_PROFILER_MEMORY_ENABLED"), "an empty ConfigMap value still counts as set")
}

func TestJavaProfilingEnabled(t *testing.T) {
	t.Setenv("AUTO_INSTRUMENTATION_JAVA_PROFILING_ENABLED", "TRUE")
	assert.False(t, javaProfilingEnabled(), "exactly \"true\", like the chart")
	t.Setenv("AUTO_INSTRUMENTATION_JAVA_PROFILING_ENABLED", "true")
	assert.True(t, javaProfilingEnabled())
}

// TestInjectJavaProfilingThroughSDKInjector covers the wiring in sdkInjector.inject: per-container decisions, one
// volume sized per profiled container, and a skipped container left without a mount.
func TestInjectJavaProfilingThroughSDKInjector(t *testing.T) {
	t.Setenv(envJavaProfilingEnabled, "true")
	inst := v1alpha1.Instrumentation{Spec: v1alpha1.InstrumentationSpec{
		Java:     v1alpha1.Java{Image: "img:1"},
		Exporter: v1alpha1.Exporter{Endpoint: "http://collector:4316"},
	}}
	insts := languageInstrumentations{Java: instrumentationWithContainers{Instrumentation: &inst, Containers: "api,grpc,worker"}}
	inj := sdkInjector{logger: logr.Discard()}
	pod := inj.inject(context.Background(), insts, corev1.Namespace{}, corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "api"},
		{Name: "grpc", Env: []corev1.EnvVar{{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "grpc"}}},
		{Name: "worker"},
	}}})
	require.NotNil(t, profilerVolume(pod))
	assert.Equal(t, "320Mi", profilerVolume(pod).EmptyDir.SizeLimit.String(), "two profiled containers")
	assert.Equal(t, "/otel-profiler-data-java/api", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, "/otel-profiler-data-java/worker", getEnvValue(pod.Spec.Containers[2].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]))
	assert.Equal(t, 0, profilerMounts(pod.Spec.Containers[1]), "the gRPC container is skipped")
	assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[1].Env, "OTEL_AWS_PROFILER_ENABLED"))
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[2]))
}
