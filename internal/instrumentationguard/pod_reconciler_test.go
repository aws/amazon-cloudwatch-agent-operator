// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

const testImageJava = "public.ecr.aws/aws-observability/adot-autoinstrumentation-java:v0.0.1"

// newerPythonImage is a configured default the pod's injected image has fallen behind: the
// operator has since been restarted with it, so a pod still carrying testImagePython is a leftover
// from a superseded rollout.
const newerPythonImage = "public.ecr.aws/aws-observability/adot-autoinstrumentation-python:v0.0.2"

// pythonInjectKey is the pod template annotation the guard sets to "false" for python.
var pythonInjectKey = instrumentation.InjectAnnotationKey(instrumentation.TypePython)

// guiltyAttribution is the reason AttributeFailure produces for the case-6 fixture crashingPod
// carries. The reconciler folds it into the record and into the Events.
const guiltyAttribution = "container app: auto-instrumentation payload failed while being imported: /otel-auto-instrumentation-python/urllib3/__init__.py line 15"

// crashingPod builds a stamped pod whose application container is over the restart threshold and
// died with the REAL captured crash output of a pod the injected payload broke, so the reconciler
// paths are driven by the same bytes attribution was validated against.
func crashingPod(t *testing.T, name string, images map[instrumentation.Type]string) corev1.Pod {
	t.Helper()
	return crashingPodWithMessage(name, images, readFixture(t, fixtureGuilty))
}

// crashingPodNotOurs builds a pod that is just as broken, but whose REAL captured crash output
// names our files only in pass-through call frames: the application dialled a dead port of its own
// accord. The guard must not touch its workload.
func crashingPodNotOurs(t *testing.T, name string, images map[instrumentation.Type]string) corev1.Pod {
	t.Helper()
	return crashingPodWithMessage(name, images, readFixture(t, fixtureInnocent))
}

func crashingPodWithMessage(name string, images map[instrumentation.Type]string, message string) corev1.Pod {
	pod := withStartTime(stampedPod(name, images), testNow.Add(-time.Minute))
	pod.Labels["app"] = "orders"
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:         testAppContainer,
		RestartCount: 3,
		State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			ExitCode: 1,
			Message:  message,
		}},
	}}
	return pod
}

// readyPod builds a stamped pod that is running normally.
func readyPod(name string, images map[instrumentation.Type]string) corev1.Pod {
	pod := withStartTime(stampedPod(name, images), testNow.Add(-time.Minute))
	pod.Labels["app"] = "orders"
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  testAppContainer,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}
	return pod
}

func newPodReconcilerForTest(t *testing.T, mode Mode, objects ...client.Object) (*PodReconciler, client.Client, *record.FakeRecorder) {
	t.Helper()
	fastWorkloadGetBackOff(t)

	c := fake.NewClientBuilder().WithObjects(objects...).Build()
	recorder := record.NewFakeRecorder(10)
	cfg := DefaultConfig()
	cfg.Mode = mode

	r := NewPodReconciler(c, testr.New(t), recorder, cfg)
	r.now = func() time.Time { return testNow }
	return r, c, recorder
}

func reconcilePod(t *testing.T, r *PodReconciler, pod corev1.Pod) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name},
	})
	require.NoError(t, err)
	return result
}

func getDeployment(t *testing.T, c client.Client, name string) *appsv1.Deployment {
	t.Helper()
	deployment := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: testNamespace, Name: name}, deployment))
	return deployment
}

func drainEvents(recorder *record.FakeRecorder) []string {
	events := []string{}
	for {
		select {
		case event := <-recorder.Events:
			events = append(events, event)
		default:
			return events
		}
	}
}

