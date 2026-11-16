// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// agentEndpoint is the endpoint the chart passes in java.profiling.endpoint (the node agent's OTLP HTTP endpoint).
const agentEndpoint = "http://cloudwatch-agent.amazon-cloudwatch:14318/v1development/profiles"

func profilingPod(env ...corev1.EnvVar) corev1.Pod {
	return corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Env: env}}}}
}

// defaultCfg is the config the chart passes with only profiling enabled: the endpoint, no other key.
func defaultCfg() javaProfilingConfig {
	return parseJavaProfilingConfig(`{"endpoint":"` + agentEndpoint + `"}`)
}

// cfgFrom parses values plus the chart's endpoint (unless values sets "endpoint" itself).
func cfgFrom(t *testing.T, values map[string]string) javaProfilingConfig {
	withEndpoint := map[string]string{"endpoint": agentEndpoint}
	for k, v := range values {
		withEndpoint[k] = v
	}
	raw, err := json.Marshal(withEndpoint)
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
	pod, injected, err := injectJavaProfiling(true, defaultCfg(), false, profilingPod(), 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, []corev1.EnvVar{
		{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "true"},
		{Name: "OTEL_AWS_PROFILER_ENDPOINT", Value: agentEndpoint},
		{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/otel-profiler-data-java/app"},
		{Name: "OTEL_AWS_PROFILER_MEMORY_ENABLED", Value: "true"},
	}, pod.Spec.Containers[0].Env)
	v := profilerVolume(pod)
	require.NotNil(t, v, "one pod-level profiler volume")
	require.NotNil(t, v.EmptyDir)
	assert.Equal(t, corev1.StorageMedium(""), v.EmptyDir.Medium, "disk-backed, not memory")
	assert.Equal(t, "160Mi", v.EmptyDir.SizeLimit.String())
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]))
}

// The profiler volume's size is fixed: 160Mi per profiled container, whatever the settings say.
func TestJavaProfilingVolumeSizeFixed(t *testing.T) {
	want := resource.MustParse("160Mi")
	assert.Equal(t, 0, want.Cmp(defaultProfilerVolumeSize), "the size is 160Mi")
	for _, in := range []string{"64Mi", "1Gi", "abc"} {
		cfg := cfgFrom(t, map[string]string{"volume_size_limit": in})
		assert.Contains(t, strings.Join(cfg.notes, "\n"), `"volume_size_limit": unknown, ignored`, in)
		pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
		require.NoError(t, err)
		assert.Equal(t, "160Mi", profilerVolume(pod).EmptyDir.SizeLimit.String(), in)
		assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_VOLUME_SIZE_LIMIT"), "never an env var")
	}
}

