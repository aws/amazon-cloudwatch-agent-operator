// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"

	corev1 "k8s.io/api/core/v1"
)

const (
	StatsD          = "statsd"
	CollectD        = "collectd"
	XrayProxy       = "aws-proxy"
	XrayTraces      = "aws-traces"
	OtlpGrpc        = "otlp-grpc"
	OtlpHttp        = "otlp-http"
	AppSignalsGrpc  = "appsignals-grpc"
	AppSignalsHttp  = "appsignals-http"
	AppSignalsProxy = "appsignals-xray"
	EMF             = "emf"
	EMFTcp          = "emf-tcp"
	EMFUdp          = "emf-udp"
	CWA             = "cwa-"
)

var receiverDefaultPortsMap = map[string]int32{
	StatsD:     8125,
	CollectD:   25826,
	XrayTraces: 2000,
	OtlpGrpc:   4317,
	OtlpHttp:   4318,
	EMF:        25888,
}

var AppSignalsPortToServicePortMap = map[int32][]corev1.ServicePort{
	4315: {{
		Name: AppSignalsGrpc,
		Port: 4315,
	}},
	4316: {{
		Name: AppSignalsHttp,
		Port: 4316,
	}},
	2000: {{
		Name: AppSignalsProxy,
		Port: 2000,
	}},
}

func PortMapToServicePortList(portMap map[int32][]corev1.ServicePort) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(portMap))
	for _, plist := range portMap {
		for _, p := range plist {
			ports = append(ports, p)
		}
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})
	return ports
}

func getContainerPorts(logger logr.Logger, cfg string, specPorts []corev1.ServicePort) map[string]corev1.ContainerPort {
	ports := map[string]corev1.ContainerPort{}
	var servicePorts []corev1.ServicePort
	config, err := adapters.ConfigStructFromJSONString(cfg)
	if err != nil {
		logger.Error(err, "error parsing cw agent config")
	} else {
		servicePorts = getServicePortsFromCWAgentConfig(logger, config)
	}

	for _, p := range servicePorts {
		truncName := naming.Truncate(p.Name, maxPortLen)
		if p.Name != truncName {
			logger.Info("truncating container port name",
				zap.String("port.name.prev", p.Name), zap.String("port.name.new", truncName))
		}
		nameErrs := validation.IsValidPortName(truncName)
		numErrs := validation.IsValidPortNum(int(p.Port))
		if len(nameErrs) > 0 || len(numErrs) > 0 {
			logger.Info("dropping invalid container port", zap.String("port.name", truncName), zap.Int32("port.num", p.Port),
				zap.Strings("port.name.errs", nameErrs), zap.Strings("num.errs", numErrs))
			continue
		}
		ports[truncName] = corev1.ContainerPort{
			Name:          truncName,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		}
	}

	for _, p := range specPorts {
		ports[p.Name] = corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		}
	}
	return ports
}

func getServicePortsFromCWAgentConfig(logger logr.Logger, config *adapters.CwaConfig) []corev1.ServicePort {
	servicePortsMap := make(map[int32][]corev1.ServicePort)

	getApplicationSignalsReceiversServicePorts(config, servicePortsMap)
	getMetricsReceiversServicePorts(logger, config, servicePortsMap)
	getLogsReceiversServicePorts(logger, config, servicePortsMap)
	getTracesReceiversServicePorts(logger, config, servicePortsMap)

	return PortMapToServicePortList(servicePortsMap)
}

func isAppSignalEnabled(config *adapters.CwaConfig) bool {
	if config.GetApplicationSignalsConfig() != nil {
		return true
	}
	return false
}

func getMetricsReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32][]corev1.ServicePort) {
	if config.Metrics == nil || config.Metrics.MetricsCollected == nil {
		return
	}
	//StatD - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-statsd.html
	if config.Metrics.MetricsCollected.StatsD != nil {
		getReceiverServicePort(logger, config.Metrics.MetricsCollected.StatsD.ServiceAddress, StatsD, corev1.ProtocolUDP, servicePortsMap)
	}
	//CollectD - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-custom-metrics-collectd.html
	if config.Metrics.MetricsCollected.CollectD != nil {
		getReceiverServicePort(logger, config.Metrics.MetricsCollected.CollectD.ServiceAddress, CollectD, corev1.ProtocolUDP, servicePortsMap)
	}
}