// TestPodReconciler_ReportsWithoutWritingAnything is the guard's whole behaviour: one Event, and
// the workload exactly as it was. The deep-equality check on the Deployment is what pins
// detect-only - it fails on any annotation, template or status write, not just on the one the
// guard used to make.
func TestPodReconciler_ReportsWithoutWritingAnything(t *testing.T) {
	images := map[instrumentation.Type]string{
		instrumentation.TypePython: testImagePython,
		instrumentation.TypeJava:   testImageJava,
	}
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	deployment := newTestDeployment("orders")
	// python was explicitly opted in; java was inherited from the namespace, so the template has
	// no annotation for it at all. Neither may be touched.
	deployment.Spec.Template.Annotations = map[string]string{pythonInjectKey: "true"}
	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, deployment, newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))

	after := getDeployment(t, c, "orders")
	assert.Equal(t, before, after, "the guard must not write to a customer's workload")
	assert.Equal(t, "true", after.Spec.Template.Annotations[pythonInjectKey],
		"the inject annotation the customer set is the customer's")
	assert.False(t, HasRecord(after), "the guard writes no record")

	events := drainEvents(recorder)
	require.Len(t, events, 1, "a Deployment replaces its own pods, so there is nothing more to say")
	assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
	assert.Contains(t, events[0], "orders-7d9f-abc")
	assert.Contains(t, events[0], "python="+testImagePython)
	assert.Contains(t, events[0], "java="+testImageJava)
	assert.Contains(t, events[0], "container app restarted 3 times (CrashLoopBackOff)")
	assert.Contains(t, events[0], guiltyAttribution, "the Event must say why the guard thinks this is ours")
	// The Event is the only output, so it carries the whole recovery instruction: the keys the
	// customer sets to "false", for every language that was injected.
	assert.Contains(t, events[0], pythonInjectKey)
	assert.Contains(t, events[0], instrumentation.InjectAnnotationKey(instrumentation.TypeJava))
	assert.Contains(t, events[0], "can be wrong", "the Event must not read as a verdict")
}

// TestPodReconciler_ModeOffReportsNothing covers the reconciler's own mode check. main.go
// registers no controller for off, so this is belt and braces: a Config that says off must produce
// nothing even if something does call Reconcile.
func TestPodReconciler_ModeOffReportsNothing(t *testing.T) {
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), "ReplicaSet", "orders-7d9f")
	r, c, recorder := newPodReconcilerForTest(t, ModeOff,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_SupersededPodIsLeftAlone(t *testing.T) {
	// The operator has already been restarted with a newer default, so this pod is a leftover
	// from the rollout before that. Reporting on its evidence would name an image the operator
	// has moved past and send the customer after a failure injection no longer causes.
	t.Setenv("AUTO_INSTRUMENTATION_PYTHON", newerPythonImage)

	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder), "a pod from a superseded rollout is not worth an Event")
}

func TestPodReconciler_ReportsAPodOnTheCurrentDefault(t *testing.T) {
	// The counterpart to the test above: the comparison must not stop the guard working on a
	// cluster whose kubelet reports the configured tag resolved to a digest, which is the
	// commonest real difference between the two strings.
	t.Setenv("AUTO_INSTRUMENTATION_PYTHON", testImagePython)

	images := map[instrumentation.Type]string{
		instrumentation.TypePython: testImagePython + "@sha256:f3b0c9a1d4e5f6a7b8c9d0e1f2a3b4c5",
	}
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	reconcilePod(t, r, pod)

	events := drainEvents(recorder)
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
}

