// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/allocation"
)

var testLabels = map[string]string{
	"app.kubernetes.io/instance":   "default.test",
	"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
}

// readyPod is an agent pod that is scheduled, Running and Ready.
func readyPod(name, node string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-ns", Labels: testLabels},
		Spec:       v1.PodSpec{NodeName: node},
		Status: v1.PodStatus{
			Phase:      v1.PodRunning,
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
		},
	}
}

// latest collects every collector map passed to fn, so tests can wait for a state.
type latest struct {
	mu   sync.Mutex
	last map[string]*allocation.Collector
}

func (l *latest) fn(m map[string]*allocation.Collector) {
	cp := make(map[string]*allocation.Collector, len(m))
	for k, v := range m {
		cp[k] = v
	}
	l.mu.Lock()
	l.last = cp
	l.mu.Unlock()
}

func (l *latest) names() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]string{}
	for k, v := range l.last {
		out[k] = v.NodeName
	}
	return out
}

// A pod deleted while no watch is open must not stay registered after the
// watch restarts. Otherwise, under per-node, it can keep its node's targets.
func TestWatchDropsPodDeletedBetweenWatchSessions(t *testing.T) {
	cs := fake.NewSimpleClientset(readyPod("cwagent-abc", "node-1"))
	watchers := make(chan *watch.FakeWatcher, 4)
	cs.PrependWatchReactor("pods", func(action clienttesting.Action) (bool, watch.Interface, error) {
		w := watch.NewFake()
		watchers <- w
		return true, w, nil
	})
	c := &Client{log: logr.Discard(), k8sClient: cs, close: make(chan struct{}), namespace: "test-ns"}
	var got latest
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Watch(ctx, testLabels, got.fn) }()

	first := <-watchers
	require.Eventually(t, func() bool { return got.names()["cwagent-abc"] == "node-1" }, 5*time.Second, 10*time.Millisecond)

	// The old agent is replaced while no watch is open: its Deleted event is never delivered.
	require.NoError(t, cs.Tracker().Delete(v1.SchemeGroupVersion.WithResource("pods"), "test-ns", "cwagent-abc"))
	require.NoError(t, cs.Tracker().Add(readyPod("cwagent-xyz", "node-1")))
	first.Stop()

	<-watchers
	require.Eventually(t, func() bool {
		n := got.names()
		_, stale := n["cwagent-abc"]
		return !stale && n["cwagent-xyz"] == "node-1"
	}, 5*time.Second, 10*time.Millisecond, "collector set not rebuilt after watch restart: %v", got.names())
	c.Close()
}

func TestCollectorEligibility(t *testing.T) {
	notReady := readyPod("p", "n")
	notReady.Status.Conditions[0].Status = v1.ConditionFalse
	pending := readyPod("p", "n")
	pending.Status.Phase = v1.PodPending
	failed := readyPod("p", "n")
	failed.Status.Phase = v1.PodFailed
	noCondition := readyPod("p", "n")
	noCondition.Status.Conditions = nil
	unscheduled := readyPod("p", "")
	terminating := readyPod("p", "n")
	now := metav1.Now()
	terminating.DeletionTimestamp = &now

	for name, tc := range map[string]struct {
		pod  *v1.Pod
		want bool
	}{
		"running and ready":  {readyPod("p", "n"), true},
		"not ready":          {notReady, false},
		"pending":            {pending, false},
		"failed (evicted)":   {failed, false},
		"no ready condition": {noCondition, false},
		"unscheduled":        {unscheduled, false},
		"terminating":        {terminating, false},
	} {
		assert.Equal(t, tc.want, isActiveCollector(tc.pod), name)
	}
}

// A collector that stops being Ready (for example it starts crash looping) is
// removed, so its node's targets go to the fallback instead of being unscraped,
// and it is registered again once it is Ready.
func TestRunWatchTracksReadiness(t *testing.T) {
	c := &Client{log: logr.Discard(), k8sClient: fake.NewSimpleClientset(), close: make(chan struct{})}
	events := make(chan watch.Event, 8)
	collectorMap := map[string]*allocation.Collector{}
	var got latest
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatch(ctx, c, events, collectorMap, got.fn)

	p := readyPod("cwagent-abc", "node-1")
	events <- watch.Event{Type: watch.Added, Object: p.DeepCopyObject()}
	require.Eventually(t, func() bool { return got.names()["cwagent-abc"] == "node-1" }, 5*time.Second, 10*time.Millisecond)

	crash := p.DeepCopy()
	crash.Status.Conditions[0].Status = v1.ConditionFalse
	events <- watch.Event{Type: watch.Modified, Object: runtime.Object(crash)}
	require.Eventually(t, func() bool { _, ok := got.names()["cwagent-abc"]; return !ok }, 5*time.Second, 10*time.Millisecond,
		"not-Ready collector still registered")

	events <- watch.Event{Type: watch.Modified, Object: p.DeepCopyObject()}
	require.Eventually(t, func() bool { return got.names()["cwagent-abc"] == "node-1" }, 5*time.Second, 10*time.Millisecond,
		"collector not re-registered when Ready again")

	evicted := p.DeepCopy()
	evicted.Status.Phase = v1.PodFailed
	events <- watch.Event{Type: watch.Modified, Object: runtime.Object(evicted)}
	require.Eventually(t, func() bool { _, ok := got.names()["cwagent-abc"]; return !ok }, 5*time.Second, 10*time.Millisecond,
		"Failed (evicted) collector still registered")
	c.Close()
}