// The data dir is always <mount>/<container>; data_dir is not a setting. A container's own
// OTEL_AWS_PROFILER_DATA_DIR wins verbatim (its escape hatch to a volume of its own size); the operator's volume is
// still mounted, unused.
func TestJavaProfilingDataDir(t *testing.T) {
	cfg := cfgFrom(t, map[string]string{"data_dir": "/var/profiler/"})
	assert.Contains(t, strings.Join(cfg.notes, "\n"), `"data_dir": unknown, ignored`)
	pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
	require.NoError(t, err)
	assert.Equal(t, "/otel-profiler-data-java/app", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))

	own := profilingPod(corev1.EnvVar{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/mine/app"})
	own.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "mine", MountPath: "/mine"}}
	own.Spec.Volumes = []corev1.Volume{{Name: "mine", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	pod, injected, err := injectJavaProfiling(true, defaultCfg(), false, own, 0, own.Spec.Containers[0].Env)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, 1, countEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, "/mine/app", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, 1, profilerMounts(pod.Spec.Containers[0]), "the operator volume is still mounted")
	assert.Len(t, pod.Spec.Containers[0].VolumeMounts, 2, "the container keeps its own mount")
	pod, _, _ = injectJavaProfiling(true, defaultCfg(), false, profilingPod(), 0, []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/from-configmap"}})
	assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"), "a ConfigMap envFrom value wins")
}

func TestInjectJavaProfilingTwoContainers(t *testing.T) {
	pod := corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "api"}, {Name: "worker"}}}}
	pod, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	pod, injected, err = injectJavaProfiling(true, defaultCfg(), false, pod, 1, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, "/otel-profiler-data-java/api", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
	assert.Equal(t, "/otel-profiler-data-java/worker", getEnvValue(pod.Spec.Containers[1].Env, "OTEL_AWS_PROFILER_DATA_DIR"))
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
	windows := profilingPod()
	windows.Spec.NodeSelector = map[string]string{"kubernetes.io/os": "windows"}
	windowsOS := profilingPod()
	windowsOS.Spec.OS = &corev1.PodOS{Name: corev1.Windows}
	grpc := profilingPod(corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "grpc"})
	grpcValueFrom := profilingPod(corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", ValueFrom: &corev1.EnvVarSource{
		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}, Key: "p"}}})
	grpcSysprop := profilingPod(corev1.EnvVar{Name: "JAVA_TOOL_OPTIONS", Value: "-Dotel.exporter.otlp.protocol=grpc"})
	ownOff := profilingPod(corev1.EnvVar{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"})
	grpcSyspropFrom := []corev1.EnvVar{{Name: "JAVA_TOOL_OPTIONS", Value: "-Xmx1g -Dotel.exporter.otlp.protocol=GRPC"}}

	tests := []struct {
		name    string
		enabled bool
		pod     corev1.Pod
		allEnvs []corev1.EnvVar
		wantErr bool
	}{
		{name: "disabled", pod: profilingPod()},
		{name: "windows", enabled: true, pod: windows, wantErr: true},
		{name: "windows from spec.os", enabled: true, pod: windowsOS, wantErr: true},
		{name: "grpc on the container", enabled: true, pod: grpc, wantErr: true},
		{name: "grpc from envFrom", enabled: true, pod: profilingPod(), allEnvs: []corev1.EnvVar{{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "GRPC"}}, wantErr: true},
		{name: "protocol from valueFrom", enabled: true, pod: grpcValueFrom, wantErr: true},
		{name: "grpc system property", enabled: true, pod: grpcSysprop, wantErr: true},
		{name: "grpc system property from envFrom, any case", enabled: true, pod: profilingPod(), allEnvs: grpcSyspropFrom, wantErr: true},
		{name: "container turns the profiler off", enabled: true, pod: ownOff, allEnvs: ownOff.Spec.Containers[0].Env},
		{name: "envFrom turns the profiler off", enabled: true, pod: profilingPod(), allEnvs: []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(tt.pod.Spec.Containers[0].Env)
			pod, injected, err := injectJavaProfiling(tt.enabled, defaultCfg(), false, tt.pod, 0, tt.allEnvs)
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
	out, injected, err := injectJavaProfiling(true, defaultCfg(), javaProfilerVolumePresent(pod), pod, 0, nil)
	assert.Error(t, err)
	assert.False(t, injected)
	assert.Len(t, out.Spec.Volumes, 1)
	assert.Empty(t, out.Spec.Containers[0].Env)

	// A customer mount at the profiler's path, or of a volume with its name.
	for _, mount := range []corev1.VolumeMount{{Name: "mine", MountPath: "/otel-profiler-data-java/"}, {Name: "opentelemetry-profiler-data-java", MountPath: "/elsewhere"}} {
		pod := profilingPod()
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{mount}
		out, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
		assert.Error(t, err, mount.MountPath)
		assert.False(t, injected)
		assert.Nil(t, profilerVolume(out))
		assert.Len(t, out.Spec.Containers[0].VolumeMounts, 1)
	}
}

func TestJavaProfilingChartKeysOnly(t *testing.T) {
	cfg := cfgFrom(t, map[string]string{
		"enabled": "true", "Enabled": "false", "export_endpoint": "x", "_ENDPOINT": "x", "Endpoint": "http://evil:4318",
		"data_dir": "/base", "volume_size_limit": "64Mi", "mode": "cpu", "memory_enabled": "false", "max_payload_bytes": "20971520", "MAX_PAYLOAD_BYTES": "1",
		"ENABLED": "false",
	})
	notes := strings.Join(cfg.notes, "\n")
	assert.Equal(t, agentEndpoint, cfg.endpoint, "the chart's lower-case endpoint key is the endpoint")
	assert.Equal(t, "20971520", cfg.maxPayloadBytes, "only the lower-case spelling is read")
	for _, key := range []string{"Endpoint", "export_endpoint", "_ENDPOINT", "data_dir", "volume_size_limit", "mode", "memory_enabled", "MAX_PAYLOAD_BYTES", "Enabled", "ENABLED"} {
		assert.Contains(t, notes, `"`+key+`": unknown, ignored`, key)
	}
	assert.NotContains(t, notes, "http://evil", "values are never logged")

	pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
	require.NoError(t, err)
	assert.Equal(t, []corev1.EnvVar{
		{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "true"},
		{Name: "OTEL_AWS_PROFILER_ENDPOINT", Value: agentEndpoint},
		{Name: "OTEL_AWS_PROFILER_DATA_DIR", Value: "/otel-profiler-data-java/app"},
		{Name: "OTEL_AWS_PROFILER_MEMORY_ENABLED", Value: "true"},
		{Name: "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES", Value: "20971520"},
	}, pod.Spec.Containers[0].Env, "exactly these five, memory the operator's own: no pass-through of other keys")
}

// The chart-passed endpoint and payload cap are validated; a bad value is not used.
func TestJavaProfilingChartKeysValidated(t *testing.T) {
	for _, bad := range []string{"cloudwatch-agent:4318", "ftp://agent:21/p", "http://", "http://user:pw@agent:4318/p", "http://a\nb",
		"http://:4318/p", "http://agent:99999/p", "http://agent:0/p", "http://agent:4318/p?token=x", "http://agent:4318/p#f", "http://$(HOST):4318/p"} {
		cfg := parseJavaProfilingConfig(`{"endpoint":"` + bad + `"}`)
		assert.Empty(t, cfg.endpoint, bad)
		pod, injected, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
		assert.NoError(t, err)
		assert.False(t, injected, "no endpoint, no injection: %s", bad)
		assert.Empty(t, pod.Spec.Containers[0].Env)
	}
	for _, bad := range []string{"0", "-1", "20MiB", "1e6"} {
		cfg := cfgFrom(t, map[string]string{"max_payload_bytes": bad})
		assert.Empty(t, cfg.maxPayloadBytes, bad)
		pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
		require.NoError(t, err)
		assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES"), bad)
	}
	assert.Equal(t, "https://agent.example:443/v1development/profiles",
		parseJavaProfilingConfig(`{"endpoint":"https://agent.example:443/v1development/profiles"}`).endpoint)
}

// A container that sends to its own endpoint keeps it and gets no chart payload cap (that cap is for the chart's
// receiver); the gRPC check does not apply to it.
func TestInjectJavaProfilingOwnEndpoint(t *testing.T) {
	own := []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_ENDPOINT", Value: "http://own:4318/v1development/profiles"}, {Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: "grpc"}}
	cfg := cfgFrom(t, map[string]string{"max_payload_bytes": "20971520"})
	pod, injected, err := injectJavaProfiling(true, cfg, false, profilingPod(own...), 0, own)
	require.NoError(t, err)
	assert.True(t, injected)
	env := pod.Spec.Containers[0].Env
	assert.Equal(t, 1, countEnv(env, "OTEL_AWS_PROFILER_ENDPOINT"))
	assert.Equal(t, "http://own:4318/v1development/profiles", getEnvValue(env, "OTEL_AWS_PROFILER_ENDPOINT"))
	assert.Equal(t, -1, getIndexOfEnv(env, "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES"))
}

// A Secret envFrom value cannot be read, so the injected env overrides it (env wins over envFrom), as for the
// javaagent: a Secret cannot opt a container out.
func TestInjectJavaProfilingSecretCannotOptOut(t *testing.T) {
	pod := profilingPod()
	pod.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "has-profiler-enabled-false"}}}}
	out, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, "true", getEnvValue(out.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_ENABLED"), "env overrides the Secret's value")
}

