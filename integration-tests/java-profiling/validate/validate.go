// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package validate checks what the operator injects into a pod for Java profiling: the OTEL_AWS_PROFILER_*
// variables of each profiled container, the shared profiler emptyDir and its mounts, and that other containers
// get none of it.
package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// ProfilerVolumeName and ProfilerMountPath are the operator's profiler volume and its mount path.
	ProfilerVolumeName = "opentelemetry-profiler-data-java"
	ProfilerMountPath  = "/otel-profiler-data-java"
	profilerEnvPrefix  = "OTEL_AWS_PROFILER_"
)

// Expectations is one expectations file, for the pods of one label selector.
type Expectations struct {
	// LabelSelector selects the pods to check.
	LabelSelector string `json:"labelSelector"`
	// JavaInstrumentedContainers must carry the javaagent in JAVA_TOOL_OPTIONS, so that an opt-out or unprofiled
	// check cannot pass only because instrumentation did not happen at all.
	JavaInstrumentedContainers []string `json:"javaInstrumentedContainers"`
	// ProfiledContainers maps a container name to its exact OTEL_AWS_PROFILER_* variables: each set once, with that
	// value, and no other OTEL_AWS_PROFILER_* variable. The container must also mount the volume.
	ProfiledContainers map[string]map[string]string `json:"profiledContainers"`
	// UnprofiledContainers must have no OTEL_AWS_PROFILER_* variable and no profiler mount, apart from the customer's
	// own variables listed in UnprofiledAllowedEnv, which must keep exactly that value.
	UnprofiledContainers []string `json:"unprofiledContainers"`
	// UnprofiledAllowedEnv lists customer-set OTEL_AWS_PROFILER_* variables an unprofiled container keeps, for example
	// the opt-out OTEL_AWS_PROFILER_ENABLED=false.
	UnprofiledAllowedEnv map[string]string `json:"unprofiledAllowedEnv"`
	// VolumeSizeLimit is the profiler volume's expected sizeLimit; empty means the pod must have no profiler volume.
	VolumeSizeLimit string `json:"volumeSizeLimit"`
}

// Load reads an expectations file.
func Load(path string) (Expectations, error) {
	var exp Expectations
	data, err := os.ReadFile(path)
	if err != nil {
		return exp, err
	}
	if err := json.Unmarshal(data, &exp); err != nil {
		return exp, fmt.Errorf("parsing %s: %w", path, err)
	}
	if exp.LabelSelector == "" {
		return exp, fmt.Errorf("%s: labelSelector is required", path)
	}
	if len(exp.JavaInstrumentedContainers) == 0 || (len(exp.ProfiledContainers) == 0 && len(exp.UnprofiledContainers) == 0) {
		return exp, fmt.Errorf("%s: expectations check nothing", path)
	}
	if (exp.VolumeSizeLimit == "") != (len(exp.ProfiledContainers) == 0) {
		return exp, fmt.Errorf("%s: volumeSizeLimit must be set exactly when profiledContainers is not empty", path)
	}
	return exp, nil
}

