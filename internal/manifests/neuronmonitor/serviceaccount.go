// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/manifestutils"
)

const neuronSrviceAcctName = "neuron-monitor-service-acct"

// ServiceAccountName returns the name of the existing or self-provisioned service account to use for the given instance.
func ServiceAccountName(instance v1alpha1.NeuronMonitor) string {
	if len(instance.Spec.ServiceAccount) == 0 {
		return neuronSrviceAcctName
	}
	return instance.Spec.ServiceAccount
}

// ServiceAccount returns the service account for the given instance.
func ServiceAccount(params manifests.Params) *corev1.ServiceAccount {
	labels := manifestutils.Labels(params.NeuronExp.ObjectMeta, neuronSrviceAcctName, params.NeuronExp.Spec.Image, ComponentNeuronExporter, []string{})

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        neuronSrviceAcctName,
			Namespace:   params.NeuronExp.Namespace,
			Labels:      labels,
			Annotations: Annotations(params.NeuronExp),
		},
	}
}