func TestPodReconciler_BrokenPodThatIsNotOursIsLeftAlone(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	pod := ownedBy(crashingPodNotOurs(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	result := reconcilePod(t, r, pod)

	assert.Equal(t, time.Duration(0), result.RequeueAfter, "the evidence does not improve by looking again")
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_AnUnattributedFailureSkipsTheOwnerWalk(t *testing.T) {
	fastWorkloadGetBackOff(t)

	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	pod := ownedBy(crashingPodNotOurs(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	replicaSetGets := 0
	c := fake.NewClientBuilder().
		WithObjects(newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, inner client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*appsv1.ReplicaSet); ok {
					replicaSetGets++
				}
				return inner.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	cfg := DefaultConfig()
	cfg.Mode = ModeDryRun
	r := NewPodReconciler(c, testr.New(t), record.NewFakeRecorder(10), cfg)
	r.now = func() time.Time { return testNow }

	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Zero(t, replicaSetGets, "attribution runs before the owner walk, so an unattributed failure costs no API Get")
}

// TestPodReconciler_WorkloadWithARecordIsLeftAlone covers the early exit on HasRecord. The guard
// never writes that annotation, so it is the customer's: a workload carrying it, with injection
// still turned off on its template, is one they have already dealt with and the guard has nothing
// to add.
func TestPodReconciler_WorkloadWithARecordIsLeftAlone(t *testing.T) {
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), "ReplicaSet", "orders-7d9f")
	deployment := newTestDeployment("orders")
	deployment.Annotations = map[string]string{RecordAnnotationKey: exampleRecordJSON}
	deployment.Spec.Template.Annotations = map[string]string{pythonInjectKey: "false"}

	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, deployment, newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder))
}

// TestPodReconciler_WorkloadWithARecordButInjectionBackOnIsReported is the other half of that exit,
// and the case that made it wrong on its own: the record annotation is still there but the inject
// annotation has returned to "true". `kubectl apply`, Argo CD and Flux all restore the annotations
// their manifest manages and none of them removes the record, so this is the ordinary state of a
// workload that was silenced once and has since been reconciled. The guard must not go quiet about
// pods that are breaking again.
func TestPodReconciler_WorkloadWithARecordButInjectionBackOnIsReported(t *testing.T) {
	for _, tt := range []struct {
		name                string
		templateAnnotations map[string]string
	}{
		{
			name:                "inject annotation restored to true",
			templateAnnotations: map[string]string{pythonInjectKey: "true"},
		},
		{
			// Nothing on the template says no, so injection is coming from the namespace or from
			// auto-monitor, and the pod in hand proves it is still happening.
			name:                "inject annotation gone from the template",
			templateAnnotations: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pod := ownedBy(crashingPod(t, "orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), "ReplicaSet", "orders-7d9f")
			deployment := newTestDeployment("orders")
			deployment.Annotations = map[string]string{RecordAnnotationKey: exampleRecordJSON}
			deployment.Spec.Template.Annotations = tt.templateAnnotations

			r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, deployment, newTestReplicaSet("orders-7d9f", "orders"), &pod)

			before := getDeployment(t, c, "orders")
			assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
			assert.Equal(t, before, getDeployment(t, c, "orders"), "reporting again is still read-only")

			events := drainEvents(recorder)
			require.Len(t, events, 1)
			assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
			assert.Contains(t, events[0], "orders-7d9f-abc")
		})
	}
}

// TestPodReconciler_PausedDeploymentAsksForAResume covers the paused Deployment: it does not roll
// out a template change, so it gets the second Event like a StatefulSet does - but the action in it
// is a resume, because deleting the pods of a paused Deployment just has its existing ReplicaSet
// recreate them from the template that still injects.
func TestPodReconciler_PausedDeploymentAsksForAResume(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	deployment := newTestDeployment("orders")
	deployment.Spec.Paused = true

	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, deployment, newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Equal(t, before, getDeployment(t, c, "orders"), "the guard must not resume the Deployment itself")

	events := drainEvents(recorder)
	require.Len(t, events, 2)
	assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
	assert.Contains(t, events[1], "Warning "+ReasonManualPodDeletionRequired)
	assert.Contains(t, events[1], "*v1.Deployment default/orders")
	assert.Contains(t, events[1], "kubectl rollout resume deployment/orders -n default")
	assert.Contains(t, events[1], "Deleting those pods does not help")
	assert.NotContains(t, events[1], "kubectl delete pod", "deleting the pods of a paused Deployment is not the fix")
}

// TestPodReconciler_RunningDeploymentGetsOneEventOnly is the control for the test above: without
// spec.paused a Deployment rolls out the customer's template change itself, so there is no second
// Event to emit.
func TestPodReconciler_RunningDeploymentGetsOneEventOnly(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	pod := ownedBy(crashingPod(t, "orders-7d9f-abc", images), "ReplicaSet", "orders-7d9f")

	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	reconcilePod(t, r, pod)

	events := drainEvents(recorder)
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
}

func TestPodReconciler_UnstampedPodIsANoOp(t *testing.T) {
	pod := crashingPod(t, "orders-7d9f-abc", nil)
	delete(pod.Labels, instrumentation.LabelAutoInstrumented)
	pod = ownedBy(pod, "ReplicaSet", "orders-7d9f")

	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_BarePodIsANoOp(t *testing.T) {
	pod := crashingPod(t, "standalone", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun, &pod)

	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_DeletedPodIsANoOp(t *testing.T) {
	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: testNamespace, Name: "gone"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_HealthyPodInsideTheWindowIsRequeued(t *testing.T) {
	pod := ownedBy(readyPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}), "ReplicaSet", "orders-7d9f")
	r, c, recorder := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	before := getDeployment(t, c, "orders")
	result := reconcilePod(t, r, pod)
	assert.Equal(t, ctrl.Result{RequeueAfter: DefaultConfig().RequeueInterval}, result)
	assert.Equal(t, before, getDeployment(t, c, "orders"))
	assert.Empty(t, drainEvents(recorder))
}

func TestPodReconciler_HealthyPodOutsideTheWindowIsDone(t *testing.T) {
	pod := readyPod("orders-7d9f-abc", map[instrumentation.Type]string{instrumentation.TypePython: testImagePython})
	startTime := metav1.NewTime(testNow.Add(-time.Hour))
	pod.Status.StartTime = &startTime
	pod = ownedBy(pod, "ReplicaSet", "orders-7d9f")

	r, _, _ := newPodReconcilerForTest(t, ModeDryRun,
		newTestDeployment("orders"), newTestReplicaSet("orders-7d9f", "orders"), &pod)

	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, pod))
}

