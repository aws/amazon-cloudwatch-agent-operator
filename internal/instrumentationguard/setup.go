// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupWithManager registers the one controller the instrumentation guard needs: a reconciler over
// auto-instrumented pods, which decides that a pod is broken and whether auto-instrumentation
// caused it, and on both counts emits a Warning Event on the owning workload. There is nothing
// after that - no patch, no deletion, no verify step and no state - because the guard is
// detect-only. See the package comment.
func SetupWithManager(mgr ctrl.Manager, cfg Config, recorder record.EventRecorder) error {
	logger := mgr.GetLogger().WithName("instrumentation-guard")

	podReconciler := NewPodReconciler(mgr.GetClient(), logger.WithValues("controller", "pod"), recorder, cfg)
	return podReconciler.SetupWithManager(mgr)
}
