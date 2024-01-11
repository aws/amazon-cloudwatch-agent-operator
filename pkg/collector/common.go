// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	corev1 "k8s.io/api/core/v1"
)

var AppSignalsCloudwatchAgentPorts = []corev1.ServicePort{
	{
		Name: AppSignalsGrpc,
		Port: 4315,
	},
	{
		Name: AppSignalsHttp,
		Port: 4316,
	},
	{
		Name: AppSignalsProxy,
		Port: 2000,
	},
}

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

var apmPortToServicePortMap = map[int32]corev1.ServicePort{
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
