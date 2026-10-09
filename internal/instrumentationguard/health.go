// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// reasonOOMKilled is the container termination reason the kubelet reports for an out-of-memory kill.
const reasonOOMKilled = "OOMKilled"

// Verdict is the outcome of evaluating one pod. Reason is empty when the pod is not broken, and
// otherwise is a short human-readable explanation that ends up in an Event and in the guard record.
type Verdict struct {
	Broken bool
	Reason string
}

// EvaluatePod decides whether a pod is broken in a way the guard attributes to
// auto-instrumentation. It is pure: it reads only the pod and the configuration.
//
// A pod is never broken outside the window, because a failure long after the pod started is far
// more likely to be the application's own doing. Inside the window, the pod is broken when an
// application container is over the restart threshold or was OOMKilled, or when one of the
// auto-instrumentation init containers failed, restarted too often, or cannot pull its image. The
// CloudWatch agent sidecar is excluded, and so is every init container the operator did not
// inject.
//
// EvaluatePod decides only THAT the pod is broken. AttributeFailure decides whether
// auto-instrumentation is why.
func EvaluatePod(pod corev1.Pod, now time.Time, cfg Config) Verdict {
	if !InWindow(pod, now, cfg.Window) {
		return Verdict{}
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == naming.Container() {
			// The CloudWatch agent sidecar is not auto-instrumentation; its restarts say nothing
			// about the ADOT payload.
			continue
		}
		if status.RestartCount >= cfg.RestartThreshold {
			return Verdict{
				Broken: true,
				Reason: fmt.Sprintf("container %s restarted %d times%s", status.Name, status.RestartCount, waitingSuffix(status)),
			}
		}
		if reason, ok := oomKilled(status); ok {
			return Verdict{
				Broken: true,
				Reason: fmt.Sprintf("container %s was %s", status.Name, reason),
			}
		}
	}

	initContainers := injectedInitContainerNames(pod)
	for _, status := range pod.Status.InitContainerStatuses {
		instType, injected := initContainers[status.Name]
		if !injected {
			// A customer's own init container never triggers the guard.
			continue
		}
		if terminated := status.State.Terminated; terminated != nil && terminated.ExitCode != 0 {
			return Verdict{
				Broken: true,
				Reason: fmt.Sprintf("auto-instrumentation init container %s (%s) exited with code %d", status.Name, instType, terminated.ExitCode),
			}
		}
		if status.RestartCount >= cfg.RestartThreshold {
			return Verdict{
				Broken: true,
				Reason: fmt.Sprintf("auto-instrumentation init container %s (%s) restarted %d times%s", status.Name, instType, status.RestartCount, waitingSuffix(status)),
			}
		}
		// A pull failure stalls the pod in init forever, so it has to be a trigger of its own:
		// nothing terminates and no restart is counted. It gets more patience than every other
		// trigger, because a failed copy is deterministic while a failed pull is often a transient
		// registry problem. InWindow above guarantees StartTime is set.
		if waiting := status.State.Waiting; waiting != nil && isImagePullFailure(waiting.Reason) &&
			now.Sub(pod.Status.StartTime.Time) >= cfg.ImagePullPatience {
			return Verdict{
				Broken: true,
				Reason: fmt.Sprintf("auto-instrumentation init container %s (%s) cannot pull image %s (%s)", status.Name, instType, status.Image, waiting.Reason),
			}
		}
	}

	return Verdict{}
}

// InWindow reports whether the pod started recently enough for the guard to consider it. A pod
// without a start time has not been scheduled yet and is never in the window.
func InWindow(pod corev1.Pod, now time.Time, window time.Duration) bool {
	startTime := pod.Status.StartTime
	if startTime == nil {
		return false
	}
	return now.Sub(startTime.Time) <= window
}

// injectedInitContainerNames maps the name of each init container the operator injected into this
// pod to the language it carries.
func injectedInitContainerNames(pod corev1.Pod) map[string]instrumentation.Type {
	names := map[string]instrumentation.Type{}
	for instType := range instrumentation.InjectedImages(pod) {
		if name, ok := instrumentation.InitContainerName(instType); ok {
			names[name] = instType
		}
	}
	return names
}

// oomKilled reports whether the container was killed for being out of memory, either in its
// current state or in the state it was in before the last restart.
func oomKilled(status corev1.ContainerStatus) (string, bool) {
	if terminated := status.State.Terminated; terminated != nil && terminated.Reason == reasonOOMKilled {
		return terminated.Reason, true
	}
	if terminated := status.LastTerminationState.Terminated; terminated != nil && terminated.Reason == reasonOOMKilled {
		return terminated.Reason, true
	}
	return "", false
}

// waitingSuffix annotates a reason with why the container is currently waiting, e.g.
// " (CrashLoopBackOff)". The waiting reason is never a trigger on its own; it is context.
func waitingSuffix(status corev1.ContainerStatus) string {
	if waiting := status.State.Waiting; waiting != nil && waiting.Reason != "" {
		return fmt.Sprintf(" (%s)", waiting.Reason)
	}
	return ""
}
