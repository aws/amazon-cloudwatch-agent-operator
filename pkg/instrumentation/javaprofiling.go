// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Continuous Java profiling with the async-profiler that the ADOT Java agent embeds
// (v2.32.0 and later). The operator enables the profiler, points it at the agent, gives it a dedicated
// disk-backed emptyDir for its data dir (the native library and rotating JFR files), and maps the chart's
// java.profiling settings onto ADOT's OTEL_AWS_PROFILER_* variables. The agent's opentelemetry.collect.otlp
// receiver on 4318 feeds the agent's profiles pipeline.
const (
	// annotationJavaProfiling set to "false" on a pod opts it out of profiling.
	annotationJavaProfiling = "cloudwatch.aws.amazon.com/java-profiling"

	envJavaProfilingEnabled  = "AUTO_INSTRUMENTATION_JAVA_PROFILING_ENABLED"
	envJavaProfilingSettings = "AUTO_INSTRUMENTATION_JAVA_PROFILING_SETTINGS"
	envProfilerPrefix        = "OTEL_AWS_PROFILER_"

	envProfilerEnabled      = "OTEL_AWS_PROFILER_ENABLED"
	envProfilerEndpoint     = "OTEL_AWS_PROFILER_ENDPOINT"
	envProfilerDataDir      = "OTEL_AWS_PROFILER_DATA_DIR"
	envOTelExporterProtocol = "OTEL_EXPORTER_OTLP_PROTOCOL"

	// The agent's plain-HTTP OTLP receiver; the Application Signals receiver (4316) has no profiles pipeline.
	javaProfilerEndpoint = "http://cloudwatch-agent.amazon-cloudwatch:4318/v1development/profiles"

	// javaProfilerVolumeName is the pod-level emptyDir that holds every profiled container's data dir. It is
	// disk-backed (no medium: Memory), so it is writable with readOnlyRootFilesystem and isolated from the
	// javaagent volume; its sizeLimit is javaProfilingConfig.volumeSize.
	javaProfilerVolumeName = "opentelemetry-profiler-data-java"
	// javaProfilerMountPath is where the volume is mounted in each profiled Java container, and the default
	// data_dir base: each container uses <base>/<container-name>.
	javaProfilerMountPath = "/otel-profiler-data-java"
)

// Reserved java.profiling keys (compared upper-cased): explicit handling, never mapped to OTEL_AWS_PROFILER_<KEY>.
const (
	profilingKeyEnabled         = "ENABLED"           // the gate; main.go copies it to envJavaProfilingEnabled
	profilingKeyEndpoint        = "ENDPOINT"          // rejected: profiles always go to the agent
	profilingKeyDataDir         = "DATA_DIR"          // a base path; the operator appends /<container-name>
	profilingKeyVolumeSizeLimit = "VOLUME_SIZE_LIMIT" // the volume's sizeLimit, never an env var
)

var (
	// javaProfilingKey is the strict form a java.profiling key must have to become an env var name.
	javaProfilingKey = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	// javaProfilingValue rejects control characters, which ADOT would echo into the pod's log.
	javaProfilingValue = regexp.MustCompile(`^[^\x00-\x1f\x7f]+$`)
	// javaProfilingPathSegment keeps a container name safe as one path segment (container names are DNS
	// labels already; this is defensive).
	javaProfilingPathSegment = regexp.MustCompile(`[^a-z0-9-]`)
	// adotProfilerSettings are the OTEL_AWS_PROFILER_* names ADOT v2.32.0 reads that the generic mapping may set
	// (ENABLED, ENDPOINT and DATA_DIR are reserved). Other names are still injected (ADOT ignores them) but
	// reported, to catch typos.
	adotProfilerSettings = map[string]bool{
		"MODE": true, "WALL_INTERVAL_MS": true, "CPU_INTERVAL_MS": true, "MEMORY_ENABLED": true,
		"ALLOC_INTERVAL_BYTES": true, "AGGREGATION_MODE": true, "WINDOW_SECONDS": true,
		"EXPORT_COMPRESSION": true, "EXPORT_TIMEOUT_MS": true, "MAX_PAYLOAD_BYTES": true,
	}
	// minProfilerVolumeSize is the smallest accepted volume_size_limit: below it ADOT's native library and first
	// window would overflow the volume and get every profiled pod evicted (a typo such as "160m" is 0.16 bytes).
	minProfilerVolumeSize   = resource.MustParse("16Mi")
	javaProfilingConfigOnce sync.Once
	javaProfilingConfigData javaProfilingConfig
)

