package collector

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"testing"
)

var logger = logf.Log.WithName("unit-tests")

func TestStatsDGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/statsDAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(8135), containerPorts[CWA+StatsD].ContainerPort)
	assert.Equal(t, CWA+StatsD, containerPorts[CWA+StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+StatsD].Protocol)
}

func TestDefaultStatsDGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/statsDDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(8125), containerPorts[StatsD].ContainerPort)
	assert.Equal(t, StatsD, containerPorts[StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[StatsD].Protocol)
}

func TestCollectDGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/collectDAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(25936), containerPorts[CWA+CollectD].ContainerPort)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+CollectD].Protocol)
}

func TestDefaultCollectDGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/collectDDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(25826), containerPorts[CollectD].ContainerPort)
	assert.Equal(t, CollectD, containerPorts[CollectD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CollectD].Protocol)
}

func TestEMFGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/emfAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 4, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(25888), containerPorts[EMF].ContainerPort)
	assert.Equal(t, EMF, containerPorts[EMF].Name)
}

func TestXrayAndOTLPGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/xrayAndOTLPAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(4327), containerPorts[CWA+OtlpGrpc].ContainerPort)
	assert.Equal(t, CWA+OtlpGrpc, containerPorts[CWA+OtlpGrpc].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+OtlpGrpc].Protocol)
	assert.Equal(t, int32(4328), containerPorts[CWA+OtlpHttp].ContainerPort)
	assert.Equal(t, CWA+OtlpHttp, containerPorts[CWA+OtlpHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+OtlpHttp].Protocol)
}

func TestDefaultXRayAndOTLPGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/xrayAndOTLPDefaultAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(4317), containerPorts[OtlpGrpc].ContainerPort)
	assert.Equal(t, OtlpGrpc, containerPorts[OtlpGrpc].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[OtlpGrpc].Protocol)
	assert.Equal(t, int32(4318), containerPorts[OtlpHttp].ContainerPort)
	assert.Equal(t, OtlpHttp, containerPorts[OtlpHttp].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[OtlpHttp].Protocol)
}

func TestXRayGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/xrayAgentConfig.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+XrayTraces].Protocol)
	assert.Equal(t, int32(2900), containerPorts[CWA+XrayProxy].ContainerPort)
	assert.Equal(t, CWA+XrayProxy, containerPorts[CWA+XrayProxy].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+XrayProxy].Protocol)
}

func TestXRayWithBindAddressDefaultGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/xrayAgentConfig.json")
	strings.Replace(cfg, "2800", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(2900), containerPorts[CWA+XrayProxy].ContainerPort)
	assert.Equal(t, CWA+XrayProxy, containerPorts[CWA+XrayProxy].Name)
	assert.Equal(t, corev1.ProtocolTCP, containerPorts[CWA+XrayProxy].Protocol)
}

func TestXRayWithTCPProxyBindAddressDefaultGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/xrayAgentConfig.json")
	strings.Replace(cfg, "2900", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 5, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(2800), containerPorts[CWA+XrayTraces].ContainerPort)
	assert.Equal(t, CWA+XrayTraces, containerPorts[CWA+XrayTraces].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+XrayTraces].Protocol)
}

func TestNilMetricsGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/nilMetrics.json")
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
}

func TestMultipleReceiversGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/multipleReceiversAgentConfig.json")
	strings.Replace(cfg, "2900", "2000", 1)
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 10, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
	assert.Equal(t, int32(8135), containerPorts[CWA+StatsD].ContainerPort)
	assert.Equal(t, CWA+StatsD, containerPorts[CWA+StatsD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+StatsD].Protocol)
	assert.Equal(t, int32(25936), containerPorts[CWA+CollectD].ContainerPort)
	assert.Equal(t, CWA+CollectD, containerPorts[CWA+CollectD].Name)
	assert.Equal(t, corev1.ProtocolUDP, containerPorts[CWA+CollectD].Protocol)
	assert.Equal(t, int32(25888), containerPorts[EMF].ContainerPort)
	assert.Equal(t, EMF, containerPorts[EMF].Name)
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

func TestInvalidConfigGetContainerPorts(t *testing.T) {
	cfg := getJSONStringFromFile("./test-resources/nilMetrics.json")
	cfg = cfg + ","
	containerPorts := getContainerPorts(logger, cfg)
	assert.Equal(t, 3, len(containerPorts))
	assert.Equal(t, int32(4315), containerPorts[AppSignalsGrpc].ContainerPort)
	assert.Equal(t, AppSignalsGrpc, containerPorts[AppSignalsGrpc].Name)
	assert.Equal(t, int32(4316), containerPorts[AppSignalsHttp].ContainerPort)
	assert.Equal(t, AppSignalsHttp, containerPorts[AppSignalsHttp].Name)
	assert.Equal(t, int32(2000), containerPorts[AppSignalsProxy].ContainerPort)
	assert.Equal(t, AppSignalsProxy, containerPorts[AppSignalsProxy].Name)
}

func getJSONStringFromFile(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(buf)
}
