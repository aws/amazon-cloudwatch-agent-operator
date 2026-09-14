// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package neuronmonitor

import (
	"crypto/sha256"
	"fmt"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
)

// Annotations return the annotations for NeuronMonitor workload metadata.
func Annotations(instance v1alpha1.NeuronMonitor) map[string]string {
	// new map every time, so that we don't touch the instance's annotations
	annotations := map[string]string{}

	annotations["k8s-app"] = ComponentNeuronExporter

	// allow override of prometheus annotations
	if nil != instance.Annotations {
		for k, v := range instance.Annotations {
			annotations[k] = v
		}
	}
	// make sure sha256 for configMap is always calculated
	annotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(instance.Spec.MonitorConfig)

	return annotations
}

// PodAnnotations return the pod template annotations for NeuronMonitor pods.
// Spec.PodAnnotations take precedence over metadata-level annotations.
func PodAnnotations(instance v1alpha1.NeuronMonitor) map[string]string {
	podAnnotations := map[string]string{}

	for k, v := range instance.Spec.PodAnnotations {
		podAnnotations[k] = v
	}

	for kMeta, vMeta := range Annotations(instance) {
		if _, found := podAnnotations[kMeta]; !found {
			podAnnotations[kMeta] = vMeta
		}
	}

	// make sure sha256 for configMap is always calculated
	podAnnotations["amazon-cloudwatch-agent-operator-config/sha256"] = getConfigMapSHA(instance.Spec.MonitorConfig)

	return podAnnotations
}

// PodLabels merges user-provided Spec.PodLabels on top of the operator-managed
// labels. Operator-managed selector labels cannot be overridden.
func PodLabels(instance v1alpha1.NeuronMonitor, base map[string]string) map[string]string {
	merged := map[string]string{}
	for k, v := range instance.Spec.PodLabels {
		merged[k] = v
	}
	for k, v := range base {
		merged[k] = v
	}
	return merged
}

func getConfigMapSHA(config string) string {
	h := sha256.Sum256([]byte(config))
	return fmt.Sprintf("%x", h)
}
