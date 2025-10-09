// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/constants"
)

func TestInitContainerMissing(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		expected bool
	}{
		{
			name: "InitContainer_Already_Inject",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "istio-init",
						},
						{
							Name: javaInitContainerName,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "InitContainer_Absent_1",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "istio-init",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "InitContainer_Absent_2",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isInitContainerMissing(test.pod, javaInitContainerName)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestAutoInstrumentationInjected(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		expected bool
	}{
		{
			name: "AutoInstrumentation_Already_Inject",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "magic-init",
						},
						{
							Name: nodejsInitContainerName,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "AutoInstrumentation_Already_Inject_go",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
					Containers: []corev1.Container{
						{
							Name: sideCarName,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "AutoInstrumentation_Already_Inject_no_init_containers",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
					Containers: []corev1.Container{
						{
							Name: "my-app",
							Env: []corev1.EnvVar{
								{
									Name:  constants.EnvNodeName,
									Value: "value",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "AutoInstrumentation_Absent_1",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "magic-init",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "AutoInstrumentation_Absent_2",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isAutoInstrumentationInjected(test.pod)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestDuplicatedContainers(t *testing.T) {
	tests := []struct {
		name               string
		containers         []string
		expectedDuplicates error
	}{
		{
			name:               "No duplicates",
			containers:         []string{"app1,app2", "app3", "app4,app5"},
			expectedDuplicates: nil,
		},
		{
			name:               "Duplicates in containers",
			containers:         []string{"app1,app2", "app1", "app1,app3,app4", "app4"},
			expectedDuplicates: fmt.Errorf("duplicated container names detected: [app1 app4]"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ok := findDuplicatedContainers(test.containers)
			assert.Equal(t, test.expectedDuplicates, ok)
		})
	}
}

func TestInstrWithContainers(t *testing.T) {
	tests := []struct {
		name           string
		containers     instrumentationWithContainers
		expectedResult int
	}{
		{
			name:           "No containers",
			containers:     instrumentationWithContainers{Containers: ""},
			expectedResult: 0,
		},
		{
			name:           "With containers",
			containers:     instrumentationWithContainers{Containers: "ct1"},
			expectedResult: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := isInstrWithContainers(test.containers)
			assert.Equal(t, test.expectedResult, res)
		})
	}
}

func TestInstrWithoutContainers(t *testing.T) {
	tests := []struct {
		name           string
		containers     instrumentationWithContainers
		expectedResult int
	}{
		{
			name:           "No containers",
			containers:     instrumentationWithContainers{Containers: ""},
			expectedResult: 1,
		},
		{
			name:           "With containers",
			containers:     instrumentationWithContainers{Containers: "ct1"},
			expectedResult: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := isInstrWithoutContainers(test.containers)
			assert.Equal(t, test.expectedResult, res)
		})
	}
}

// TestContainsCloudWatchAgent tests CloudWatch agent hostname matching
func TestContainsCloudWatchAgent(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected bool
	}{
		{
			name:     "Valid: Standard CloudWatch agent endpoint",
			endpoint: "http://cloudwatch-agent.amazon-cloudwatch:4316",
			expected: true,
		},
		{
			name:     "Valid: grpc protocol",
			endpoint: "grpc://cloudwatch-agent.amazon-cloudwatch:4316",
			expected: true,
		},
		{
			name:     "Valid: https protocol",
			endpoint: "https://cloudwatch-agent.amazon-cloudwatch:4316",
			expected: true,
		},
		{
			name:     "Valid: Windows headless service",
			endpoint: "http://cloudwatch-agent-windows-headless.amazon-cloudwatch.svc.cluster.local:4316",
			expected: true,
		},
		{
			name:     "Valid: Custom port",
			endpoint: "http://cloudwatch-agent.amazon-cloudwatch:8080",
			expected: true,
		},
		{
			name:     "Invalid: CloudWatch agent in URL path",
			endpoint: "http://custom:4318/cloudwatch-agent.amazon-cloudwatch",
			expected: false,
		},
		{
			name:     "Invalid: Prefix before hostname",
			endpoint: "http://my-cloudwatch-agent.amazon-cloudwatch:4316",
			expected: false,
		},
		{
			name:     "Invalid: Custom endpoint",
			endpoint: "http://custom-collector:4318",
			expected: false,
		},
		{
			name:     "Invalid: No protocol separator",
			endpoint: "cloudwatch-agent.amazon-cloudwatch:4316",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := containsCloudWatchAgent(test.endpoint)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestIsApplicationSignalsExplicitlyEnabled tests Application Signals enabled detection
func TestIsApplicationSignalsExplicitlyEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envs     []corev1.EnvVar
		expected bool
	}{
		{
			name:     "Enabled: lowercase true",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"}},
			expected: true,
		},
		{
			name:     "Enabled: uppercase TRUE",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "TRUE"}},
			expected: true,
		},
		{
			name:     "Enabled: mixed case True",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "True"}},
			expected: true,
		},
		{
			name:     "Not enabled: false",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "false"}},
			expected: false,
		},
		{
			name:     "Not enabled: not set",
			envs:     []corev1.EnvVar{},
			expected: false,
		},
		{
			name:     "Not enabled: empty string",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: ""}},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isApplicationSignalsExplicitlyEnabled(test.envs)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestIsApplicationSignalsExplicitlyDisabled tests Application Signals disabled detection
func TestIsApplicationSignalsExplicitlyDisabled(t *testing.T) {
	tests := []struct {
		name     string
		envs     []corev1.EnvVar
		expected bool
	}{
		{
			name:     "Disabled: lowercase false",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "false"}},
			expected: true,
		},
		{
			name:     "Disabled: uppercase FALSE",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "FALSE"}},
			expected: true,
		},
		{
			name:     "Not disabled: true",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"}},
			expected: false,
		},
		{
			name:     "Not disabled: not set",
			envs:     []corev1.EnvVar{},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isApplicationSignalsExplicitlyDisabled(test.envs)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestShouldInjectEnvVar tests environment variable injection decision
func TestShouldInjectEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envs     []corev1.EnvVar
		envName  string
		expected bool
	}{
		{
			name:     "Application Signals disabled: Skip OTEL_ var",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "false"}},
			envName:  "OTEL_METRICS_EXPORTER",
			expected: false,
		},
		{
			name:     "Application Signals disabled: Skip non-OTEL var",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "false"}},
			envName:  "MY_CUSTOM_VAR",
			expected: false,
		},
		{
			name:     "Application Signals enabled: Inject OTEL_ var",
			envs:     []corev1.EnvVar{{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"}},
			envName:  "OTEL_METRICS_EXPORTER",
			expected: true,
		},
		{
			name:     "Application Signals not set: Inject OTEL_ var",
			envs:     []corev1.EnvVar{},
			envName:  "OTEL_TRACES_SAMPLER",
			expected: true,
		},
		{
			name:     "User already set: Skip injection",
			envs:     []corev1.EnvVar{{Name: "OTEL_METRICS_EXPORTER", Value: "otlp"}},
			envName:  "OTEL_METRICS_EXPORTER",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := shouldInjectEnvVar(test.envs, test.envName)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestShouldInjectADOTSDK tests SDK injection decision based on SecurityContext and endpoints
func TestShouldInjectADOTSDK(t *testing.T) {
	tests := []struct {
		name      string
		envs      []corev1.EnvVar
		pod       corev1.Pod
		container *corev1.Container
		expected  bool
	}{
		{
			name:      "No constraints: Inject",
			envs:      []corev1.EnvVar{},
			pod:       corev1.Pod{},
			container: &corev1.Container{},
			expected:  true,
		},
		{
			name: "Pod runAsNonRoot=true without runAsUser: Skip",
			envs: []corev1.EnvVar{},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: func() *bool { b := true; return &b }(),
					},
				},
			},
			container: &corev1.Container{},
			expected:  false,
		},
		{
			name: "Pod runAsNonRoot=true with runAsUser: Inject",
			envs: []corev1.EnvVar{},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: func() *bool { b := true; return &b }(),
						RunAsUser:    func() *int64 { i := int64(1000); return &i }(),
					},
				},
			},
			container: &corev1.Container{},
			expected:  true,
		},
		{
			name: "Container runAsNonRoot=true without runAsUser: Skip",
			envs: []corev1.EnvVar{},
			pod:  corev1.Pod{},
			container: &corev1.Container{
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: func() *bool { b := true; return &b }(),
				},
			},
			expected: false,
		},
		{
			name: "Container runAsNonRoot=true, pod has runAsUser: Inject",
			envs: []corev1.EnvVar{},
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: func() *int64 { i := int64(1000); return &i }(),
					},
				},
			},
			container: &corev1.Container{
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: func() *bool { b := true; return &b }(),
				},
			},
			expected: true,
		},
		{
			name: "Application Signals enabled + custom endpoint: Inject",
			envs: []corev1.EnvVar{
				{Name: "OTEL_AWS_APPLICATION_SIGNALS_ENABLED", Value: "true"},
				{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://custom:4318"},
			},
			pod:       corev1.Pod{},
			container: &corev1.Container{},
			expected:  true,
		},
		{
			name: "Application Signals not set + custom endpoint: Skip",
			envs: []corev1.EnvVar{
				{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://custom:4318"},
			},
			pod:       corev1.Pod{},
			container: &corev1.Container{},
			expected:  false,
		},
		{
			name: "CloudWatch endpoint: Inject",
			envs: []corev1.EnvVar{
				{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316"},
			},
			pod:       corev1.Pod{},
			container: &corev1.Container{},
			expected:  true,
		},
		{
			name: "Custom traces endpoint + CloudWatch metrics: Skip",
			envs: []corev1.EnvVar{
				{Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", Value: "http://custom:4318"},
				{Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", Value: "http://cloudwatch-agent.amazon-cloudwatch:4316"},
			},
			pod:       corev1.Pod{},
			container: &corev1.Container{},
			expected:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := shouldInjectADOTSDK(test.envs, test.pod, test.container)
			assert.Equal(t, test.expected, result)
		})
	}
}
