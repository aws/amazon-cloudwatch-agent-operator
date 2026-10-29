// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// Event reasons the guard emits on the affected workload. The guard is detect-only, so an Event
// is the customer's ONLY signal and the only thing the guard produces: every message names the pod
// that triggered it, the languages and images involved, and what the customer can change.
const (
	// ReasonInstrumentationSuspected is the guard's finding: this workload has an
	// auto-instrumented pod that broke, and the crash output points at the injected payload.
	ReasonInstrumentationSuspected = "InstrumentationSuspected"
	// ReasonManualPodDeletionRequired is emitted alongside ReasonInstrumentationSuspected on a
	// workload that does not replace its own pods, because on those a customer who disables
	// injection on the pod template still has to act by hand before anything changes. On most of
	// them that means deleting the pods (ManualPodDeletionRequiredMessage); on a paused Deployment
	// it means resuming the rollout instead, because deleting its pods does not help
	// (PausedDeploymentMessage). The guard never deletes a pod. See replacesPodsItself.
	ReasonManualPodDeletionRequired = "ManualPodDeletionRequired"
)

// InstrumentationSuspectedMessage reports the finding and hands the decision to the customer.
//
// The guard changes nothing, so this message has two jobs. It has to say what the operator
// believes and on what evidence - the pod, the languages and images, why the pod counts as broken
// and why auto-instrumentation is held responsible - and it has to say what the customer would
// change to stop the injection, because the operator will not do it for them. The inject
// annotation keys come from formatInjectKeys so the message names the same keys injection reads.
//
// It also says the finding can be wrong. Attribution is pattern matching on crash output and it
// has known false-positive sources (see the known limitations in README.md), so a message that
// read as a verdict would invite a customer to give up telemetry on the operator's word.
func InstrumentationSuspectedMessage(podName string, images map[instrumentation.Type]string, reason string) string {
	return fmt.Sprintf(
		"Auto-instrumentation (%s) is suspected of breaking this workload: pod %s: %s. The instrumentation guard only reports this - it has changed nothing on this workload and never will. If you agree auto-instrumentation is the cause, set the %s pod template annotations to \"false\" to stop injecting it; the application then runs without telemetry instead of crash-looping. This finding is based on pattern matching the pod's crash output and can be wrong, so read that output before you act on it.",
		formatImages(images), podName, reason, formatInjectKeys(images))
}

// eventMessageLimit is the number of bytes the guard keeps an Event message within.
//
// MEASURED against a real API server (envtest, Kubernetes v1.33.0), creating Events whose message
// ran from 1024 bytes up to 1 MiB:
//
//	core/v1          Event.message   accepted at EVERY size tried, up to and including 1048576
//	events.k8s.io/v1 Event.note      rejected at 1025: "can have at most 1024 characters"
//
// So the 1024-character cap is real but it belongs to events.k8s.io/v1, and the recorder main.go
// hands the guard - manager.GetEventRecorderFor, i.e. client-go's tools/record - writes core/v1,
// which validates no length at all. Nothing the guard emits is rejected today.
//
// The bound is therefore self-imposed, and it is set to the smaller of the two numbers for two
// reasons. main.go already carries a TODO to migrate to events.EventRecorder, and under that API
// an over-long message is not truncated but REJECTED, which would lose the Event silently and
// take the recovery instruction with it. And a message a customer has to read out of
// `kubectl describe` stops being useful long before a megabyte.
const eventMessageLimit = 1024

// maxNamedPods is how many pods the manual-deletion Event names before it summarises the rest as
// "and N more". A customer who has to delete pods by hand needs the command to run, and a command
// with dozens of names in it is no longer something to copy out of `kubectl describe`; the count
// still tells them how much is left.
const maxNamedPods = 5

// eventMessageTruncationMarker ends a message boundEventMessage had to cut.
const eventMessageTruncationMarker = "... (truncated)"

// ManualPodDeletionRequiredMessage tells the customer that disabling injection on this workload's
// pod template will not reach its running pods on its own, and names the pods they will have to
// delete. See replacesPodsItself for which workloads this applies to.
//
// triggeringPod is the bare name of the pod the guard found broken and is always named first,
// because it is the one pod the customer can already see in the suspected-instrumentation Event.
// otherPods are the workload's remaining auto-instrumented pods from autoInstrumentedPodNames; it may
// contain triggeringPod, and is de-duplicated and sorted here so the message does not change from
// one reconcile to the next. Every pod of a workload lives in the workload's own namespace, so one
// -n covers all of them.
func ManualPodDeletionRequiredMessage(workloadName, namespace, triggeringPod string, otherPods []string) string {
	named, remaining := namedPods(triggeringPod, otherPods)

	message := manualPodDeletionMessage(workloadName, namespace, named, remaining)
	for len(message) > eventMessageLimit && len(named) > 1 {
		// Dropping a name moves it into the "and N more" count, which is shorter than the name
		// itself plus its place in the command.
		named, remaining = named[:len(named)-1], remaining+1
		message = manualPodDeletionMessage(workloadName, namespace, named, remaining)
	}
	return boundEventMessage(message)
}