func TestPodReconciler_ReportsOnStatefulSetsAndDaemonSets(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}

	t.Run("StatefulSet", func(t *testing.T) {
		pod := ownedBy(crashingPod(t, "carts-0", images), "StatefulSet", "carts")
		r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, newTestStatefulSet("carts"), &pod)

		before := getStatefulSet(t, c, "carts")
		reconcilePod(t, r, pod)
		assert.Equal(t, before, getStatefulSet(t, c, "carts"), "the guard must not write to a customer's workload")

		// A StatefulSet does not replace its pods on its own, even on the default update
		// strategy, so a customer who disables injection on the template is still left with the
		// broken pods running. The guard does not delete pods, so it says which ones to delete.
		events := drainEvents(recorder)
		require.Len(t, events, 2)
		assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
		assert.Contains(t, events[1], "Warning "+ReasonManualPodDeletionRequired)
		assert.Contains(t, events[1], "*v1.StatefulSet default/carts")
		// This pod carries a plain owner reference, not a controller one, so it is not in the
		// List's result and the Event names the triggering pod alone. See
		// autoInstrumentedPodNames.
		assert.Contains(t, events[1], "kubectl delete pod carts-0 -n default")
	})

	t.Run("DaemonSet", func(t *testing.T) {
		pod := ownedBy(crashingPod(t, "agents-xyz", images), "DaemonSet", "agents")
		r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, newTestDaemonSet("agents"), &pod)

		before := getDaemonSet(t, c, "agents")
		reconcilePod(t, r, pod)
		assert.Equal(t, before, getDaemonSet(t, c, "agents"))

		events := drainEvents(recorder)
		require.Len(t, events, 1, "a DaemonSet on the default strategy rolls its own pods")
		assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
	})

	t.Run("DaemonSet with updateStrategy OnDelete", func(t *testing.T) {
		pod := ownedBy(crashingPod(t, "agents-xyz", images), "DaemonSet", "agents")
		r, c, recorder := newPodReconcilerForTest(t, ModeDryRun, onDeleteDaemonSet("agents"), &pod)

		before := getDaemonSet(t, c, "agents")
		reconcilePod(t, r, pod)
		assert.Equal(t, before, getDaemonSet(t, c, "agents"))

		events := drainEvents(recorder)
		require.Len(t, events, 2)
		assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected)
		assert.Contains(t, events[1], "Warning "+ReasonManualPodDeletionRequired)
		assert.Contains(t, events[1], "kubectl delete pod agents-xyz -n default")
	})
}

// ownedByController sets the controlling owner reference a workload's own controller writes,
// including the UID autoInstrumentedPodNames matches on. The fake client generates no UIDs, so a
// test that left them unset would have every pod in the namespace match every workload and would
// prove nothing about the filtering.
func ownedByController(pod corev1.Pod, kind, name string, uid types.UID) corev1.Pod {
	ref := controllerRef(kind, name)
	ref.UID = uid
	pod.OwnerReferences = []metav1.OwnerReference{ref}
	return pod
}

// TestPodReconciler_ManualDeletionEventNamesEveryPodOfTheWorkload covers the pod List behind the
// Event: on a StatefulSet nothing is replaced until EVERY pod is deleted, so naming only the pod
// the guard found broken would have the customer fix one pod and leave the rest down.
func TestPodReconciler_ManualDeletionEventNamesEveryPodOfTheWorkload(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}

	carts := newTestStatefulSet("carts")
	carts.UID = "carts-uid"
	baskets := newTestStatefulSet("baskets")
	baskets.UID = "baskets-uid"

	trigger := ownedByController(crashingPod(t, "carts-0", images), "StatefulSet", "carts", carts.UID)
	sibling := ownedByController(readyPod("carts-1", images), "StatefulSet", "carts", carts.UID)
	// Broken too, and just as much in need of deleting: the guard reacts to one pod but the
	// finding has to cover all of them.
	brokenSibling := ownedByController(crashingPod(t, "carts-2", images), "StatefulSet", "carts", carts.UID)

	// Excluded, in the same namespace: a pod this operator never injected into; a pod of a
	// DIFFERENT workload; and a pod of a workload with the same name and kind but a different UID,
	// which is what a deleted-and-recreated StatefulSet leaves behind. The last one is excluded by
	// its UID alone, which is why the match is on UID and not on name.
	unlabelled := ownedByController(readyPod("carts-3", images), "StatefulSet", "carts", carts.UID)
	delete(unlabelled.Labels, instrumentation.LabelAutoInstrumented)
	foreign := ownedByController(readyPod("baskets-0", images), "StatefulSet", "baskets", baskets.UID)
	stale := ownedByController(readyPod("carts-9", images), "StatefulSet", "carts", "an-older-carts-uid")

	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun,
		carts, baskets, &trigger, &sibling, &brokenSibling, &unlabelled, &foreign, &stale)

	reconcilePod(t, r, trigger)

	events := drainEvents(recorder)
	require.Len(t, events, 2)
	message := events[1]
	assert.Contains(t, message, "Warning "+ReasonManualPodDeletionRequired)
	assert.Contains(t, message, "kubectl delete pod carts-0 carts-1 carts-2 -n default")
	assert.NotContains(t, message, "carts-3", "a pod the operator did not inject into is not the guard's to delete")
	assert.NotContains(t, message, "baskets-0", "another workload's pod has nothing to do with this finding")
	assert.NotContains(t, message, "carts-9", "a pod controlled by a different UID belongs to a workload that is gone")
	assert.LessOrEqual(t, len(ManualPodDeletionRequiredMessage("*v1.StatefulSet default/carts", "default", "carts-0",
		[]string{"carts-1", "carts-2"})), eventMessageLimit)
}