// envFrom Secrets and prefixed sources do not skip the container (env wins over envFrom, as for the javaagent).
func TestInjectJavaProfilingEnvFromSecretOrPrefix(t *testing.T) {
	for name, from := range map[string]corev1.EnvFromSource{
		"secret":              {SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "s"}}},
		"prefixed config map": {Prefix: "OTEL_AWS_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}}},
	} {
		t.Run(name, func(t *testing.T) {
			pod := profilingPod()
			pod.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{from}
			out, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
			require.NoError(t, err)
			assert.True(t, injected)
			assert.Equal(t, agentEndpoint, getEnvValue(out.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_ENDPOINT"))
		})
	}
}

// There is no generic OTEL_AWS_PROFILER_<KEY> mapping: ADOT options in java.profiling are ignored and reported.
func TestJavaProfilingNoPassThrough(t *testing.T) {
	keys := []string{"mode", "wall_interval_ms", "cpu_interval_ms", "memory_enabled", "alloc_interval_bytes",
		"window_seconds", "aggregation_mode", "export_compression", "export_timeout_ms"}
	values := map[string]string{}
	for _, k := range keys {
		values[k] = "v-" + k
	}
	cfg := cfgFrom(t, values)
	assert.Len(t, cfg.notes, len(keys))
	pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(), 0, nil)
	require.NoError(t, err)
	for _, k := range keys {
		if k == "memory_enabled" {
			// Memory is the operator's default, never the key's value.
			assert.Equal(t, "true", getEnvValue(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_MEMORY_ENABLED"), k)
			continue
		}
		assert.Equal(t, -1, getIndexOfEnv(pod.Spec.Containers[0].Env, "OTEL_AWS_PROFILER_"+strings.ToUpper(k)), k)
	}
	cfg = cfgFrom(t, map[string]string{"empty": "", "max_payload_bytes": "30\nforged"})
	assert.NotContains(t, strings.Join(cfg.notes, "\n"), "forged", "values are never logged")
	assert.Empty(t, cfg.maxPayloadBytes, "a value with a control character is not a positive integer")
	assert.Len(t, parseJavaProfilingConfig("not json").notes, 1)
}

// Per-workload tuning: the container's own OTEL_AWS_PROFILER_* values (env, valueFrom or a ConfigMap) win.
func TestInjectJavaProfilingCustomerWins(t *testing.T) {
	cfg := cfgFrom(t, map[string]string{"max_payload_bytes": "20971520"})
	ref := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}, Key: "enabled"}}
	own := []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_MODE", Value: "cpu"}, {Name: "OTEL_AWS_PROFILER_DATA_DIR", ValueFrom: ref}}
	allEnvs := append(append([]corev1.EnvVar{}, own...), corev1.EnvVar{Name: "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES", Value: ""})
	pod, _, err := injectJavaProfiling(true, cfg, false, profilingPod(own...), 0, allEnvs)
	require.NoError(t, err)
	env := pod.Spec.Containers[0].Env
	assert.Equal(t, "cpu", getEnvValue(env, "OTEL_AWS_PROFILER_MODE"), "the container's own ADOT setting stays")
	assert.Equal(t, 1, countEnv(env, "OTEL_AWS_PROFILER_DATA_DIR"), "a valueFrom on the pod wins")
	assert.Equal(t, -1, getIndexOfEnv(env, "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES"), "an empty ConfigMap value still counts as set")
}