// javaProfilingConfig is the parsed java.profiling block, other than enabled.
type javaProfilingConfig struct {
	// settings are the generic OTEL_AWS_PROFILER_<KEY> variables, sorted by name.
	settings []corev1.EnvVar
	// dataDirBase is the data_dir base; each container's OTEL_AWS_PROFILER_DATA_DIR is <base>/<container-name>.
	dataDirBase string
	// volumeSize is the profiler volume's sizeLimit.
	volumeSize resource.Quantity
	// notes lists one line per skipped, rejected or unknown key, without values.
	notes []string
}

// javaProfilingEnabled reports whether main.go copied java.profiling.enabled "true" from
// --auto-instrumentation-config.
func javaProfilingEnabled() bool {
	// Exactly "true", the same rule the chart uses to add the agent's 4318 receiver.
	return os.Getenv(envJavaProfilingEnabled) == "true"
}

// The rule, as documented in the chart README: when java.profiling.enabled is "true", the reserved keys
// enabled, endpoint, data_dir and volume_size_limit have explicit meanings, and every other key under
// java.profiling becomes OTEL_AWS_PROFILER_<KEY upper-cased>. A variable already set on the container wins.
//
// parseJavaProfilingConfig parses the java.profiling keys other than enabled, as copied by main.go as JSON.
// Reserved keys, compared after upper-casing:
//   - ENABLED is ignored here (main.go handles it);
//   - ENDPOINT, and any name containing ENDPOINT, is rejected, so profiles always go to the agent Service;
//   - DATA_DIR must be an absolute path without control characters or a ".." segment, else the default base is used;
//   - VOLUME_SIZE_LIMIT must parse as a resource.Quantity of whole bytes, at least 16Mi, else the default size
//     is used. It is the size per profiled container: the pod's volume gets it once for each.
//
// Generic keys: a key that is not letters, digits and underscores, an empty value (a non-string chart value
// also arrives empty), a value with control characters, or a second key with the same upper-cased name is
// skipped. A name ADOT v2.32.0 does not read is injected and reported.
func parseJavaProfilingConfig(raw string) javaProfilingConfig {
	cfg := javaProfilingConfig{dataDirBase: javaProfilerMountPath, volumeSize: defaultProfilerVolumeSize.DeepCopy()}
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
	seen := map[string]bool{}
	for _, key := range keys {
		value := values[key]
		name := strings.ToUpper(key)
		switch {
		case !javaProfilingKey.MatchString(key):
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: only letters, digits and underscores", key))
		case name == profilingKeyEnabled:
			// main.go reads only the lower-case key as the switch.
			if key != "enabled" {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: ignored, the switch is the lower-case key \"enabled\"", key))
			}
		case strings.Contains(name, profilingKeyEndpoint):
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: not settable from the chart, profiles always go to the agent", key))
		case seen[name]:
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: duplicates %s", key, name))
		case value == "":
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: empty value (chart values must be strings)", key))
		case !javaProfilingValue.MatchString(value):
			cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: value has control characters", key))
		case name == profilingKeyDataDir:
			seen[name] = true
			if !strings.HasPrefix(value, "/") || hasDotDotSegment(value) {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: must be an absolute path without a \"..\" segment; using %s", key, javaProfilerMountPath))
				continue
			}
			cfg.dataDirBase = value
		case name == profilingKeyVolumeSizeLimit:
			seen[name] = true
			quantity, err := resource.ParseQuantity(value)
			_, wholeBytes := quantity.AsInt64()
			if err != nil || !wholeBytes || quantity.Cmp(minProfilerVolumeSize) < 0 {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: not a whole number of bytes of at least %s; using %s", key, minProfilerVolumeSize.String(), defaultProfilerVolumeSize.String()))
				continue
			}
			cfg.volumeSize = quantity
		default:
			seen[name] = true
			cfg.settings = append(cfg.settings, corev1.EnvVar{Name: envProfilerPrefix + name, Value: value})
			if !adotProfilerSettings[name] {
				cfg.notes = append(cfg.notes, fmt.Sprintf("key %q: injected, but ADOT v2.32.0 does not read %s%s", key, envProfilerPrefix, name))
			}
		}
	}
	sort.SliceStable(cfg.settings, func(a, b int) bool { return cfg.settings[a].Name < cfg.settings[b].Name })
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

