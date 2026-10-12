// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	fakemonitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metadatafake "k8s.io/client-go/metadata/fake"
	clienttesting "k8s.io/client-go/testing"
)

// listGate makes LIST for one monitoring resource fail with Forbidden until it
// is opened, so its informer never syncs while the gate is closed. This models
// a CRD that exists but cannot be listed yet (not Established, or the role is
// missing list/watch on monitoring.coreos.com). The fake clientset otherwise
// syncs immediately, which is why the start paths below were never exercised.
type listGate struct {
	open atomic.Bool
}

func (g *listGate) block(w *PrometheusCRWatcher, resource string) {
	w.kubeMonitoringClient.(*fakemonitoringclient.Clientset).PrependReactor("list", resource,
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			if g.open.Load() {
				return false, nil, nil
			}
			return true, nil, apierrors.NewForbidden(
				schema.GroupResource{Group: monitoringv1.SchemeGroupVersion.Group, Resource: resource}, "", fmt.Errorf("not yet"))
		})
}

func running(w *PrometheusCRWatcher, resource string) bool {
	w.informersMtx.RLock()
	defer w.informersMtx.RUnlock()
	_, ok := w.informers[resource]
	return ok
}

// Once the blocked type becomes listable, it is picked up without a restart.
func TestWatchRetriesMonitorTypeThatFailedToStart(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, true)
	w.eventInterval = 5 * time.Millisecond
	w.startRetryInterval = 50 * time.Millisecond
	w.syncTimeout = 500 * time.Millisecond
	defer func() { _ = w.Close() }()
	var g listGate
	g.block(w, monitoringv1.ServiceMonitorName)

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 8)) }()
	require.Eventually(t, func() bool { return running(w, monitoringv1.PodMonitorName) }, 5*time.Second, 20*time.Millisecond)
	require.False(t, running(w, monitoringv1.ServiceMonitorName))

	g.open.Store(true)
	require.Eventually(t, func() bool { return running(w, monitoringv1.ServiceMonitorName) },
		5*time.Second, 20*time.Millisecond, "ServiceMonitor informer not retried after it became listable")
}

// A start that cannot sync within the deadline is reported on upstreamErrors,
// so the failure reaches main.go and is not only a log line.
func TestWatchReportsStartFailureUpstream(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, true)
	w.eventInterval = 5 * time.Millisecond
	w.syncTimeout = 500 * time.Millisecond
	w.startRetryInterval = time.Hour
	defer func() { _ = w.Close() }()
	var g listGate
	g.block(w, monitoringv1.ServiceMonitorName)

	errs := make(chan error, 8)
	go func() { _ = w.Watch(make(chan Event, 1), errs) }()

	select {
	case err := <-errs:
		assert.Contains(t, err.Error(), monitoringv1.ServiceMonitorName)
	case <-time.After(5 * time.Second):
		t.Fatal("no error reported on upstreamErrors for a monitor type that could not start")
	}
}

// startMonitorInformer reports whether it actually registered an informer.
func TestStartMonitorInformerReportsStarted(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, true)
	defer func() { _ = w.Close() }()
	notifyEvents := make(chan struct{}, 1)

	started, err := w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents)
	require.NoError(t, err)
	assert.True(t, started, "first start registers the informer")

	started, err = w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents)
	require.NoError(t, err)
	assert.False(t, started, "second start is a no-op and must not report started")

	_ = w.Close()
	started, err = w.startMonitorInformer(monitoringv1.PodMonitorName, notifyEvents)
	require.NoError(t, err)
	assert.False(t, started, "start after Close must not report started")
}

// With the CRD check Forbidden and the CRDs really absent, the fallback start is
// bounded: it gives up within the sync deadline instead of retrying a LIST on an
// unserved resource for the pod lifetime.
func TestForbiddenFallbackIsBoundedWhenCRDAbsent(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	w.eventInterval = 5 * time.Millisecond
	w.syncTimeout = 500 * time.Millisecond
	w.startRetryInterval = 50 * time.Millisecond
	w.maxStartRetryInterval = 100 * time.Millisecond
	w.maxBlindStartAttempts = 2
	defer func() { _ = w.Close() }()
	forbidCRDAccess(w)
	var lists atomic.Int64
	w.kubeMonitoringClient.(*fakemonitoringclient.Clientset).PrependReactor("list", "*",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			lists.Add(1)
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: monitoringv1.SchemeGroupVersion.Group, Resource: action.GetResource().Resource}, "")
		})

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 64)) }()

	// Every fallback start gives up after maxBlindStartAttempts, so nothing is
	// left retrying.
	require.Eventually(t, func() bool {
		w.startMtx.Lock()
		defer w.startMtx.Unlock()
		return lists.Load() > 0 && len(w.starting) == 0
	}, 10*time.Second, 50*time.Millisecond, "fallback start was not bounded")
	settled := lists.Load()
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, settled, lists.Load(), "no further LIST attempts after the fallback gave up")
	assert.False(t, running(w, monitoringv1.ServiceMonitorName))
	assert.False(t, running(w, monitoringv1.PodMonitorName))
	assert.Greater(t, lists.Load(), int64(0), "fallback did try to list")
}

// hangList makes LIST for one monitoring resource block until release is closed,
// as a CRD that exists but is not yet served would. Unlike a Forbidden LIST,
// which fails fast, this keeps the informer from ever syncing, which is what
// wedged the synchronous startup start.
func hangList(w *PrometheusCRWatcher, resource string, release <-chan struct{}) {
	w.kubeMonitoringClient.(*fakemonitoringclient.Clientset).PrependReactor("list", resource,
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			<-release
			return false, nil, nil
		})
}

// A present CRD whose LIST hangs must not stop Watch from reaching the CRD
// watch: the CRD watch's LIST is observed (via the metadata client) only once
// startup has moved on.
func TestWatchDoesNotBlockOnHangingList(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, true)
	w.eventInterval = 5 * time.Millisecond
	release := make(chan struct{})
	defer func() { close(release); _ = w.Close() }()
	hangList(w, monitoringv1.ServiceMonitorName, release)
	hangList(w, monitoringv1.PodMonitorName, release)

	var crdWatchListed atomic.Bool
	w.metadataClient.(*metadatafake.FakeMetadataClient).PrependReactor("list", "customresourcedefinitions",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			crdWatchListed.Store(true)
			return false, nil, nil
		})

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 8)) }()

	require.Eventually(t, crdWatchListed.Load, 5*time.Second, 10*time.Millisecond,
		"CRD watch never started while both monitor LISTs hang")
}