func TestPodReconciler_ManualDeletionEventSummarisesPodsBeyondTheCap(t *testing.T) {
	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}

	carts := newTestStatefulSet("carts")
	carts.UID = "carts-uid"

	trigger := ownedByController(crashingPod(t, "carts-0", images), "StatefulSet", "carts", carts.UID)
	objects := []client.Object{carts, &trigger}
	for i := 1; i < 8; i++ {
		sibling := ownedByController(readyPod(fmt.Sprintf("carts-%d", i), images), "StatefulSet", "carts", carts.UID)
		objects = append(objects, &sibling)
	}

	r, _, recorder := newPodReconcilerForTest(t, ModeDryRun, objects...)
	reconcilePod(t, r, trigger)

	events := drainEvents(recorder)
	require.Len(t, events, 2)
	message := events[1]
	assert.Contains(t, message, "kubectl delete pod carts-0 carts-1 carts-2 carts-3 carts-4 -n default")
	assert.Contains(t, message, "and 3 more auto-instrumented pod(s)")
	// The Event body the FakeRecorder reports is prefixed with "Warning <reason> ", so the
	// message itself is shorter still.
	assert.LessOrEqual(t, len(message), eventMessageLimit)
}

// TestPodReconciler_ManualDeletionEventSurvivesAListFailure: the finding has already been emitted
// when this second Event is built, so a List failure must degrade the message rather than fail the
// reconcile and emit the finding twice.
func TestPodReconciler_ManualDeletionEventSurvivesAListFailure(t *testing.T) {
	fastWorkloadGetBackOff(t)

	images := map[instrumentation.Type]string{instrumentation.TypePython: testImagePython}
	carts := newTestStatefulSet("carts")
	carts.UID = "carts-uid"
	trigger := ownedByController(crashingPod(t, "carts-0", images), "StatefulSet", "carts", carts.UID)
	sibling := ownedByController(readyPod("carts-1", images), "StatefulSet", "carts", carts.UID)

	c := fake.NewClientBuilder().
		WithObjects(carts, &trigger, &sibling).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, inner client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*corev1.PodList); ok {
					return errors.New("the cache is not ready")
				}
				return inner.List(ctx, list, opts...)
			},
		}).
		Build()

	cfg := DefaultConfig()
	cfg.Mode = ModeDryRun
	recorder := record.NewFakeRecorder(10)
	r := NewPodReconciler(c, testr.New(t), recorder, cfg)
	r.now = func() time.Time { return testNow }

	// No error: the reconcile completes.
	assert.Equal(t, ctrl.Result{}, reconcilePod(t, r, trigger))

	events := drainEvents(recorder)
	require.Len(t, events, 2)
	assert.Contains(t, events[0], "Warning "+ReasonInstrumentationSuspected, "the finding is still reported")
	assert.Contains(t, events[1], "kubectl delete pod carts-0 -n default")
	assert.NotContains(t, events[1], "carts-1")
}

func getDaemonSet(t *testing.T, c client.Client, name string) *appsv1.DaemonSet {
	t.Helper()
	daemonSet := &appsv1.DaemonSet{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: testNamespace, Name: name}, daemonSet))
	return daemonSet
}

func getStatefulSet(t *testing.T, c client.Client, name string) *appsv1.StatefulSet {
	t.Helper()
	statefulSet := &appsv1.StatefulSet{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: testNamespace, Name: name}, statefulSet))
	return statefulSet
}
