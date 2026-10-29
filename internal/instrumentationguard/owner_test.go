// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testNamespace = "default"

func testSelector(app string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: map[string]string{"app": app}}
}

func newTestDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: appsv1.DeploymentSpec{
			Selector: testSelector(name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
			},
		},
	}
}

func newTestReplicaSet(name string, deploymentName string) *appsv1.ReplicaSet {
	replicaSet := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec:       appsv1.ReplicaSetSpec{Selector: testSelector(deploymentName)},
	}
	if deploymentName != "" {
		replicaSet.OwnerReferences = []metav1.OwnerReference{ownerRef("Deployment", deploymentName)}
	}
	return replicaSet
}

func newTestStatefulSet(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: appsv1.StatefulSetSpec{
			Selector: testSelector(name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
			},
		},
	}
}

func newTestDaemonSet(name string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: testSelector(name),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
			},
		},
	}
}

func ownerRef(kind, name string) metav1.OwnerReference {
	return metav1.OwnerReference{APIVersion: "apps/v1", Kind: kind, Name: name}
}

// controllerRef is the owner reference a controller sets on the objects it creates: the same as
// ownerRef, flagged as the controlling one.
func controllerRef(kind, name string) metav1.OwnerReference {
	ref := ownerRef(kind, name)
	isController := true
	ref.Controller = &isController
	return ref
}

// withUID sets an object's UID, which the fake client does not generate. The owner walk matches on
// it, so the cases that exercise that matching have to set it on both sides.
func withUID[T client.Object](obj T, uid types.UID) T {
	obj.SetUID(uid)
	return obj
}

// withRefUID sets the UID on an owner reference, the other side of the same match.
func withRefUID(ref metav1.OwnerReference, uid types.UID) metav1.OwnerReference {
	ref.UID = uid
	return ref
}

func ownedBy(pod corev1.Pod, kind, name string) corev1.Pod {
	pod.OwnerReferences = []metav1.OwnerReference{ownerRef(kind, name)}
	return pod
}

func ownedByRefs(pod corev1.Pod, refs ...metav1.OwnerReference) corev1.Pod {
	pod.OwnerReferences = refs
	return pod
}

// fastWorkloadGetBackOff shortens the owner-walk retry so the NotFound cases do not spend seconds
// backing off against a fake client that will never produce the object.
func fastWorkloadGetBackOff(t *testing.T) {
	t.Helper()
	original := workloadGetBackOff
	workloadGetBackOff = wait.Backoff{Duration: time.Millisecond, Steps: 2}
	t.Cleanup(func() { workloadGetBackOff = original })
}

