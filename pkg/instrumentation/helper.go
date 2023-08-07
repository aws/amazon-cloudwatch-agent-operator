// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	corev1 "k8s.io/api/core/v1"
)

// Calculate if we already inject InitContainers.
func isInitContainerMissing(pod corev1.Pod) bool {
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == initContainerName {
			return false
		}
	}
	return true
}

// Checks if Pod is already instrumented by checking Instrumentation InitContainer presence.
func isAutoInstrumentationInjected(pod corev1.Pod) bool {
	for _, cont := range pod.Spec.InitContainers {
		if cont.Name == initContainerName {
			return true
		}
	}
	// Go uses a side car
	for _, cont := range pod.Spec.Containers {
		if cont.Name == sideCarName {
			return true
		}
	}
	return false
}
