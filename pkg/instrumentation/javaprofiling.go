// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

// Continuous Java profiling with the async-profiler that the ADOT Java agent embeds (v2.32.0 and later). The operator
// enables the profiler, points it at the agent, and gives it a dedicated disk-backed emptyDir for its data dir (the
// native library and rotating JFR files). Profiles go to the endpoint the chart passes (java.profiling.endpoint, the
// node agent's OTLP HTTP endpoint), with the payload cap it passes for that receiver (java.profiling.max_payload_bytes).
// Without an endpoint nothing is injected. It also selects ADOT settings the chart does not expose: allocation profiling
// (defaultProfiler* below), which a container overrides in its own env.
const (
	// defaultProfilerMemoryEnabled turns on ADOT's allocation profiling (off in ADOT by default). async-profiler then
	// adds alloc=<OTEL_AWS_PROFILER_ALLOC_INTERVAL_BYTES> to the same session, and ADOT exports alloc_space and
	// alloc_objects profiles next to the wall one. On JDK 11 and later it uses JVMTI allocation sampling.
	// Before JDK 11 it needs HotSpot's AllocTracer symbols in libjvm; without them async-profiler fails to start the
	// whole session, so such a container is not profiled at all until it sets OTEL_AWS_PROFILER_MEMORY_ENABLED=false.
	defaultProfilerMemoryEnabled = "true"

	envJavaProfilingEnabled  = "AUTO_INSTRUMENTATION_JAVA_PROFILING_ENABLED"
	envJavaProfilingSettings = "AUTO_INSTRUMENTATION_JAVA_PROFILING_SETTINGS"

	envProfilerEnabled         = "OTEL_AWS_PROFILER_ENABLED"
	envProfilerEndpoint        = "OTEL_AWS_PROFILER_ENDPOINT"
	envProfilerDataDir         = "OTEL_AWS_PROFILER_DATA_DIR"
	envProfilerMaxPayloadBytes = "OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES"
	envProfilerMemoryEnabled   = "OTEL_AWS_PROFILER_MEMORY_ENABLED"
	envOTelExporterProtocol    = "OTEL_EXPORTER_OTLP_PROTOCOL"

	// fargateSchedulerName and fargateProfileLabel mark an EKS Fargate pod, which has no node-local agent.
	fargateSchedulerName = "fargate-scheduler"
	fargateProfileLabel  = "eks.amazonaws.com/fargate-profile"

	// javaProfilerVolumeName is the pod-level emptyDir that holds every profiled container's data dir. It is
	// disk-backed (no medium: Memory), so it is writable with readOnlyRootFilesystem and isolated from the
	// javaagent volume; its sizeLimit is defaultProfilerVolumeSize per profiled container.
	javaProfilerVolumeName = "opentelemetry-profiler-data-java"
	// javaProfilerMountPath is where the volume is mounted in each profiled Java container; each container's data
	// dir is <mount>/<container-name>, unless the container sets OTEL_AWS_PROFILER_DATA_DIR itself.
	javaProfilerMountPath = "/otel-profiler-data-java"
)

var (
	javaProfilingConfigOnce sync.Once
	javaProfilingConfigData javaProfilingConfig
)

// javaProfilingConfig is the parsed java.profiling block, other than enabled.
type javaProfilingConfig struct {
	// endpoint is the profiles URL the chart passes; empty means no endpoint, so nothing is injected.
	endpoint string
	// maxPayloadBytes is the payload cap the chart passes with the endpoint; empty leaves ADOT's default.
	maxPayloadBytes string
	// notes lists one line per rejected or unknown key, without values.
	notes []string
}

// javaProfilingEnabled reports whether main.go copied java.profiling.enabled "true" from
// --auto-instrumentation-config.
func javaProfilingEnabled() bool {
	// Exactly "true", the value the chart requires for enabled ("true" or "false").
	return os.Getenv(envJavaProfilingEnabled) == "true"
}

// parseJavaProfilingConfig parses the java.profiling keys other than enabled, as main.go copies them as JSON. The
// chart passes exactly two:
//   - endpoint, the profiles URL, which must pass validProfilesEndpoint;
//   - max_payload_bytes, the receiver's payload cap, a positive integer.
//
// Any other key is ignored and reported, never mapped to an env var: ADOT is tuned per workload with
// OTEL_AWS_PROFILER_* in the container's own env, which always wins. The profiler volume's size is fixed:
// defaultProfilerVolumeSize per profiled container. A container that needs more space mounts its own volume and sets
// OTEL_AWS_PROFILER_DATA_DIR to a path on it.
func parseJavaProfilingConfig(raw string) javaProfilingConfig {
	cfg := javaProfilingConfig{}
	if raw == "" {
		return cfg
	}
	values := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		cfg.notes = append(cfg.notes, fmt.Sprintf("java.profiling settings are not a string map: %v", err))
		return cfg
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		switch key {
		case "endpoint":
			if !validProfilesEndpoint(value) {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: not an http or https URL with a host", key))
				continue
			}
			cfg.endpoint = value
		case "max_payload_bytes":
			if n, err := strconv.ParseInt(value, 10, 64); err != nil || n <= 0 {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: not a positive integer; using ADOT's default", key))
				continue
			}
			cfg.maxPayloadBytes = value
		default:
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: unknown, ignored (set OTEL_AWS_PROFILER_* in the container's env to tune ADOT)", key))
		}
	}
	return cfg
}