func TestResolveWorkload(t *testing.T) {
	fastWorkloadGetBackOff(t)

	for _, tt := range []struct {
		name    string
		objects []client.Object
		pod     corev1.Pod
		want    client.Object
	}{
		{
			name:    "pod to replicaset to deployment",
			objects: []client.Object{newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders")},
			pod:     ownedBy(stampedPod("orders-7d9f-abc", nil), "ReplicaSet", "orders-7d9f"),
			want:    newTestDeployment("orders"),
		},
		{
			name:    "pod to statefulset",
			objects: []client.Object{newTestStatefulSet("carts")},
			pod:     ownedBy(stampedPod("carts-0", nil), "StatefulSet", "carts"),
			want:    newTestStatefulSet("carts"),
		},
		{
			name:    "pod to daemonset",
			objects: []client.Object{newTestDaemonSet("agents")},
			pod:     ownedBy(stampedPod("agents-xyz", nil), "DaemonSet", "agents"),
			want:    newTestDaemonSet("agents"),
		},
		{
			name: "bare pod",
			pod:  stampedPod("standalone", nil),
		},
		{
			name: "pod owned by a job",
			objects: []client.Object{&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "backfill", Namespace: testNamespace},
			}},
			pod: ownedBy(stampedPod("backfill-abc", nil), "Job", "backfill"),
		},
		{
			name:    "replicaset with no deployment owner",
			objects: []client.Object{newTestReplicaSet("orphan-rs", "")},
			pod:     ownedBy(stampedPod("orphan-rs-abc", nil), "ReplicaSet", "orphan-rs"),
		},
		{
			name: "replicaset that no longer exists",
			pod:  ownedBy(stampedPod("gone-abc", nil), "ReplicaSet", "gone"),
		},
		{
			name:    "deployment that no longer exists",
			objects: []client.Object{newTestReplicaSet("orders-7d9f", "orders")},
			pod:     ownedBy(stampedPod("orders-7d9f-abc", nil), "ReplicaSet", "orders-7d9f"),
		},
		{
			name:    "pod owned by a same-named kind from another api group",
			objects: []client.Object{newTestStatefulSet("carts")},
			pod: ownedByRefs(stampedPod("carts-0", nil),
				metav1.OwnerReference{APIVersion: "example.com/v1", Kind: "StatefulSet", Name: "carts"}),
		},
		{
			name: "replicaset owned by a deployment from another api group",
			objects: []client.Object{newTestDeployment("orders"), func() client.Object {
				replicaSet := newTestReplicaSet("orders-7d9f", "")
				replicaSet.OwnerReferences = []metav1.OwnerReference{
					{APIVersion: "example.com/v1", Kind: "Deployment", Name: "orders"},
				}
				return replicaSet
			}()},
			pod: ownedBy(stampedPod("orders-7d9f-abc", nil), "ReplicaSet", "orders-7d9f"),
		},
		{
			// The UID is what identifies an owner; the name is reusable. A StatefulSet that was
			// deleted and recreated under the same name is a different object, and a pod left
			// over from the old one is nothing the new one owns - so the guard must not address
			// an Event to it.
			name:    "pod whose statefulset was replaced by a same-named one",
			objects: []client.Object{withUID(newTestStatefulSet("carts"), "carts-uid-2")},
			pod: ownedByRefs(stampedPod("carts-0", nil),
				withRefUID(controllerRef("StatefulSet", "carts"), "carts-uid-1")),
		},
		{
			name:    "pod whose daemonset was replaced by a same-named one",
			objects: []client.Object{withUID(newTestDaemonSet("agents"), "agents-uid-2")},
			pod: ownedByRefs(stampedPod("agents-xyz", nil),
				withRefUID(controllerRef("DaemonSet", "agents"), "agents-uid-1")),
		},
		{
			name:    "pod whose replicaset was replaced by a same-named one",
			objects: []client.Object{newTestDeployment("orders"), withUID(newTestReplicaSet("orders-7d9f", "orders"), "rs-uid-2")},
			pod: ownedByRefs(stampedPod("orders-7d9f-abc", nil),
				withRefUID(controllerRef("ReplicaSet", "orders-7d9f"), "rs-uid-1")),
		},
		{
			// The second hop needs the same check: an old ReplicaSet outlives the Deployment that
			// made it, and a Deployment created afterwards with that name never owned it.
			name: "replicaset whose deployment was replaced by a same-named one",
			objects: []client.Object{withUID(newTestDeployment("orders"), "deploy-uid-2"), func() client.Object {
				replicaSet := newTestReplicaSet("orders-7d9f", "")
				replicaSet.UID = "rs-uid"
				replicaSet.OwnerReferences = []metav1.OwnerReference{
					withRefUID(controllerRef("Deployment", "orders"), "deploy-uid-1"),
				}
				return replicaSet
			}()},
			pod: ownedByRefs(stampedPod("orders-7d9f-abc", nil),
				withRefUID(controllerRef("ReplicaSet", "orders-7d9f"), "rs-uid")),
		},
		{
			// The whole walk with matching UIDs at both hops, so the check above is not passing
			// by refusing everything.
			name: "pod to replicaset to deployment with matching uids",
			objects: []client.Object{withUID(newTestDeployment("orders"), "deploy-uid"), func() client.Object {
				replicaSet := newTestReplicaSet("orders-7d9f", "")
				replicaSet.UID = "rs-uid"
				replicaSet.OwnerReferences = []metav1.OwnerReference{
					withRefUID(controllerRef("Deployment", "orders"), "deploy-uid"),
				}
				return replicaSet
			}()},
			pod: ownedByRefs(stampedPod("orders-7d9f-abc", nil),
				withRefUID(controllerRef("ReplicaSet", "orders-7d9f"), "rs-uid")),
			want: newTestDeployment("orders"),
		},
		{
			name:    "controller reference wins over a plain owner reference",
			objects: []client.Object{newTestStatefulSet("carts"), newTestDaemonSet("agents")},
			pod: ownedByRefs(stampedPod("carts-0", nil),
				ownerRef("DaemonSet", "agents"),
				controllerRef("StatefulSet", "carts")),
			want: newTestStatefulSet("carts"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithObjects(tt.objects...).Build()

			got, err := ResolveWorkload(context.Background(), c, tt.pod)
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.IsType(t, tt.want, got)
			assert.Equal(t, tt.want.GetName(), got.GetName())
			assert.Equal(t, tt.want.GetNamespace(), got.GetNamespace())
		})
	}
}

