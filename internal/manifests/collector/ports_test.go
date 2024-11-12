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
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4311), containerPorts[CWA+Server].ContainerPort)
	assert.Equal(t, CWA+Server, containerPorts[CWA+Server].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+Server].Protocol)
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
	wantPorts := []corev1.ContainerPort{
		{
			Name:          CWA + XrayTraces,
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: int32(2000),
		},
		{
			Name:          OtlpGrpc + "-4327",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4327),
		},
		{
			Name:          OtlpHttp + "-4328",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4328),
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	checkPorts(t, wantPorts, containerPorts)
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
	wantPorts := []corev1.ContainerPort{
		{
			Name:          CWA + Server,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4311),
		},
		{
			Name:          CWA + AppSignalsGrpc,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4315),
		},
		{
			Name:          CWA + AppSignalsHttp,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4316),
		},
		{
			Name:          CWA + AppSignalsProxy,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(2000),
		},
		{
			Name:          CWA + StatsD,
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: int32(8135),
		},
		{
			Name:          CWA + CollectD,
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: int32(25936),
		},
		{
			Name:          EMFTcp,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(25888),
		},
		{
			Name:          EMFUdp,
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: int32(25888),
		},
		{
			Name:          CWA + XrayTraces,
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: int32(2800),
		},
		{
			Name:          CWA + XrayProxy,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(2900),
		},
		{
			Name:          OtlpGrpc + "-4327",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4327),
		},
		{
			Name:          OtlpHttp + "-4328",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4328),
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	checkPorts(t, wantPorts, containerPorts)
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
	wantPorts := []corev1.ContainerPort{
		{
			Name:          OtlpGrpc + "-1234",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(1234),
		},
		{
			Name:          OtlpHttp + "-2345",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(2345),
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	checkPorts(t, wantPorts, containerPorts)
}

func TestValidOTLPLogsPort(t *testing.T) {
	cfg := getStringFromFile("./test-resources/otlpLogsAgentConfig.json")
	wantPorts := []corev1.ContainerPort{
		{
			Name:          OtlpGrpc + "-1234",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(1234),
		},
		{
			Name:          OtlpHttp + "-2345",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(2345),
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	checkPorts(t, wantPorts, containerPorts)
}

func TestValidOTLPLogsAndMetricsPort(t *testing.T) {
	cfg := getStringFromFile("./test-resources/otlpMetricsLogsAgentConfig.json")
	wantPorts := []corev1.ContainerPort{
		{
			Name:          OtlpGrpc + "-1234",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(1234),
		},
		{
			Name:          OtlpHttp + "-2345",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(2345),
		},
		{
			Name:          OtlpGrpc + "-4317",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4317),
		},
		{
			Name:          OtlpHttp + "-4318",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(4318),
		},
	}
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	checkPorts(t, wantPorts, containerPorts)
}

func TestValidJSONAndValidOtelConfig(t *testing.T) {
	cfg := getStringFromFile("./test-resources/application_signals.json")
	otelCfg := getStringFromFile("./test-resources/otelConfigs/otlpOtelConfig.yaml")
	containerPorts := getContainerPorts(logger, cfg, otelCfg, []corev1.ServicePort{})
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4311), containerPorts[CWA+Server].ContainerPort)
	assert.Equal(t, CWA+Server, containerPorts[CWA+Server].Name)
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
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4311), containerPorts[CWA+Server].ContainerPort)
	assert.Equal(t, CWA+Server, containerPorts[CWA+Server].Name)
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
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4311), containerPorts[CWA+Server].ContainerPort)
	assert.Equal(t, CWA+Server, containerPorts[CWA+Server].Name)
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
	assert.Equal(t, 7, len(containerPorts))
	assert.Equal(t, int32(4311), containerPorts[CWA+Server].ContainerPort)
	assert.Equal(t, CWA+Server, containerPorts[CWA+Server].Name)
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

func TestJMXGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/jmxAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(4314), containerPorts[JmxHttp].ContainerPort)
	assert.Equal(t, JmxHttp, containerPorts[JmxHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[JmxHttp].Protocol)
}

func TestJMXContainerInsightsGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/jmxContainerInsightsConfig.json")
	containerPorts := getContainerPorts(logger, cfg, "", []corev1.ServicePort{})
	assert.Equal(t, 1, len(containerPorts))
	assert.Equal(t, int32(4314), containerPorts[JmxHttp].ContainerPort)
	assert.Equal(t, JmxHttp, containerPorts[JmxHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[JmxHttp].Protocol)
}

func checkPorts(t *testing.T, want []corev1.ContainerPort, got map[string]corev1.ContainerPort) {
	t.Helper()

	assert.Equal(t, len(want), len(got))
	for _, wantPort := range want {
		gotPort := got[wantPort.Name]
		assert.Equal(t, wantPort.Name, gotPort.Name)
		assert.Equal(t, wantPort.Protocol, gotPort.Protocol)
		assert.Equal(t, wantPort.ContainerPort, gotPort.ContainerPort)
	}
}

func getStringFromFile(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(buf)
}

func getJSONStringFromFile(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(buf)
}
