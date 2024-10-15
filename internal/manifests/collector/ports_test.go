// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestStatsDGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/statsDAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(8135), containerPorts[CWA+StatsD].ContainerPort)
	assert.Equal(t, CWA+StatsD, containerPorts[CWA+StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+StatsD].Protocol)
}

func TestDefaultStatsDGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/statsDDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(8125), containerPorts[StatsD].ContainerPort)
	assert.Equal(t, StatsD, containerPorts[StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[StatsD].Protocol)
}

func TestCollectDGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/collectDAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(25936), containerPorts[CWA+CollectD].ContainerPort)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+CollectD].Protocol)
}

func TestDefaultCollectDGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/collectDDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(25826), containerPorts[CollectD].ContainerPort)
	assert.Equal(t, CollectD, containerPorts[CollectD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CollectD].Protocol)
}

func TestApplicationSignals(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
}

func TestEMFGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/emfAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(25888), containerPorts[EMFTcp].ContainerPort)
	assert.Equal(t, EMFTcp, containerPorts[EMFTcp].Name)
	assert.Equal(t, int32(25888), containerPorts[EMFUdp].ContainerPort)
	assert.Equal(t, EMFUdp, containerPorts[EMFUdp].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[EMFUdp].Protocol)
}

func TestXrayAndOTLPGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/xrayAndOTLPAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(2000), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, int32(4327), containerPorts[CWA+OtlpGrpc].ContainerPort)
	assert.Equal(t, CWA+OtlpGrpc, containerPorts[CWA+OtlpGrpc].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+OtlpGrpc].Protocol)
	assert.Equal(t, int32(4328), containerPorts[CWA+OtlpHttp].ContainerPort)
	assert.Equal(t, CWA+OtlpHttp, containerPorts[CWA+OtlpHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+OtlpHttp].Protocol)
}

func TestDefaultXRayAndOTLPGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/xrayAndOTLPDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(2000), containerPorts[XrayTraces].ContainerPort)
	assert.Equal(t, XrayTraces, containerPorts[XrayTraces].Name)
	assert.Equal(t, int32(4317), containerPorts[OtlpGrpc].ContainerPort)
	assert.Equal(t, OtlpGrpc, containerPorts[OtlpGrpc].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[OtlpGrpc].Protocol)
	assert.Equal(t, int32(4318), containerPorts[OtlpHttp].ContainerPort)
	assert.Equal(t, OtlpHttp, containerPorts[OtlpHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[OtlpHttp].Protocol)
}

func TestXRayGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/xrayAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+XrayTraces].Protocol)
	assert.Equal(t, int32(2900), containerPorts[CWA+XrayProxy].ContainerPort)
	assert.Equal(t, CWA+XrayProxy, containerPorts[CWA+XrayProxy].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+XrayProxy].Protocol)
}

func TestXRayWithBindAddressDefaultGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/xrayAgentConfig.json")
	strings.Replace(cfg, "2800", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, int32(2900), containerPorts[CWA+XrayProxy].ContainerPort)
	assert.Equal(t, CWA+XrayProxy, containerPorts[CWA+XrayProxy].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+XrayProxy].Protocol)
}

func TestXRayWithTCPProxyBindAddressDefaultGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/xrayAgentConfig.json")
	strings.Replace(cfg, "2900", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+XrayTraces].Protocol)
}

func TestNilMetricsGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/nilMetrics.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 0, len(containerPorts))
}

func TestMultipleReceiversGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/multipleReceiversAgentConfig.json")
	strings.Replace(cfg, "2900", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 11, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
	assert.Equal(t, int32(8135), containerPorts[CWA+StatsD].ContainerPort)
	assert.Equal(t, CWA+StatsD, containerPorts[CWA+StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+StatsD].Protocol)
	assert.Equal(t, int32(25936), containerPorts[CWA+CollectD].ContainerPort)
	assert.Equal(t, CWA+CollectD, containerPorts[CWA+CollectD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+CollectD].Protocol)
	assert.Equal(t, int32(25888), containerPorts[EMFTcp].ContainerPort)
	assert.Equal(t, EMFTcp, containerPorts[EMFTcp].Name)
	assert.Equal(t, int32(25888), containerPorts[EMFUdp].ContainerPort)
	assert.Equal(t, EMFUdp, containerPorts[EMFUdp].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[EMFUdp].Protocol)
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+XrayTraces].Protocol)
	assert.Equal(t, int32(2900), containerPorts[CWA+XrayProxy].ContainerPort)
	assert.Equal(t, CWA+XrayProxy, containerPorts[CWA+XrayProxy].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+XrayProxy].Protocol)
	assert.Equal(t, int32(4327), containerPorts[CWA+OtlpGrpc].ContainerPort)
	assert.Equal(t, CWA+OtlpGrpc, containerPorts[CWA+OtlpGrpc].Name)
	assert.Equal(t, int32(4328), containerPorts[CWA+OtlpHttp].ContainerPort)
	assert.Equal(t, CWA+OtlpHttp, containerPorts[CWA+OtlpHttp].Name)
}

func TestSpecPortsOverrideGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/statsDAgentConfig.json")
	specPorts := []corev1.ServicePort{
		{
			Name: AppSignalsGrpc,
			Port: 12345,
		},
		{
			Name: AppSignalsProxy,
			Port: 12346,
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", specPorts)
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(12345), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(12346), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(8135), containerPorts[CWA+StatsD].ContainerPort)
	assert.Equal(t, CWA+StatsD, containerPorts[CWA+StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+StatsD].Protocol)
}

func TestInvalidConfigGetContainerPorts(t *testing.T) {
	cfg := getStringFromFile("./test-resources/nilMetrics.json")
	cfg = cfg + ","
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 0, len(containerPorts))

}

func TestValidOTLPMetricsPort(t *testing.T) {
	cfg := getStringFromFile("./test-resources/otlpMetricsAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(1234), containerPorts["cwa-otlp-grpc"].ContainerPort)
	assert.Equal(t, "cwa-otlp-grpc", containerPorts["cwa-otlp-grpc"].Name)
	assert.Equal(t, int32(2345), containerPorts["cwa-otlp-http"].ContainerPort)
	assert.Equal(t, "cwa-otlp-http", containerPorts["cwa-otlp-http"].Name)

}

func TestValidOTLPLogsPort(t *testing.T) {
	cfg := getStringFromFile("./test-resources/otlpLogsAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(1234), containerPorts["cwa-otlp-grpc"].ContainerPort)
	assert.Equal(t, "cwa-otlp-grpc", containerPorts["cwa-otlp-grpc"].Name)
	assert.Equal(t, int32(2345), containerPorts["cwa-otlp-http"].ContainerPort)
	assert.Equal(t, "cwa-otlp-http", containerPorts["cwa-otlp-http"].Name)
}

func TestValidOTLPLogsAndMetricsPort(t *testing.T) {
	cfg := getStringFromFile("./test-resources/otlpMetricsLogsAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 2, len(containerPorts))
	assert.Equal(t, int32(1234), containerPorts["cwa-otlp-grpc"].ContainerPort)
	assert.Equal(t, "cwa-otlp-grpc", containerPorts["cwa-otlp-grpc"].Name)
	assert.Equal(t, int32(2345), containerPorts["cwa-otlp-http"].ContainerPort)
	assert.Equal(t, "cwa-otlp-http", containerPorts["cwa-otlp-http"].Name)
}

func TestValidJSONAndValidOtelConfig(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals.json")
	otelCfg := getStringFromFile("./test-resources/otelConfigs/otlpOtelConfig.yaml")
	containerPorts := getContainerPorts(logger, cfg, otelCfg, []corev1.ServicePort{})
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
	assert.Equal(t, int32(4317), containerPorts[OtlpGrpc].ContainerPort)
	assert.Equal(t, OtlpGrpc, containerPorts[OtlpGrpc].Name)
}

func TestValidJSONAndInvalidOtelConfig(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals.json")
	otelCfg := getStringFromFile("./test-resources/otelConfigs/invalidOtlpConfig.yaml")
	containerPorts := getContainerPorts(logger, cfg, otelCfg, []corev1.ServicePort{})
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
}

func TestValidJSONAndConflictingOtelConfig(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals.json")
	otelCfg := getStringFromFile("./test-resources/otelConfigs/conflictingPortOtlpConfig.yaml")
	containerPorts := getContainerPorts(logger, cfg, otelCfg, []corev1.ServicePort{})
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
}

func TestValidJSONAndConflictingOtelConfigForXray(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals_with_traces.json")
	otelCfg := getStringFromFile("./test-resources/otelConfigs/xrayOtelConfig.yaml")
	containerPorts := getContainerPorts(logger, cfg, otelCfg, []corev1.ServicePort{})
	assert.Equal(t, 6, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[CWA+AppSignalsGrpc].ContainerPort)
	assert.Equal(t, CWA+AppSignalsGrpc, containerPorts[CWA+AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[CWA+AppSignalsHttp].ContainerPort)
	assert.Equal(t, CWA+AppSignalsHttp, containerPorts[CWA+AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[CWA+AppSignalsProxy].ContainerPort)
	assert.Equal(t, CWA+AppSignalsProxy, containerPorts[CWA+AppSignalsProxy].Name)
	assert.Equal(t, int32(2000), containerPorts["awsxray"].ContainerPort)
	assert.Equal(t, "awsxray", containerPorts["awsxray"].Name)
	assert.Equal(t, int32(4317), containerPorts["otlp-grpc"].ContainerPort)
	assert.Equal(t, "otlp-grpc", containerPorts["otlp-grpc"].Name)
	assert.Equal(t, int32(4318), containerPorts["otlp-http"].ContainerPort)
	assert.Equal(t, "otlp-http", containerPorts["otlp-http"].Name)
}

func TestIsDuplicatePort(t *testing.T) {
	containerPorts := map[string]corev1.ContainerPort{
		"cwa-appsig-grpc": {
			Name:          "cwa-appsig-grpc",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4315),
		},
	}

	assert.True(t, isDuplicatePort(containerPorts, corev1.ServicePort{Name: "same-port-same-protocol", Port: 4315, Protocol: corev1.ProtocolTCP}))
}

func getStringFromFile(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(buf)
}