// PausedDeploymentMessage is the ReasonManualPodDeletionRequired message for a paused Deployment,
// where the action the customer needs is a resume and NOT a pod deletion.
//
// A paused Deployment creates no new ReplicaSet for a template change, so disabling injection on
// the template changes nothing while it stays paused - and deleting its pods changes nothing
// either, because the ReplicaSet it already has recreates them from the template that still
// injects. So this message asks for the resume and says why deleting pods is not the fix, which
// the generic message would otherwise have the customer try first.
//
// name is the Deployment's bare name, for the kubectl command; workloadName is the rendered
// "<type> <namespace>/<name>" the other messages open with.
func PausedDeploymentMessage(workloadName, namespace, name string) string {
	resume := "kubectl rollout resume deployment/" + name
	if namespace != "" {
		resume += " -n " + namespace
	}
	return boundEventMessage(fmt.Sprintf(
		"%s is paused (spec.paused is true), so it does not roll out a pod template change: disabling auto-instrumentation on the template will not reach the pods that are already running. Deleting those pods does not help either - while the Deployment is paused its existing ReplicaSet recreates them from the template that still injects. Disable auto-instrumentation on the template and then resume the rollout: %s. The instrumentation guard never deletes a pod, and has changed nothing on this workload.",
		workloadName, resume))
}

// manualPodDeletionMessage renders the message for an already-chosen set of names.
func manualPodDeletionMessage(workloadName, namespace string, named []string, remaining int) string {
	more := ""
	if remaining > 0 {
		more = fmt.Sprintf(", and %d more auto-instrumented pod(s) of this workload this Event has no room to name", remaining)
	}
	return fmt.Sprintf(
		"%s does not replace its pods when its pod template changes, so disabling auto-instrumentation on the template will not reach the pods that are already running. Every one of them has to be deleted for the change to take effect: %s%s. The instrumentation guard never deletes a pod, and has changed nothing on this workload.",
		workloadName, kubectlDeletePods(namespace, named), more)
}

// namedPods returns the pod names to put in the message, triggering pod first and the rest sorted,
// along with how many were left out by maxNamedPods.
func namedPods(triggeringPod string, otherPods []string) ([]string, int) {
	unique := make([]string, 0, len(otherPods)+1)
	seen := make(map[string]struct{}, len(otherPods)+1)
	for _, name := range append([]string{triggeringPod}, otherPods...) {
		if name == "" {
			continue
		}
		if _, repeated := seen[name]; repeated {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	if len(unique) > 1 {
		// The triggering pod keeps its place at the front; only the pods the List added are
		// sorted, because their order is whatever the cache happened to return.
		sort.Strings(unique[1:])
	}
	if len(unique) <= maxNamedPods {
		return unique, 0
	}
	return unique[:maxNamedPods], len(unique) - maxNamedPods
}

// kubectlDeletePods renders a runnable deletion command, so the customer can copy it straight out
// of `kubectl describe`.
func kubectlDeletePods(namespace string, names []string) string {
	command := "kubectl delete pod " + strings.Join(names, " ")
	if namespace != "" {
		command += " -n " + namespace
	}
	return command
}

// boundEventMessage is the last-resort guarantee of eventMessageLimit, for the case the loop in
// ManualPodDeletionRequiredMessage cannot fix: a single pod name can be 253 characters and a
// workload name another 253, so even one named pod can overrun the bound. Dropping that last name
// would leave a command with nothing to delete, so the message is cut instead - the customer still
// gets the workload and the first pod, which is where the prose puts them.
//
// Cutting bytes cannot split a character here: pod and workload names are DNS names, and the rest
// of every message in this file is ASCII prose.
func boundEventMessage(message string) string {
	if len(message) <= eventMessageLimit {
		return message
	}
	return message[:eventMessageLimit-len(eventMessageTruncationMarker)] + eventMessageTruncationMarker
}

// formatImages renders a language-to-image map as "java=<image>, python=<image>", sorted by
// language so the message is stable.
func formatImages(images map[instrumentation.Type]string) string {
	parts := make([]string, 0, len(images))
	for instType, image := range images {
		parts = append(parts, fmt.Sprintf("%s=%s", instType, image))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// formatInjectKeys renders the inject annotation keys for the injected languages, sorted. These
// are the keys the customer sets to "false" to stop the injection; the guard only names them.
func formatInjectKeys(images map[instrumentation.Type]string) string {
	parts := make([]string, 0, len(images))
	for instType := range images {
		if key := instrumentation.InjectAnnotationKey(instType); key != "" {
			parts = append(parts, key)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
