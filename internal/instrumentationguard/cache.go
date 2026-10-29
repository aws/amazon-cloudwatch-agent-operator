// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TrimPodForCache strips a Pod down to the fields the guard reads before controller-runtime
// commits it to the manager's cache. It is wired up in main.go as the Transform of the Pod entry
// in cache.Options.ByObject, alongside the label selector that limits the cache to
// auto-instrumented pods. The selector decides WHICH pods are cached; this decides how much of
// each one is kept. At the ~40,000 pods of a large cluster, the pod spec - containers, env,
// volumes, tolerations, the injected instrumentation's own env block - is the bulk of the bytes,
// and the guard needs none of it.
//
// What is kept, and who reads it:
//
//   - metadata.labels: the controller predicate and instrumentation.InjectedImages gate on
//     LabelAutoInstrumented.
//   - metadata.annotations: HasRecord. (The guard record lives on the WORKLOAD, but a pod's
//     annotations are cheap and the webhook chain reads them.)
//   - metadata.ownerReferences: ResolveWorkload walks them to the Deployment, StatefulSet or
//     DaemonSet.
//   - metadata.name / namespace: the reconcile request and every log line and Event.
//   - metadata.deletionTimestamp: AttributeFailure refuses to attribute a terminating pod's
//     failure to us, because a pod being drained, evicted, preempted or rolled gets its
//     containers signalled by something that is not auto-instrumentation.
//   - metadata.uid, resourceVersion, creationTimestamp: controller-runtime's own watch and
//     DeepCopy bookkeeping. Dropping resourceVersion in particular would break the cache.
//   - status.startTime: InWindow and the image-pull patience window.
//   - status.containerStatuses: EvaluatePod's restart and termination checks, and the termination
//     messages AttributeFailure parses.
//   - status.initContainerStatuses: instrumentation.InjectedImages (which is how the guard learns
//     the injected language and image at all) and the structural attribution path.
//
// Nothing in the controller reads pod spec. Verified by reading every consumer:
// instrumentation.InjectedImages reads status.initContainerStatuses; EvaluatePod and InWindow read
// status; AttributeFailure reads metadata.deletionTimestamp and status plus the mount-path
// constants reached through instrumentation.InstrMountPath; ResolveWorkload reads
// metadata.ownerReferences. The two
// functions that did read more are gone - IsReady read status.conditions and healthySibling listed
// sibling pods. &corev1.Pod{} appears only in this package's own controller registration, in
// main.go's cache configuration, and in a test table, so the guard is the only consumer of Pods
// through the manager's cached client.
//
// There is one corev1.PodList caller: autoInstrumentedPodNames, which lists a workload's
// auto-instrumented pods SOLELY to name them in the ManualPodDeletionRequired Event. It reads
// metadata.name and metadata.ownerReferences, both kept above, and it is served from this same
// label-filtered cache - the label selector in main.go is what makes it cheap, since it never sees
// a pod the guard was not already caching.
//
// The webhook is unaffected: it mutates the pod the API server sends it, not a cached copy.
func TrimPodForCache(obj interface{}) (interface{}, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		// A cache.DeletedFinalStateUnknown tombstone, or any type this is not meant for. Passing
		// it through unchanged is the only safe answer: returning an error here would make
		// controller-runtime drop the event.
		return obj, nil
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              pod.Name,
			Namespace:         pod.Namespace,
			UID:               pod.UID,
			ResourceVersion:   pod.ResourceVersion,
			CreationTimestamp: pod.CreationTimestamp,
			DeletionTimestamp: pod.DeletionTimestamp,
			Labels:            pod.Labels,
			Annotations:       pod.Annotations,
			OwnerReferences:   pod.OwnerReferences,
		},
		Status: corev1.PodStatus{
			StartTime:             pod.Status.StartTime,
			ContainerStatuses:     pod.Status.ContainerStatuses,
			InitContainerStatuses: pod.Status.InitContainerStatuses,
		},
	}, nil
}
