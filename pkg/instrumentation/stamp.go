// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/webhook/podmutation"
)

const (
	// LabelAutoInstrumented marks pods that the operator injected auto-instrumentation into.
	// It is a label so that the instrumentation guard can watch and cache only these pods.
	LabelAutoInstrumented = "cloudwatch.aws.amazon.com/auto-instrumented"
)

// stampedTypes are the languages whose injection is recorded on the pod.
var stampedTypes = []Type{TypeJava, TypeNodeJS, TypePython, TypeDotNet}

// InitContainerName returns the name of the init container that copies the auto-instrumentation
// payload for a language, and whether the language uses one.
func InitContainerName(instType Type) (string, bool) {
	switch instType {
	case TypeJava:
		return javaInitContainerName, true
	case TypeNodeJS:
		return nodejsInitContainerName, true
	case TypePython:
		return pythonInitContainerName, true
	case TypeDotNet:
		return dotnetInitContainerName, true
	default:
		return "", false
	}
}

// InstrMountPath returns the path the auto-instrumentation payload for a language is mounted at
// inside the application container, and whether the language has one. It is the accessor every
// caller outside this package must use: the constants themselves stay unexported, so a consumer
// such as the instrumentation guard's attribution matcher cannot drift from the path injection
// actually sets.
//
// Only the Linux mount paths are returned. The Windows variants (javaInstrMountPathWindows,
// dotnetInstrMountPathWindows) are deliberately out of scope: the only consumer is message
// attribution, which is Python-only, and Python has no Windows mount path.
func InstrMountPath(instType Type) (string, bool) {
	switch instType {
	case TypeJava:
		return javaInstrMountPath, true
	case TypeNodeJS:
		return nodejsInstrMountPath, true
	case TypePython:
		return pythonInstrMountPath, true
	case TypeDotNet:
		return dotnetInstrMountPath, true
	default:
		return "", false
	}
}

// instrVolumeName returns the name of the emptyDir volume that carries the auto-instrumentation
// payload for a language, and whether the language uses one. The volume is mounted into both the
// language's init container and every application container that language was injected into, which
// is what makes the mount name a reliable way to find those containers.
func instrVolumeName(instType Type) (string, bool) {
	switch instType {
	case TypeJava:
		return javaVolumeName, true
	case TypeNodeJS:
		return nodejsVolumeName, true
	case TypePython:
		return pythonVolumeName, true
	case TypeDotNet:
		return dotnetVolumeName, true
	default:
		return "", false
	}
}

// InjectedImages returns the languages that were injected into the pod, mapped to the image the
// kubelet reports for that language's init container. The map is empty if the pod is not stamped.
//
// InjectedImages runs in the CONTROLLER, so it reads pod.Status.InitContainerStatuses rather than
// pod.Spec.InitContainers: the guard's pod cache transform strips the spec to keep ~40,000 cached
// pods cheap, and by the time a controller sees a pod its status exists. Mutate, the webhook-side
// counterpart in this file, reads the spec instead. Same file, opposite sources, both correct for
// their caller.
//
// An init container status whose name is one the operator sets is the injection signal; the image
// is just the value carried alongside it, so a status the kubelet has not filled in an image for
// yet still counts as injected.
func InjectedImages(pod corev1.Pod) map[Type]string {
	images := map[Type]string{}
	if pod.Labels[LabelAutoInstrumented] != "true" {
		return images
	}
	byInitContainerName := make(map[string]Type, len(stampedTypes))
	for _, instType := range stampedTypes {
		if name, ok := InitContainerName(instType); ok {
			byInitContainerName[name] = instType
		}
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if instType, ok := byInitContainerName[status.Name]; ok {
			images[instType] = status.Image
		}
	}
	return images
}

// stampMutator records that the operator injected auto-instrumentation into a pod, and makes the
// pod's crash output readable from its status. It must run after the instrumentation mutator in the
// webhook's mutator chain, because it inspects the init containers that mutator adds.
type stampMutator struct{}

var _ podmutation.PodMutator = (*stampMutator)(nil)

// NewStampMutator creates a pod mutator that stamps injected pods. See stampMutator.
func NewStampMutator() *stampMutator {
	return &stampMutator{}
}

// Mutate adds LabelAutoInstrumented and sets terminationMessagePolicy to FallbackToLogsOnError on
// the application containers auto-instrumentation was injected into. It uses the presence of each
// language's init container as the signal that injection succeeded, which is the same signal
// isAutoInstrumentationInjected uses. Pods without any such init container are returned unchanged.
//
// Mutate runs in the WEBHOOK, so it reads pod.Spec.InitContainers: at admission time the pod has no
// status at all. InjectedImages, the controller-side counterpart in this file, reads
// pod.Status.InitContainerStatuses instead. Same file, opposite sources, both correct for their
// caller.
func (m *stampMutator) Mutate(_ context.Context, _ corev1.Namespace, pod corev1.Pod) (corev1.Pod, error) {
	for _, instType := range stampedTypes {
		name, _ := InitContainerName(instType)
		if _, found := initContainerImage(pod, name); !found {
			continue
		}
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[LabelAutoInstrumented] = "true"
		setTerminationMessagePolicy(&pod, instType)
	}
	return pod, nil
}

// setTerminationMessagePolicy sets FallbackToLogsOnError on every application container that mounts
// the auto-instrumentation volume for a language, which makes the kubelet copy the tail of the
// container's log into status.*.terminated.message when it exits with an error. That status field is
// what lets the guard attribute a crash to auto-instrumentation without pods/log permissions.
// Containers are located BY MOUNT NAME, never by index, because the mutator chain ahead of this one
// may have reordered or added containers.
//
// Overwriting the field is safe. The API server defaults terminationMessagePolicy to File before
// webhooks run, so the value seen here is almost always that default rather than customer intent,
// and FallbackToLogsOnError is a strict superset of File anyway: it only engages when the container
// exits with an error AND /dev/termination-log is empty, so an application that writes its own
// message to /dev/termination-log still has that message reported.
func setTerminationMessagePolicy(pod *corev1.Pod, instType Type) {
	volume, ok := instrVolumeName(instType)
	if !ok {
		return
	}
	for i := range pod.Spec.Containers {
		for _, mount := range pod.Spec.Containers[i].VolumeMounts {
			if mount.Name == volume {
				pod.Spec.Containers[i].TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
				break
			}
		}
	}
}

func initContainerImage(pod corev1.Pod, name string) (string, bool) {
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return c.Image, true
		}
	}
	return "", false
}