// hasDotDotSegment reports whether path has a ".." segment ("/a/../b"), not just ".." inside a name ("/a..b").
func hasDotDotSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// javaProfilerDataDir returns <base>/<container-name>, with the container name reduced to one safe segment.
func javaProfilerDataDir(base, containerName string) string {
	segment := javaProfilingPathSegment.ReplaceAllString(strings.ToLower(containerName), "-")
	if segment == "" {
		segment = "container"
	}
	return strings.TrimRight(base, "/") + "/" + segment
}

// injectJavaProfiling adds OTEL_AWS_PROFILER_ENABLED, OTEL_AWS_PROFILER_ENDPOINT, OTEL_AWS_PROFILER_DATA_DIR and the
// chart's generic settings to the Java container at index after the javaagent was injected, mounts the profiler
// volume in it, and adds the volume to the pod once, growing its sizeLimit by cfg.volumeSize per profiled container. A variable the container already sets wins; a customer
// DATA_DIR is used verbatim. volumeClash is javaProfilerVolumePresent of the pod as it was before any injection,
// a customer volume with the profiler's name. It returns whether it injected.
func injectJavaProfiling(enabled bool, cfg javaProfilingConfig, volumeClash bool, ns corev1.Namespace, pod corev1.Pod, index int, allEnvs []corev1.EnvVar) (corev1.Pod, bool, error) {
	if !enabled {
		return pod, false, nil
	}
	// The pod annotation wins over the namespace one, as for the inject-* annotations.
	if strings.EqualFold(strings.TrimSpace(annotationValue(ns.ObjectMeta, pod.ObjectMeta, annotationJavaProfiling)), "false") {
		return pod, false, nil
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
	// allEnvs resolves only ConfigMaps and ignores prefixes, so a Secret or a prefixed source could hold a
	// profiler variable the operator cannot see; injecting would override the customer's value.
	for _, from := range container.EnvFrom {
		if from.SecretRef != nil || from.Prefix != "" {
			return pod, false, fmt.Errorf("the container has an envFrom Secret or prefixed source, which may set OTEL_AWS_PROFILER_* variables the operator cannot read")
		}
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
			return pod, false, fmt.Errorf("the container may use OTLP gRPC (%s or -Dotel.exporter.otlp.protocol), which %s does not accept; set %s to profile it", envOTelExporterProtocol, javaProfilerEndpoint, envProfilerEndpoint)
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
		{Name: envProfilerEndpoint, Value: javaProfilerEndpoint},
		{Name: envProfilerDataDir, Value: javaProfilerDataDir(cfg.dataDirBase, container.Name)},
	}
	for _, setting := range cfg.settings {
		if setting.Name != envProfilerEnabled && setting.Name != envProfilerDataDir && !strings.Contains(setting.Name, "ENDPOINT") {
			envs = append(envs, setting)
		}
	}
	for _, env := range envs {
		if !envSetByCustomer(container, allEnvs, env.Name) {
			container.Env = append(container.Env, env)
		}
	}
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{Name: javaProfilerVolumeName, MountPath: javaProfilerMountPath})
	// One volume per pod, sized volumeSize per profiled container: each JVM has its own 128 MiB JFR budget.
	// A volume present here was added by this function for an earlier container (volumeClash covers the rest).
	for i := range pod.Spec.Volumes {
		if v := &pod.Spec.Volumes[i]; v.Name == javaProfilerVolumeName && v.EmptyDir != nil && v.EmptyDir.SizeLimit != nil {
			v.EmptyDir.SizeLimit.Add(cfg.volumeSize)
			return pod, true, nil
		}
	}
	size := cfg.volumeSize.DeepCopy()
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name:         javaProfilerVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &size}},
	})
	return pod, true, nil
}