func getReceiverServicePort(logger logr.Logger, serviceAddress string, receiverName string, protocol corev1.Protocol, servicePortsMap map[int32][]corev1.ServicePort) {
	if serviceAddress != "" {
		port, err := portFromEndpoint(serviceAddress)
		if err != nil {
			logger.Error(err, "error parsing port from endpoint for receiver", zap.String("endpoint", serviceAddress), zap.String("receiver", receiverName))
		} else {
			if _, ok := servicePortsMap[port]; ok {
				logger.Info("Duplicate port has been configured in Agent Config for port", zap.Int32("port", port))
			} else {
				sp := corev1.ServicePort{
					Name:     CWA + receiverName,
					Port:     port,
					Protocol: protocol,
				}
				servicePortsMap[port] = []corev1.ServicePort{sp}
			}
		}
	} else {
		if _, ok := servicePortsMap[receiverDefaultPortsMap[receiverName]]; ok {
			logger.Info("Duplicate port has been configured in Agent Config for port", zap.Int32("port", receiverDefaultPortsMap[receiverName]))
		} else {
			sp := corev1.ServicePort{
				Name:     receiverName,
				Port:     receiverDefaultPortsMap[receiverName],
				Protocol: protocol,
			}
			servicePortsMap[receiverDefaultPortsMap[receiverName]] = []corev1.ServicePort{sp}
		}
	}
}

func getLogsReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32][]corev1.ServicePort) {
	//EMF - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation_CloudWatch_Agent.html
	if config.Logs != nil && config.Logs.LogMetricsCollected != nil && config.Logs.LogMetricsCollected.EMF != nil {
		if _, ok := servicePortsMap[receiverDefaultPortsMap[EMF]]; ok {
			logger.Info("Duplicate port has been configured in Agent Config for port", zap.Int32("port", receiverDefaultPortsMap[EMF]))
		} else {
			tcp := corev1.ServicePort{
				Name:     EMFTcp,
				Port:     receiverDefaultPortsMap[EMF],
				Protocol: corev1.ProtocolTCP,
			}
			udp := corev1.ServicePort{
				Name:     EMFUdp,
				Port:     receiverDefaultPortsMap[EMF],
				Protocol: corev1.ProtocolUDP,
			}
			servicePortsMap[receiverDefaultPortsMap[EMF]] = []corev1.ServicePort{tcp, udp}
		}
	}
}

func getTracesReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32][]corev1.ServicePort) []corev1.ServicePort {
	var tracesPorts []corev1.ServicePort

	if config.Traces == nil || config.Traces.TracesCollected == nil {
		return tracesPorts
	}
	//Traces - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html#CloudWatch-Agent-Configuration-File-Tracessection
	//OTLP
	if config.Traces.TracesCollected.OTLP != nil {
		//GRPC
		getReceiverServicePort(logger, config.Traces.TracesCollected.OTLP.GRPCEndpoint, OtlpGrpc, corev1.ProtocolTCP, servicePortsMap)
		//HTTP
		getReceiverServicePort(logger, config.Traces.TracesCollected.OTLP.HTTPEndpoint, OtlpHttp, corev1.ProtocolTCP, servicePortsMap)

	}
	//Xray
	if config.Traces.TracesCollected.XRay != nil {
		getReceiverServicePort(logger, config.Traces.TracesCollected.XRay.BindAddress, XrayTraces, corev1.ProtocolUDP, servicePortsMap)
		if config.Traces.TracesCollected.XRay.TCPProxy != nil {
			getReceiverServicePort(logger, config.Traces.TracesCollected.XRay.TCPProxy.BindAddress, XrayProxy, corev1.ProtocolTCP, servicePortsMap)
		}
	}
	return tracesPorts
}

func addAppSignalServicePorts(servicePortsMap map[int32][]corev1.ServicePort) {
	for k, v := range AppSignalsPortToServicePortMap {
		servicePortsMap[k] = v
	}
}

func getApplicationSignalsReceiversServicePorts(config *adapters.CwaConfig, servicePortsMap map[int32][]corev1.ServicePort) {
	if isAppSignalEnabled(config) {
		addAppSignalServicePorts(servicePortsMap)
	}
}

func portFromEndpoint(endpoint string) (int32, error) {
	var err error
	var port int64

	r := regexp.MustCompile(":[0-9]+")

	if r.MatchString(endpoint) {
		port, err = strconv.ParseInt(strings.Replace(r.FindString(endpoint), ":", "", -1), 10, 32)

		if err != nil {
			return 0, err
		}
	}

	if port == 0 {
		return 0, errors.New("port should not be empty")
	}

	return int32(port), err
}
