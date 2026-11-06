// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func profiledPod() corev1.Pod {
	size := resource.MustParse("64Mi")
	return corev1.Pod{Spec: corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "app", Env: []corev1.EnvVar{
				{Name: "JAVA_TOOL_OPTIONS", Value: " -javaagent:/otel-auto-instrumentation-java/javaagent.jar"},
				{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "true"},
			}, VolumeMounts: []corev1.VolumeMount{{Name: ProfilerVolumeName, MountPath: ProfilerMountPath}}},
			{Name: "sidecar"},
		},
		Volumes: []corev1.Volume{{Name: ProfilerVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &size}}}},
	}}
}

var profiledExp = Expectations{
	LabelSelector:              "app=x",
	JavaInstrumentedContainers: []string{"app"},
	ProfiledContainers:         map[string]map[string]string{"app": {"OTEL_AWS_PROFILER_ENABLED": "true"}},
	UnprofiledContainers:       []string{"sidecar"},
	VolumeSizeLimit:            "67108864",
}

func TestPodMatches(t *testing.T) {
	assert.Empty(t, Pod(profiledPod(), profiledExp))
}

func TestPodMismatches(t *testing.T) {
	tests := map[string]func(*corev1.Pod){
		"env missing":         func(p *corev1.Pod) { p.Spec.Containers[0].Env = p.Spec.Containers[0].Env[:1] },
		"env value differs":   func(p *corev1.Pod) { p.Spec.Containers[0].Env[1].Value = "false" },
		"not instrumented":    func(p *corev1.Pod) { p.Spec.Containers[0].Env[0].Value = "" },
		"mount missing":       func(p *corev1.Pod) { p.Spec.Containers[0].VolumeMounts = nil },
		"mount read-only":     func(p *corev1.Pod) { p.Spec.Containers[0].VolumeMounts[0].ReadOnly = true },
		"volume missing":      func(p *corev1.Pod) { p.Spec.Volumes = nil },
		"volume size differs": func(p *corev1.Pod) { q := resource.MustParse("128Mi"); p.Spec.Volumes[0].EmptyDir.SizeLimit = &q },
		"volume in memory":    func(p *corev1.Pod) { p.Spec.Volumes[0].EmptyDir.Medium = corev1.StorageMediumMemory },
		"sidecar gets env": func(p *corev1.Pod) {
			p.Spec.Containers[1].Env = []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_MODE", Value: "cpu"}}
		},
		"sidecar gets the mount": func(p *corev1.Pod) { p.Spec.Containers[1].VolumeMounts = p.Spec.Containers[0].VolumeMounts },
		"container missing":      func(p *corev1.Pod) { p.Spec.Containers = p.Spec.Containers[:1] },
		"extra profiler var": func(p *corev1.Pod) {
			p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, corev1.EnvVar{Name: "OTEL_AWS_PROFILER_VOLUME_SIZE_LIMIT", Value: "64Mi"})
		},
		"duplicate var": func(p *corev1.Pod) {
			p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, corev1.EnvVar{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "false"})
		},
		"init container gets env": func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "init", Env: []corev1.EnvVar{{Name: "OTEL_AWS_PROFILER_ENABLED", Value: "true"}}}}
		},
		"init container gets the mount": func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "init", VolumeMounts: p.Spec.Containers[0].VolumeMounts}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			pod := profiledPod()
			mutate(&pod)
			assert.NotEmpty(t, Pod(pod, profiledExp))
		})
	}
	// With no profiled containers, any profiler volume is a mismatch.
	optOut := Expectations{LabelSelector: "app=x", JavaInstrumentedContainers: []string{"app"}, UnprofiledContainers: []string{"app"}}
	assert.NotEmpty(t, Pod(profiledPod(), optOut))
}

func TestLoad(t *testing.T) {
	for _, file := range []string{"../java_profiling_expectations.json", "../java_profiling_opt_out_expectations.json"} {
		_, err := Load(file)
		assert.NoError(t, err, file)
	}
	// Files that would check nothing, or a volume size without profiled containers, are rejected.
	for name, body := range map[string]string{
		"no selector":         `{"javaInstrumentedContainers":["app"],"unprofiledContainers":["app"]}`,
		"checks nothing":      `{"labelSelector":"app=x","javaInstrumentedContainers":["app"]}`,
		"size without pods":   `{"labelSelector":"app=x","javaInstrumentedContainers":["app"],"unprofiledContainers":["app"],"volumeSizeLimit":"64Mi"}`,
		"pods without a size": `{"labelSelector":"app=x","javaInstrumentedContainers":["app"],"profiledContainers":{"app":{}}}`,
	} {
		path := filepath.Join(t.TempDir(), "exp.json")
		assert.NoError(t, os.WriteFile(path, []byte(body), 0o600))
		_, err := Load(path)
		assert.Error(t, err, name)
	}
}
