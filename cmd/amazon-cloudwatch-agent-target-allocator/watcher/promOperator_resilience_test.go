// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"fmt"
	"testing"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// TestStartMonitorInformerIdempotent verifies that starting an already-running
// informer is a no-op: it stays running and no second reload is signalled.
func TestStartMonitorInformerIdempotent(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, false)
	defer func() { _ = w.Close() }()

	notifyEvents := make(chan struct{}, 1)
	require.NoError(t, w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents))
	// drain the reload notification emitted by the first (real) start
	select {
	case <-notifyEvents:
	default:
	}

	// Second call must be a no-op.
	require.NoError(t, w.startMonitorInformer(monitoringv1.ServiceMonitorName, notifyEvents))
	require.True(t, runningInformers(w)[monitoringv1.ServiceMonitorName])
	select {
	case <-notifyEvents:
		t.Fatal("no-op start must not signal a reload")
	default:
	}
}

// TestStartMonitorInformerUnknownResource verifies an unrecognised resource name
// is rejected with an error and starts nothing.
func TestStartMonitorInformerUnknownResource(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	defer func() { _ = w.Close() }()

	err := w.startMonitorInformer("widgets", make(chan struct{}, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown monitoring resource")
	assert.Empty(t, runningInformers(w))
}

// TestStopMonitorInformerNotRunning verifies stopping an informer that is not
// running is a no-op and signals no reload.
func TestStopMonitorInformerNotRunning(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	defer func() { _ = w.Close() }()

	notifyEvents := make(chan struct{}, 1)
	w.stopMonitorInformer(monitoringv1.PodMonitorName, notifyEvents)

	assert.Empty(t, runningInformers(w))
	select {
	case <-notifyEvents:
		t.Fatal("stopping a non-running informer must not signal a reload")
	default:
	}
}

// TestCrdExistsSurfacesNonNotFoundError verifies crdExists propagates
// non-NotFound API errors instead of treating them as "absent".
func TestCrdExistsSurfacesNonNotFoundError(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	defer func() { _ = w.Close() }()

	fakeCRD := w.crdClient.(*apiextensionsfake.Clientset)
	fakeCRD.PrependReactor("get", "customresourcedefinitions",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(fmt.Errorf("boom"))
		})

	exists, err := w.crdExists(context.Background(), crdFor(monitoringv1.ServiceMonitorName).Name)
	require.Error(t, err)
	assert.False(t, exists)
}

// TestWatchToleratesCRDCheckError verifies a non-NotFound error while probing
// for a CRD at startup does not take the watcher down: Watch keeps running.
func TestWatchToleratesCRDCheckError(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, true, true)
	w.eventInterval = 5 * time.Millisecond
	defer func() { _ = w.Close() }()

	fakeCRD := w.crdClient.(*apiextensionsfake.Clientset)
	fakeCRD.PrependReactor("get", "customresourcedefinitions",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(fmt.Errorf("boom"))
		})

	watchDone := make(chan error, 1)
	go func() { watchDone <- w.Watch(make(chan Event, 1), make(chan error, 1)) }()

	select {
	case err := <-watchDone:
		t.Fatalf("Watch exited unexpectedly with: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestWatchIgnoresUntrackedCRD verifies that creating a CRD the watcher does not
// track never starts a monitoring informer.
func TestWatchIgnoresUntrackedCRD(t *testing.T) {
	w := getTestPrometheusCRWatcherWithCRDs(t, nil, nil, false, false)
	w.eventInterval = 5 * time.Millisecond
	defer func() { _ = w.Close() }()

	go func() { _ = w.Watch(make(chan Event, 1), make(chan error, 1)) }()

	_, err := w.crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(
		context.Background(),
		&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com"}},
		metav1.CreateOptions{})
	require.NoError(t, err)

	require.Never(t, func() bool {
		return len(runningInformers(w)) > 0
	}, 200*time.Millisecond, 20*time.Millisecond)
}

// TestNotifyHandlerSignalsOnAllEvents verifies the coalescing handler signals a
// reload on add, update and delete events.
func TestNotifyHandlerSignalsOnAllEvents(t *testing.T) {
	for _, tc := range []string{"add", "update", "delete"} {
		t.Run(tc, func(t *testing.T) {
			ch := make(chan struct{}, 1)
			h := notifyHandler(ch)
			switch tc {
			case "add":
				h.AddFunc(nil)
			case "update":
				h.UpdateFunc(nil, nil)
			case "delete":
				h.DeleteFunc(nil)
			}
			select {
			case <-ch:
			default:
				t.Fatalf("%s handler did not signal a reload", tc)
			}
		})
	}
}
