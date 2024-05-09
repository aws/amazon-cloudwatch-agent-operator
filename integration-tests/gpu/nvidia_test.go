// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package emf

import (
	"time"

	"github.com/aws/amazon-cloudwatch-agent-test/environment"
	"github.com/aws/amazon-cloudwatch-agent-test/test/metric"
	"github.com/aws/amazon-cloudwatch-agent-test/test/status"
	"github.com/aws/amazon-cloudwatch-agent-test/test/test_runner"
)

const (
	gpuMetricIndicator = "_gpu_"

	containerMemTotal   = "container_gpu_memory_total"
	containerMemUsed    = "container_gpu_memory_used"
	containerPower      = "container_gpu_power_draw"
	containerTemp       = "container_gpu_temperature"
	containerUtil       = "container_gpu_utilization"
	containerMemUtil    = "container_gpu_memory_utilization"
	podMemTotal         = "pod_gpu_memory_total"
	podMemUsed          = "pod_gpu_memory_used"
	podPower            = "pod_gpu_power_draw"
	podTemp             = "pod_gpu_temperature"
	podUtil             = "pod_gpu_utilization"
	podMemUtil          = "pod_gpu_memory_utilization"
	nodeMemTotal        = "node_gpu_memory_total"
	nodeMemUsed         = "node_gpu_memory_used"
	nodePower           = "node_gpu_power_draw"
	nodeTemp            = "node_gpu_temperature"
	nodeUtil            = "node_gpu_utilization"
	nodeMemUtil         = "node_gpu_memory_utilization"
	nodeCountTotal      = "node_gpu_total"
	nodeCountRequest    = "node_gpu_request"
	nodeCountLimit      = "node_gpu_limit"
	clusterCountTotal   = "cluster_gpu_total"
	clusterCountRequest = "cluster_gpu_request"
)

var expectedDimsToMetrics = map[string][]string{
	"ClusterName": {
		containerMemTotal, containerMemUsed, containerPower, containerTemp, containerUtil, containerMemUtil,
		podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
		nodeMemTotal, nodeMemUsed, nodePower, nodeTemp, nodeUtil, nodeMemUtil,
		//nodeCountTotal, nodeCountRequest, nodeCountLimit,
		//clusterCountTotal, clusterCountRequest,
	},
	"ClusterName-Namespace": {
		podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
	},
	//"ClusterName-Namespace-Service": {
	//	podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
	//},
	"ClusterName-Namespace-PodName": {
		podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
	},
	"ClusterName-ContainerName-Namespace-PodName": {
		containerMemTotal, containerMemUsed, containerPower, containerTemp, containerUtil, containerMemUtil,
	},
	"ClusterName-ContainerName-FullPodName-Namespace-PodName": {
		containerMemTotal, containerMemUsed, containerPower, containerTemp, containerUtil, containerMemUtil,
	},
	"ClusterName-ContainerName-FullPodName-GpuDevice-Namespace-PodName": {
		containerMemTotal, containerMemUsed, containerPower, containerTemp, containerUtil, containerMemUtil,
	},
	"ClusterName-FullPodName-Namespace-PodName": {
		podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
	},
	"ClusterName-FullPodName-GpuDevice-Namespace-PodName": {
		podMemTotal, podMemUsed, podPower, podTemp, podUtil, podMemUtil,
	},
	"ClusterName-InstanceId-NodeName": {
		nodeMemTotal, nodeMemUsed, nodePower, nodeTemp, nodeUtil, nodeMemUtil,
		//nodeCountTotal, nodeCountRequest, nodeCountLimit,
	},
	"ClusterName-GpuDevice-InstanceId-InstanceType-NodeName": {
		nodeMemTotal, nodeMemUsed, nodePower, nodeTemp, nodeUtil, nodeMemUtil,
	},
}

type NvidiaTestRunner struct {
	test_runner.BaseTestRunner
	testName string
	env      *environment.MetaData
}

var _ test_runner.ITestRunner = (*NvidiaTestRunner)(nil)

func (t *NvidiaTestRunner) Validate() status.TestGroupResult {
	var testResults []status.TestResult
	testResults = append(testResults, metric.ValidateMetrics(t.env, gpuMetricIndicator, expectedDimsToMetrics)...)
	testResults = append(testResults, metric.ValidateLogs(t.env))
	return status.TestGroupResult{
		Name:        t.GetTestName(),
		TestResults: testResults,
	}
}

func (t *NvidiaTestRunner) GetTestName() string {
	return t.testName
}

func (t *NvidiaTestRunner) GetAgentConfigFileName() string {
	return ""
}

func (t *NvidiaTestRunner) GetAgentRunDuration() time.Duration {
	return 3 * time.Minute
}

func (t *NvidiaTestRunner) GetMeasuredMetrics() []string {
	return nil
}