// injectedDefaults are the ADOT settings the operator injects as defaults, with a value a container may set itself.
var injectedDefaults = []struct{ name, value, own string }{
	{name: "OTEL_AWS_PROFILER_MEMORY_ENABLED", value: "true", own: "false"},
}

// Each injected default follows the rule of the other profiler variables: a value the container sets itself wins,
// in env (a value or a valueFrom) or in a ConfigMap it loads with envFrom (allEnvs). A Secret envFrom value, which the
// operator cannot read, does not stop the default (Kubernetes gives env precedence over envFrom). An opted-out
// container, or a pod with profiling off, gets none of them. ADOT reads the matching system property
// (-Dotel.aws.profiler.<name>) first; the operator does not look for it, so it still injects the default.
func TestInjectJavaProfilingInjectedDefaults(t *testing.T) {
	for _, d := range injectedDefaults {
		t.Run(d.name, func(t *testing.T) {
			ref := &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "c"}, Key: "k"}}
			tests := []struct {
				name      string
				pod       corev1.Pod
				wantValue string
				wantFrom  bool
			}{
				{name: "default", pod: profilingPod(), wantValue: d.value},
				{name: "own value in env", pod: profilingPod(corev1.EnvVar{Name: d.name, Value: d.own}), wantValue: d.own},
				{name: "own empty value in env", pod: profilingPod(corev1.EnvVar{Name: d.name, Value: ""}), wantValue: ""},
				{name: "own valueFrom", pod: profilingPod(corev1.EnvVar{Name: d.name, ValueFrom: ref}), wantFrom: true},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					out, injected, err := injectJavaProfiling(true, defaultCfg(), false, tt.pod, 0, tt.pod.Spec.Containers[0].Env)
					require.NoError(t, err)
					assert.True(t, injected)
					env := out.Spec.Containers[0].Env
					require.Equal(t, 1, countEnv(env, d.name), "set exactly once")
					idx := getIndexOfEnv(env, d.name)
					assert.Equal(t, tt.wantValue, env[idx].Value)
					assert.Equal(t, tt.wantFrom, env[idx].ValueFrom != nil)
				})
			}
			t.Run("configmap envFrom wins", func(t *testing.T) {
				out, injected, err := injectJavaProfiling(true, defaultCfg(), false, profilingPod(), 0, []corev1.EnvVar{{Name: d.name, Value: d.own}})
				require.NoError(t, err)
				assert.True(t, injected)
				assert.Equal(t, -1, getIndexOfEnv(out.Spec.Containers[0].Env, d.name), "the ConfigMap value is the one ADOT reads")
			})
			t.Run("opted out", func(t *testing.T) {
				off := profilingPod(corev1.EnvVar{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"})
				out, injected, err := injectJavaProfiling(true, defaultCfg(), false, off, 0, off.Spec.Containers[0].Env)
				require.NoError(t, err)
				assert.False(t, injected)
				assert.Equal(t, -1, getIndexOfEnv(out.Spec.Containers[0].Env, d.name))
			})
			t.Run("not enabled", func(t *testing.T) {
				out, _, _ := injectJavaProfiling(false, defaultCfg(), false, profilingPod(), 0, nil)
				assert.Equal(t, -1, getIndexOfEnv(out.Spec.Containers[0].Env, d.name))
			})
			t.Run("system property left alone", func(t *testing.T) {
				prop := "-D" + strings.ToLower(strings.ReplaceAll(d.name, "_", ".")) + "=" + d.own
				own := profilingPod(corev1.EnvVar{Name: "JAVA_TOOL_OPTIONS", Value: prop})
				out, injected, err := injectJavaProfiling(true, defaultCfg(), false, own, 0, own.Spec.Containers[0].Env)
				require.NoError(t, err)
				assert.True(t, injected)
				assert.Equal(t, prop, getEnvValue(out.Spec.Containers[0].Env, "JAVA_TOOL_OPTIONS"))
				assert.Equal(t, d.value, getEnvValue(out.Spec.Containers[0].Env, d.name), "not special-cased: the default is still injected")
			})
		})
	}
}