// TestReplacesPodsItself pins the self-healing table measured on a live cluster at Kubernetes
// v1.34.8; see replacesPodsItself for the measurement itself. A StatefulSet on its DEFAULT update
// strategy is the surprising row and the reason the guard emits ManualPodDeletionRequired at all.
func TestReplacesPodsItself(t *testing.T) {
	for _, tt := range []struct {
		name string
		obj  client.Object
		want bool
	}{
		{
			name: "Deployment, default strategy",
			obj:  newTestDeployment("orders"),
			want: true,
		},
		{
			name: "Deployment, RollingUpdate",
			obj: func() client.Object {
				deployment := newTestDeployment("orders")
				deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
				return deployment
			}(),
			want: true,
		},
		{
			name: "Deployment, Recreate",
			obj: func() client.Object {
				deployment := newTestDeployment("orders")
				deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
				return deployment
			}(),
			want: true,
		},
		{
			// spec.paused stops the Deployment controller acting on the spec, so a template
			// change creates no new ReplicaSet and the broken pods stay as they are.
			name: "Deployment, paused",
			obj: func() client.Object {
				deployment := newTestDeployment("orders")
				deployment.Spec.Paused = true
				return deployment
			}(),
		},
		{
			name: "DaemonSet, default strategy",
			obj:  newTestDaemonSet("agents"),
			want: true,
		},
		{
			name: "DaemonSet, RollingUpdate",
			obj: func() client.Object {
				daemonSet := newTestDaemonSet("agents")
				daemonSet.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType}
				return daemonSet
			}(),
			want: true,
		},
		{
			name: "DaemonSet, OnDelete",
			obj:  onDeleteDaemonSet("agents"),
		},
		{
			name: "StatefulSet, default strategy",
			// The default is RollingUpdate with OrderedReady pod management, which stops on the
			// pod that never becomes Ready - the broken one.
			obj: newTestStatefulSet("carts"),
		},
		{
			name: "StatefulSet, RollingUpdate",
			obj: func() client.Object {
				statefulSet := newTestStatefulSet("carts")
				statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
				return statefulSet
			}(),
		},
		{
			name: "StatefulSet, OnDelete",
			obj: func() client.Object {
				statefulSet := newTestStatefulSet("carts")
				statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.OnDeleteStatefulSetStrategyType}
				return statefulSet
			}(),
		},
		{
			name: "a kind the guard does not report on",
			obj:  newTestReplicaSet("orders-7d9f", "orders"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, replacesPodsItself(tt.obj))
		})
	}
}

// onDeleteDaemonSet is the one workload shape whose self-healing depends on a field, so the pod
// reconciler's Event test needs it too.
func onDeleteDaemonSet(name string) *appsv1.DaemonSet {
	daemonSet := newTestDaemonSet(name)
	daemonSet.Spec.UpdateStrategy = appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType}
	return daemonSet
}
