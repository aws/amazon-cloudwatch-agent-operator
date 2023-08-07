// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package sidecar contains operations related to sidecar manipulation (Add, update, remove).
package sidecar

import (
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/collector/reconcile"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/naming"
)

const (
	label      = "sidecar.opentelemetry.io/injected"
	confEnvVar = "OTEL_CONFIG"
)

// add a new sidecar container to the given pod, based on the given AmazonCloudWatchAgent.
func add(cfg config.Config, logger logr.Logger, otelcol v1alpha1.AmazonCloudWatchAgent, pod corev1.Pod, attributes []corev1.EnvVar) (corev1.Pod, error) {
	otelColCfg, err := reconcile.ReplaceConfig(otelcol)
	if err != nil {
		return pod, err
	}

	container := collector.Container(cfg, logger, otelcol, false)
	container.Args = append(container.Args, fmt.Sprintf("--config=env:%s", confEnvVar))

	container.Env = append(container.Env, corev1.EnvVar{Name: confEnvVar, Value: otelColCfg})
	if !hasResourceAttributeEnvVar(container.Env) {
		container.Env = append(container.Env, attributes...)
	}
	pod.Spec.Containers = append(pod.Spec.Containers, container)
	pod.Spec.Volumes = append(pod.Spec.Volumes, otelcol.Spec.Volumes...)

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	pod.Labels[label] = fmt.Sprintf("%s.%s", otelcol.Namespace, otelcol.Name)

	return pod, nil
}

// remove the sidecar container from the given pod.
func remove(pod corev1.Pod) (corev1.Pod, error) {
	if !existsIn(pod) {
		return pod, nil
	}

	var containers []corev1.Container
	for _, container := range pod.Spec.Containers {
		if container.Name != naming.Container() {
			containers = append(containers, container)
		}
	}
	pod.Spec.Containers = containers
	return pod, nil
}

// existsIn checks whether a sidecar container exists in the given pod.
func existsIn(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == naming.Container() {
			return true
		}
	}
	return false
}
