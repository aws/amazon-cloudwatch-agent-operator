// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
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

var AppSignalsPortToServicePortMap = map[int32]corev1.ServicePort{
	4315: {
		Name: AppSignalsGrpc,
		Port: 4315,
	},
	4316: {
		Name: AppSignalsHttp,
		Port: 4316,
	},
	2000: {
		Name: AppSignalsProxy,
		Port: 2000,
	},
}

func PortMapToServicePortList(portMap map[int32]corev1.ServicePort) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(portMap))
	for _, p := range portMap {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})
	return ports
}