// A value in the Instrumentation's env is in the container's env before the profiler is injected, so it wins over
// the operator's default for every container it instruments.
func TestInjectJavaProfilingDefaultsFromInstrumentation(t *testing.T) {
	t.Setenv(envJavaProfilingEnabled, "true")
	t.Setenv(envJavaProfilingSettings, `{"endpoint":"`+agentEndpoint+`"}`)
	javaProfilingConfigOnce = sync.Once{}
	t.Cleanup(func() { javaProfilingConfigOnce = sync.Once{} })
	var own []corev1.EnvVar
	for _, d := range injectedDefaults {
		own = append(own, corev1.EnvVar{Name: d.name, Value: d.own})
	}
	for name, inst := range map[string]v1alpha1.Instrumentation{
		"spec.env":      {Spec: v1alpha1.InstrumentationSpec{Java: v1alpha1.Java{Image: "img:1"}, Env: own}},
		"spec.java.env": {Spec: v1alpha1.InstrumentationSpec{Java: v1alpha1.Java{Image: "img:1", Env: own}}},
	} {
		t.Run(name, func(t *testing.T) {
			inst := inst
			insts := languageInstrumentations{Java: instrumentationWithContainers{Instrumentation: &inst, Containers: "app"}}
			inj := sdkInjector{logger: logr.Discard()}
			pod := inj.inject(context.Background(), insts, corev1.Namespace{}, profilingPod())
			env := pod.Spec.Containers[0].Env
			assert.Equal(t, "true", getEnvValue(env, "OTEL_AWS_PROFILER_ENABLED"), "profiled")
			for _, d := range injectedDefaults {
				assert.Equal(t, 1, countEnv(env, d.name), d.name)
				assert.Equal(t, d.own, getEnvValue(env, d.name), d.name)
			}
		})
	}
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
	t.Setenv(envJavaProfilingSettings, `{"endpoint":"`+agentEndpoint+`"}`)
	javaProfilingConfigOnce = sync.Once{}
	t.Cleanup(func() { javaProfilingConfigOnce = sync.Once{} })
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

func TestInjectJavaProfilingSkipsFargate(t *testing.T) {
	bySchedulerName := profilingPod()
	bySchedulerName.Spec.SchedulerName = "fargate-scheduler"
	byProfileLabel := profilingPod()
	byProfileLabel.Labels = map[string]string{"eks.amazonaws.com/fargate-profile": "fp-default"}
	for name, pod := range map[string]corev1.Pod{"scheduler name": bySchedulerName, "profile label": byProfileLabel} {
		t.Run(name, func(t *testing.T) {
			out, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
			assert.Error(t, err)
			assert.False(t, injected)
			assert.Equal(t, -1, getIndexOfEnv(out.Spec.Containers[0].Env, envProfilerEnabled))
			assert.Nil(t, profilerVolume(out))
		})
	}
}

// No pod or namespace annotation opts out of profiling; the container's own OTEL_AWS_PROFILER_ENABLED=false does.
func TestInjectJavaProfilingIgnoresAnnotations(t *testing.T) {
	pod := profilingPod()
	pod.Annotations = map[string]string{"cloudwatch.aws.amazon.com/java-profiling": "false"}
	out, injected, err := injectJavaProfiling(true, defaultCfg(), false, pod, 0, nil)
	require.NoError(t, err)
	assert.True(t, injected)
	assert.Equal(t, "true", getEnvValue(out.Spec.Containers[0].Env, envProfilerEnabled))
}