// Pod returns one line per mismatch between pod and exp; an empty result means the pod passes.
func Pod(pod corev1.Pod, exp Expectations) []string {
	var problems []string
	fail := func(format string, args ...interface{}) { problems = append(problems, fmt.Sprintf(format, args...)) }

	containers := map[string]corev1.Container{}
	for _, c := range pod.Spec.Containers {
		containers[c.Name] = c
	}
	lookup := func(name string) (corev1.Container, bool) {
		c, ok := containers[name]
		if !ok {
			fail("container %q: not in the pod", name)
		}
		return c, ok
	}

	for _, name := range exp.JavaInstrumentedContainers {
		if c, ok := lookup(name); ok && !strings.Contains(envValue(c, "JAVA_TOOL_OPTIONS"), "-javaagent:") {
			fail("container %q: not Java-instrumented (no -javaagent in JAVA_TOOL_OPTIONS)", name)
		}
	}

	names := make([]string, 0, len(exp.ProfiledContainers))
	for name := range exp.ProfiledContainers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c, ok := lookup(name)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(exp.ProfiledContainers[name]))
		for key := range exp.ProfiledContainers[name] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			want := exp.ProfiledContainers[name][key]
			if got := envValues(c, key); len(got) == 0 {
				fail("container %q: %s is not set, want %q", name, key, want)
			} else if len(got) > 1 {
				fail("container %q: %s is set %d times %q, want once", name, key, len(got), got)
			} else if got[0] != want {
				fail("container %q: %s=%q, want %q", name, key, got[0], want)
			}
		}
		for _, env := range c.Env {
			if _, expected := exp.ProfiledContainers[name][env.Name]; strings.HasPrefix(env.Name, profilerEnvPrefix) && !expected {
				fail("container %q: unexpected %s=%q", name, env.Name, env.Value)
			}
		}
		if mounts := profilerMounts(c); len(mounts) != 1 || mounts[0].Name != ProfilerVolumeName ||
			mounts[0].MountPath != ProfilerMountPath || mounts[0].ReadOnly || mounts[0].SubPath != "" {
			fail("container %q: want one read-write mount of %s at %s, got %v", name, ProfilerVolumeName, ProfilerMountPath, mounts)
		}
	}

	for _, name := range exp.UnprofiledContainers {
		c, ok := lookup(name)
		if !ok {
			continue
		}
		for _, env := range c.Env {
			if !strings.HasPrefix(env.Name, profilerEnvPrefix) {
				continue
			}
			if want, allowed := exp.UnprofiledAllowedEnv[env.Name]; !allowed {
				fail("container %q: unexpected %s", name, env.Name)
			} else if env.Value != want {
				fail("container %q: %s=%q, want the customer's %q", name, env.Name, env.Value, want)
			}
		}
		for key, want := range exp.UnprofiledAllowedEnv {
			if got := envValues(c, key); len(got) != 1 {
				fail("container %q: %s set %d times, want the customer's single %q", name, key, len(got), want)
			}
		}
		if mounts := profilerMounts(c); len(mounts) > 0 {
			fail("container %q: unexpected profiler mount %v", name, mounts)
		}
	}

	// The javaagent init container (and any other) never gets profiler variables or mounts.
	for _, c := range pod.Spec.InitContainers {
		for _, env := range c.Env {
			if strings.HasPrefix(env.Name, profilerEnvPrefix) {
				fail("init container %q: unexpected %s", c.Name, env.Name)
			}
		}
		if mounts := profilerMounts(c); len(mounts) > 0 {
			fail("init container %q: unexpected profiler mount %v", c.Name, mounts)
		}
	}

	var volumes []corev1.Volume
	for _, v := range pod.Spec.Volumes {
		if v.Name == ProfilerVolumeName {
			volumes = append(volumes, v)
		}
	}
	switch {
	case exp.VolumeSizeLimit == "" && len(volumes) > 0:
		fail("unexpected volume %s", ProfilerVolumeName)
	case exp.VolumeSizeLimit == "":
	case len(volumes) != 1:
		fail("want exactly one volume %s, got %d", ProfilerVolumeName, len(volumes))
	case volumes[0].EmptyDir == nil:
		fail("volume %s: not an emptyDir", ProfilerVolumeName)
	case volumes[0].EmptyDir.Medium != corev1.StorageMediumDefault:
		fail("volume %s: medium %q, want disk-backed", ProfilerVolumeName, volumes[0].EmptyDir.Medium)
	case volumes[0].EmptyDir.SizeLimit == nil:
		fail("volume %s: no sizeLimit, want %s", ProfilerVolumeName, exp.VolumeSizeLimit)
	default:
		want, err := resource.ParseQuantity(exp.VolumeSizeLimit)
		if err != nil {
			fail("volumeSizeLimit %q: %v", exp.VolumeSizeLimit, err)
		} else if volumes[0].EmptyDir.SizeLimit.Cmp(want) != 0 {
			fail("volume %s: sizeLimit %s, want %s", ProfilerVolumeName, volumes[0].EmptyDir.SizeLimit.String(), want.String())
		}
	}
	return problems
}

// envValues returns every value the container sets for name, in order (the last one is effective).
func envValues(c corev1.Container, name string) []string {
	var values []string
	for _, env := range c.Env {
		if env.Name == name {
			values = append(values, env.Value)
		}
	}
	return values
}

func envValue(c corev1.Container, name string) string {
	values := envValues(c, name)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

// profilerMounts returns the container's mounts of the profiler volume or of anything at the profiler mount path.
func profilerMounts(c corev1.Container) []corev1.VolumeMount {
	var mounts []corev1.VolumeMount
	for _, m := range c.VolumeMounts {
		if m.Name == ProfilerVolumeName || strings.TrimRight(m.MountPath, "/") == ProfilerMountPath {
			mounts = append(mounts, m)
		}
	}
	return mounts
}