// javaProfilingConfigFromEnv returns parseJavaProfilingConfig for the settings main.go copied, computed once
// (they do not change while the operator runs). first is true on the first call only, so the caller logs the
// notes once.
func javaProfilingConfigFromEnv() (cfg javaProfilingConfig, first bool) {
	javaProfilingConfigOnce.Do(func() {
		first = true
		javaProfilingConfigData = parseJavaProfilingConfig(os.Getenv(envJavaProfilingSettings))
	})
	return javaProfilingConfigData, first
}

// envSetByCustomer reports whether the container already defines name: directly (with a value or a
// valueFrom) or through envFrom, as resolved in allEnvs.
func envSetByCustomer(container *corev1.Container, allEnvs []corev1.EnvVar, name string) bool {
	if getIndexOfEnv(container.Env, name) > -1 {
		return true
	}
	return getIndexOfEnv(allEnvs, name) > -1
}

// javaProfilerVolumePresent reports whether the pod has a volume named javaProfilerVolumeName. Called on the pod
// as it was before any injection, it detects a customer volume with that name (a clash).
func javaProfilerVolumePresent(pod corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == javaProfilerVolumeName {
			return true
		}
	}
	return false
}

// validProfilesEndpoint reports whether value is an http or https URL with a host name, an optional port in 1-65535,
// and no user info, query, fragment or $( (which Kubernetes would expand from the container's env).
func validProfilesEndpoint(value string) bool {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || strings.Contains(value, "$(") {
		return false
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

// isFargatePod reports whether the pod runs on EKS Fargate, where no node-local cloudwatch-agent receives profiles.
func isFargatePod(pod corev1.Pod) bool {
	if pod.Spec.SchedulerName == fargateSchedulerName {
		return true
	}
	_, ok := pod.Labels[fargateProfileLabel]
	return ok
}

// injectJavaProfiling adds OTEL_AWS_PROFILER_ENABLED, OTEL_AWS_PROFILER_ENDPOINT (cfg.endpoint),
// OTEL_AWS_PROFILER_MAX_PAYLOAD_BYTES (cfg.maxPayloadBytes, when passed),
// OTEL_AWS_PROFILER_DATA_DIR=/otel-profiler-data-java/<container-name> and OTEL_AWS_PROFILER_MEMORY_ENABLED=true
// (defaultProfilerMemoryEnabled) to the Java container at index after the javaagent was injected, mounts the profiler
// volume in it, and adds the volume to the pod once, growing its sizeLimit by defaultProfilerVolumeSize per profiled container. Without
// cfg.endpoint it injects nothing (the caller logs that once).
//
// Precedence, as for the javaagent and Application Signals variables: a variable the container sets in env, or in a
// ConfigMap it loads with envFrom (allEnvs), wins and is not injected. A customer OTEL_AWS_PROFILER_DATA_DIR is used
// verbatim (a path outside the volume is not on it). A value from an envFrom Secret, which the operator cannot read,
// is overridden by the injected env, because Kubernetes gives env precedence over envFrom. A variable from the
// Instrumentation resource's env is already in the container's env here, so it wins too
// (OTEL_AWS_PROFILER_MEMORY_ENABLED=false in a user-written Instrumentation keeps allocation profiling off for every
// pod it instruments; the operator's default Instrumentation has no such env). An envFrom ConfigMap loaded with a
// prefix is matched without the prefix (resolveEnvFrom), and only a literal OTEL_AWS_PROFILER_ENABLED value opts out. ADOT reads the
// -Dotel.aws.profiler.memory.enabled system property before the env var, so a container that sets it (in
// JAVA_TOOL_OPTIONS, for example) keeps its own value; the operator does not look for it. volumeClash is
// javaProfilerVolumePresent of the pod as it was before any injection, a customer volume with the profiler's name. It
// returns whether it injected.
func injectJavaProfiling(enabled bool, cfg javaProfilingConfig, volumeClash bool, pod corev1.Pod, index int, allEnvs []corev1.EnvVar) (corev1.Pod, bool, error) {
	if !enabled || cfg.endpoint == "" {
		return pod, false, nil
	}
	if isFargatePod(pod) {
		return pod, false, fmt.Errorf("the pod runs on Fargate, which has no node-local cloudwatch-agent")
	}
	// isWindowsPod reads only the nodeSelector; spec.os.name also marks a Windows pod.
	if isWindowsPod(pod) || (pod.Spec.OS != nil && pod.Spec.OS.Name == corev1.Windows) {
		return pod, false, fmt.Errorf("the ADOT Java profiler is linux only")
	}
	container := &pod.Spec.Containers[index]
	// A container that turns the profiler off itself gets no volume or mount (its own value wins anyway).
	if idx := getIndexOfEnv(container.Env, envProfilerEnabled); idx > -1 {
		if container.Env[idx].ValueFrom == nil && !strings.EqualFold(strings.TrimSpace(container.Env[idx].Value), "true") {
			return pod, false, nil
		}
	} else if idx := getIndexOfEnv(allEnvs, envProfilerEnabled); idx > -1 && !strings.EqualFold(strings.TrimSpace(allEnvs[idx].Value), "true") {
		return pod, false, nil
	}
	// ADOT sends gRPC when the OTLP protocol is grpc, which the HTTP endpoint does not accept. container.Env
	// already holds the Instrumentation's env (injected before this) and allEnvs adds envFrom. A valueFrom
	// protocol is treated as gRPC. The -Dotel.exporter.otlp.protocol system property (which ADOT reads first)
	// is checked only in JAVA_TOOL_OPTIONS, not in the command, args or JDK_JAVA_OPTIONS.
	if !envSetByCustomer(container, allEnvs, envProfilerEndpoint) {
		idx := getIndexOfEnv(container.Env, envOTelExporterProtocol)
		protocol := getEnvValue(allEnvs, envOTelExporterProtocol)
		unknown := false
		if idx > -1 {
			protocol, unknown = container.Env[idx].Value, container.Env[idx].ValueFrom != nil
		}
		toolOptions := strings.ToLower(getEnvValue(container.Env, envJavaToolsOptions) + " " + getEnvValue(allEnvs, envJavaToolsOptions))
		if unknown || strings.EqualFold(strings.TrimSpace(protocol), "grpc") ||
			strings.Contains(toolOptions, "-dotel.exporter.otlp.protocol=grpc") {
			return pod, false, fmt.Errorf("the container may use OTLP gRPC (%s or -Dotel.exporter.otlp.protocol), which the HTTP profiles endpoint does not accept; set %s to profile it", envOTelExporterProtocol, envProfilerEndpoint)
		}
	}
	// Never let the injected volume or mount make the pod invalid: skip this container instead.
	if volumeClash {
		return pod, false, fmt.Errorf("the pod already has a volume named %s", javaProfilerVolumeName)
	}
	for _, mount := range container.VolumeMounts {
		if mount.Name == javaProfilerVolumeName || strings.TrimRight(mount.MountPath, "/") == javaProfilerMountPath {
			return pod, false, fmt.Errorf("the container already mounts %s or %s", javaProfilerVolumeName, javaProfilerMountPath)
		}
	}

	envs := []corev1.EnvVar{
		{Name: envProfilerEnabled, Value: "true"},
		{Name: envProfilerEndpoint, Value: cfg.endpoint},
		{Name: envProfilerDataDir, Value: javaProfilerMountPath + "/" + container.Name},
		{Name: envProfilerMemoryEnabled, Value: defaultProfilerMemoryEnabled},
	}
	// The payload cap belongs to the chart's receiver: skip it when the container sends to its own endpoint.
	if cfg.maxPayloadBytes != "" && !envSetByCustomer(container, allEnvs, envProfilerEndpoint) {
		envs = append(envs, corev1.EnvVar{Name: envProfilerMaxPayloadBytes, Value: cfg.maxPayloadBytes})
	}
	for _, env := range envs {
		if !envSetByCustomer(container, allEnvs, env.Name) {
			container.Env = append(container.Env, env)
		}
	}
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: javaProfilerVolumeName, MountPath: javaProfilerMountPath})
	// One volume per pod, sized defaultProfilerVolumeSize per profiled container: each JVM has its own 128 MiB JFR budget.
	// A volume present here was added by this function for an earlier container (volumeClash covers the rest).
	for i := range pod.Spec.Volumes {
		if v := &pod.Spec.Volumes[i]; v.Name == javaProfilerVolumeName && v.EmptyDir != nil && v.EmptyDir.SizeLimit != nil {
			v.EmptyDir.SizeLimit.Add(defaultProfilerVolumeSize)
			return pod, true, nil
		}
	}
	size := defaultProfilerVolumeSize.DeepCopy()
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name:         javaProfilerVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &size}},
	})
	return pod, true, nil
}
