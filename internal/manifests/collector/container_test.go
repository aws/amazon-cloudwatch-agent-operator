// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"

	"github.com/stretchr/testify/assert"
)

var metricContainerPort = corev1.ContainerPort{
	Name:          "metrics",
	ContainerPort: 8888,
	Protocol:      corev1.ProtocolTCP,
}

var emfContainerPort = []corev1.ContainerPort{
	{
		Name:          "emf-tcp",
		ContainerPort: 25888,
		Protocol:      corev1.ProtocolTCP,
	},
	{
		Name:          "emf-udp",
		ContainerPort: 25888,
		Protocol:      corev1.ProtocolUDP,
	},
}

func TestGetVolumeMounts(t *testing.T) {
	volumeMount := getVolumeMounts("windows")
	assert.Equal(t, volumeMount.MountPath, "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\cwagentconfig")

	volumeMount = getVolumeMounts("linux")
	assert.Equal(t, volumeMount.MountPath, "/etc/cwagentconfig")

	volumeMount = getVolumeMounts("")
	assert.Equal(t, volumeMount.MountPath, "/etc/cwagentconfig")
}

func TestContainerPorts(t *testing.T) {
	var sampleJSONConfig = `{
	  "logs": {
		"metrics_collected": {
		  "emf": {}
		}
	  }
	}`

	var goodOtelConfig = `receivers:
  examplereceiver:
    endpoint: "0.0.0.0:12345"
exporters:
  debug:
service:
  pipelines:
    metrics:
      receivers: [examplereceiver]
      exporters: [debug]`

	tests := []struct {
		description   string
		specConfig    string
		specPorts     []corev1.ServicePort
		expectedPorts []corev1.ContainerPort
	}{
		{
			description:   "bad otel spec config",
			specConfig:    "🦄",
			specPorts:     nil,
			expectedPorts: emfContainerPort,
		},
		{
			description: "couldn't build ports from spec config",
			specConfig:  "",
			specPorts: []corev1.ServicePort{
				{
					Name:     "metrics",
					Port:     8888,
					Protocol: corev1.ProtocolTCP,
				},
			},
			expectedPorts: append(emfContainerPort, metricContainerPort),
		},
		{
			description: "ports in spec Config",
			specConfig:  goodOtelConfig,
			specPorts:   nil,
			expectedPorts: append(emfContainerPort, corev1.ContainerPort{
				Name:          "examplereceiver",
				ContainerPort: 12345,
			}),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.description, func(t *testing.T) {
			// prepare
			otelcol := v1alpha1.AmazonCloudWatchAgent{
				Spec: v1alpha1.AmazonCloudWatchAgentSpec{
					Config:     sampleJSONConfig,
					OtelConfig: testCase.specConfig,
					Ports:      testCase.specPorts,
				},
			}

			cfg := config.New(config.WithCollectorImage("default-image"))

			// test
			c := Container(cfg, logger, otelcol, true)
			// verify
			assert.ElementsMatch(t, testCase.expectedPorts, c.Ports, testCase.description)
		})
	}
}

func TestContainerProbe(t *testing.T) {
	// prepare
	initialDelaySeconds := int32(10)
	timeoutSeconds := int32(11)
	periodSeconds := int32(12)
	successThreshold := int32(13)
	failureThreshold := int32(14)
	terminationGracePeriodSeconds := int64(15)
	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			OtelConfig: `extensions:
 health_check:
service:
 extensions: [health_check]`,
			LivenessProbe: &v1alpha1.Probe{
				InitialDelaySeconds:           &initialDelaySeconds,
				TimeoutSeconds:                &timeoutSeconds,
				PeriodSeconds:                 &periodSeconds,
				SuccessThreshold:              &successThreshold,
				FailureThreshold:              &failureThreshold,
				TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			},
		},
	}
	cfg := config.New()

	// test
	c := Container(cfg, logger, otelcol, true)

	// verify
	assert.Equal(t, "/", c.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(13133), c.LivenessProbe.HTTPGet.Port.IntVal)
	assert.Equal(t, "", c.LivenessProbe.HTTPGet.Host)

	assert.Equal(t, initialDelaySeconds, c.LivenessProbe.InitialDelaySeconds)
	assert.Equal(t, timeoutSeconds, c.LivenessProbe.TimeoutSeconds)
	assert.Equal(t, periodSeconds, c.LivenessProbe.PeriodSeconds)
	assert.Equal(t, successThreshold, c.LivenessProbe.SuccessThreshold)
	assert.Equal(t, failureThreshold, c.LivenessProbe.FailureThreshold)
	assert.Equal(t, terminationGracePeriodSeconds, *c.LivenessProbe.TerminationGracePeriodSeconds)
}

func TestContainerProbeEmptyConfig(t *testing.T) {
	// prepare

	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			OtelConfig: `extensions:
  health_check:
service:
  extensions: [health_check]`,
			LivenessProbe: &v1alpha1.Probe{},
		},
	}
	cfg := config.New()

	// test
	c := Container(cfg, logger, otelcol, true)

	// verify
	assert.Equal(t, "/", c.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(13133), c.LivenessProbe.HTTPGet.Port.IntVal)
	assert.Equal(t, "", c.LivenessProbe.HTTPGet.Host)
}

func TestContainerProbeNoConfig(t *testing.T) {
	// prepare

	otelcol := v1alpha1.AmazonCloudWatchAgent{
		Spec: v1alpha1.AmazonCloudWatchAgentSpec{
			OtelConfig: `extensions:
  health_check:
service:
  extensions: [health_check]`,
		},
	}
	cfg := config.New()

	// test
	c := Container(cfg, logger, otelcol, true)

	// verify
	assert.Equal(t, "/", c.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(13133), c.LivenessProbe.HTTPGet.Port.IntVal)
	assert.Equal(t, "", c.LivenessProbe.HTTPGet.Host)
}
