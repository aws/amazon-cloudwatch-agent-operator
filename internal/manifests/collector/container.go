// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

// maxPortLen allows us to truncate a port name according to what is considered valid port syntax:
// https://pkg.go.dev/k8s.io/apimachinery/pkg/util/validation#IsValidPortName
const maxPortLen = 15

// Container builds a container for the given collector.
func Container(cfg config.Config, logger logr.Logger, agent v1alpha1.AmazonCloudWatchAgent, addConfig bool) corev1.Container {
	image := agent.Spec.Image
	if len(image) == 0 {
		image = cfg.CollectorImage()
	}

	ports := getContainerPorts(logger, agent.Spec.Config)
	for _, p := range agent.Spec.Ports {
		ports[p.Name] = corev1.ContainerPort{
			Name:          p.Name,
			ContainerPort: p.Port,
			Protocol:      p.Protocol,
		}
	}

	var volumeMounts []corev1.VolumeMount
	argsMap := agent.Spec.Args
	if argsMap == nil {
		argsMap = map[string]string{}
	}
	// defines the output (sorted) array for final output
	var args []string
	// When adding a config via v1alpha1.AmazonCloudWatchAgentSpec.Config, we ensure that it is always the
	// first item in the args. At the time of writing, although multiple configs are allowed in the
	// cloudwatch agent, the operator has yet to implement such functionality.  When multiple configs
	// are present they should be merged in a deterministic manner using the order given, and because
	// v1alpha1.AmazonCloudWatchAgentSpec.Config is a required field we assume that it will always be the
	// "primary" config and in the future additional configs can be appended to the container args in a simple manner.

	if addConfig {
		if agent.Spec.NodeSelector["kubernetes.io/os"] == "windows" {
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      naming.ConfigMapVolume(),
					MountPath: "C:\\Program Files\\Amazon\\AmazonCloudWatchAgent\\cwagentconfig",
				},
			)
		} else {
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      naming.ConfigMapVolume(),
					MountPath: "/etc/cwagentconfig",
				},
			)
		}
	}

	// ensure that the v1alpha1.AmazonCloudWatchAgentSpec.Args are ordered when moved to container.Args,
	// where iterating over a map does not guarantee, so that reconcile will not be fooled by different
	// ordering in args.
	var sortedArgs []string
	for k, v := range argsMap {
		sortedArgs = append(sortedArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	sort.Strings(sortedArgs)
	args = append(args, sortedArgs...)

	if len(agent.Spec.VolumeMounts) > 0 {
		volumeMounts = append(volumeMounts, agent.Spec.VolumeMounts...)
	}

	var envVars = agent.Spec.Env
	if agent.Spec.Env == nil {
		envVars = []corev1.EnvVar{}
	}

	envVars = append(envVars, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	if _, err := adapters.ConfigFromJSONString(agent.Spec.Config); err != nil {
		logger.Error(err, "error parsing config")
	}

	return corev1.Container{
		Name:            naming.Container(),
		Image:           image,
		ImagePullPolicy: agent.Spec.ImagePullPolicy,
		VolumeMounts:    volumeMounts,
		Args:            args,
		Env:             envVars,
		EnvFrom:         agent.Spec.EnvFrom,
		Resources:       agent.Spec.Resources,
		Ports:           portMapToContainerPortList(ports),
		SecurityContext: agent.Spec.SecurityContext,
		Lifecycle:       agent.Spec.Lifecycle,
	}
}

func getContainerPorts(logger logr.Logger, cfg string) map[string]corev1.ContainerPort {
	ports := map[string]corev1.ContainerPort{}
	var servicePorts []corev1.ServicePort
	config, err := adapters.ConfigStructFromJSONString(cfg)
	if err != nil {
		logger.Error(err, "error parsing cw agent config")
		servicePorts = PortMapToServicePortList(AppSignalsPortToServicePortMap)
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
	return ports
}

func getServicePortsFromCWAgentConfig(logger logr.Logger, config *adapters.CwaConfig) []corev1.ServicePort {
	servicePortsMap := getAppSignalsServicePortsMap()
	getMetricsReceiversServicePorts(logger, config, servicePortsMap)
	getLogsReceiversServicePorts(logger, config, servicePortsMap)
	getTracesReceiversServicePorts(logger, config, servicePortsMap)
	return PortMapToServicePortList(servicePortsMap)
}

func getMetricsReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32]corev1.ServicePort) {
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

func getReceiverServicePort(logger logr.Logger, serviceAddress string, receiverName string, protocol corev1.Protocol, servicePortsMap map[int32]corev1.ServicePort) {
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
				servicePortsMap[port] = sp
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
			servicePortsMap[receiverDefaultPortsMap[receiverName]] = sp
		}
	}
}

func getLogsReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32]corev1.ServicePort) {
	//EMF - https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Generation_CloudWatch_Agent.html
	if config.Logs != nil && config.Logs.LogMetricsCollected != nil && config.Logs.LogMetricsCollected.EMF != nil {
		if _, ok := servicePortsMap[receiverDefaultPortsMap[EMF]]; ok {
			logger.Info("Duplicate port has been configured in Agent Config for port", zap.Int32("port", receiverDefaultPortsMap[EMF]))
		} else {
			sp := corev1.ServicePort{
				Name: EMF,
				Port: receiverDefaultPortsMap[EMF],
			}
			servicePortsMap[receiverDefaultPortsMap[EMF]] = sp
		}
	}
}

func getTracesReceiversServicePorts(logger logr.Logger, config *adapters.CwaConfig, servicePortsMap map[int32]corev1.ServicePort) []corev1.ServicePort {
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

func getAppSignalsServicePortsMap() map[int32]corev1.ServicePort {
	servicePortMap := make(map[int32]corev1.ServicePort)
	for k, v := range AppSignalsPortToServicePortMap {
		servicePortMap[k] = v
	}
	return servicePortMap
}

func portMapToContainerPortList(portMap map[string]corev1.ContainerPort) []corev1.ContainerPort {
	ports := make([]corev1.ContainerPort, 0, len(portMap))
	for _, p := range portMap {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})
	return ports
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
