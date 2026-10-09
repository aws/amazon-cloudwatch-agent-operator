// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManualPodDeletionRequiredMessage(t *testing.T) {
	message := ManualPodDeletionRequiredMessage("*v1.StatefulSet default/carts", "default", "carts-2",
		// What the List returns: unordered, and including the triggering pod itself.
		[]string{"carts-1", "carts-2", "carts-0"})

	assert.Contains(t, message, "kubectl delete pod carts-2 carts-0 carts-1 -n default",
		"the triggering pod comes first, the rest are sorted, and no pod is named twice")
	assert.Contains(t, message, "*v1.StatefulSet default/carts")
	assert.NotContains(t, message, "more auto-instrumented pod(s)",
		"every pod is named, so there is nothing left to summarise")
	assert.LessOrEqual(t, len(message), eventMessageLimit)
}

func TestManualPodDeletionRequiredMessageWithOnlyTheTriggeringPod(t *testing.T) {
	// What the pod reconciler falls back to when the List fails.
	message := ManualPodDeletionRequiredMessage("*v1.DaemonSet default/agents", "default", "agents-xyz", nil)

	assert.Contains(t, message, "kubectl delete pod agents-xyz -n default")
	assert.LessOrEqual(t, len(message), eventMessageLimit)
}

func TestManualPodDeletionRequiredMessageSummarisesThePodsItCannotName(t *testing.T) {
	others := []string{}
	for i := 0; i < 8; i++ {
		others = append(others, fmt.Sprintf("carts-%d", i))
	}

	message := ManualPodDeletionRequiredMessage("*v1.StatefulSet default/carts", "default", "carts-0", others)

	assert.Contains(t, message, "kubectl delete pod carts-0 carts-1 carts-2 carts-3 carts-4 -n default")
	assert.Contains(t, message, "and 3 more auto-instrumented pod(s)",
		"the count is what tells the customer the five names are not the whole job")
	assert.NotContains(t, message, "carts-7")
	assert.LessOrEqual(t, len(message), eventMessageLimit)
}

// TestManualPodDeletionRequiredMessageStaysWithinTheEventCap drives the bound with the longest
// names Kubernetes allows - 253 characters for a pod and for a workload - which is the input that
// would overrun it. See eventMessageLimit for where the number comes from.
func TestManualPodDeletionRequiredMessageStaysWithinTheEventCap(t *testing.T) {
	longPod := strings.Repeat("a", 253)
	others := []string{}
	for i := 0; i < 20; i++ {
		others = append(others, fmt.Sprintf("%s-%02d", longPod[:250], i))
	}
	workload := "*v1.StatefulSet default/" + strings.Repeat("w", 253)

	message := ManualPodDeletionRequiredMessage(workload, strings.Repeat("n", 63), longPod, others)

	require.LessOrEqual(t, len(message), eventMessageLimit,
		"an Event message the guard cannot fit is one the customer may never see")
	assert.Contains(t, message, longPod, "the pod the guard found broken is the one name that must survive")
	assert.Contains(t, message, "and 20 more auto-instrumented pod(s)")
	assert.NotContains(t, message, eventMessageTruncationMarker,
		"dropping names must be enough on its own: truncation is only the backstop")
}

// TestBoundEventMessage covers that backstop directly, since no realistic workload reaches it.
func TestBoundEventMessage(t *testing.T) {
	assert.Equal(t, "short", boundEventMessage("short"))

	exact := strings.Repeat("x", eventMessageLimit)
	assert.Equal(t, exact, boundEventMessage(exact), "the limit itself is allowed")

	bounded := boundEventMessage(strings.Repeat("x", eventMessageLimit+1))
	assert.Len(t, bounded, eventMessageLimit)
	assert.True(t, strings.HasSuffix(bounded, eventMessageTruncationMarker))
}

// TestPausedDeploymentMessage: the action in a paused Deployment's Event is a resume, and the
// message has to say that deleting the pods is not the fix - the generic message would have the
// customer try exactly that, and the paused Deployment's existing ReplicaSet would recreate them
// from the template that still injects.
func TestPausedDeploymentMessage(t *testing.T) {
	message := PausedDeploymentMessage("*v1.Deployment default/orders", "default", "orders")

	assert.Contains(t, message, "*v1.Deployment default/orders is paused")
	assert.Contains(t, message, "kubectl rollout resume deployment/orders -n default")
	assert.Contains(t, message, "Deleting those pods does not help")
	assert.NotContains(t, message, "kubectl delete pod")
	assert.Contains(t, message, "never deletes a pod")
	assert.LessOrEqual(t, len(message), eventMessageLimit)
}

func TestPausedDeploymentMessageWithoutANamespace(t *testing.T) {
	message := PausedDeploymentMessage("*v1.Deployment orders", "", "orders")
	assert.Contains(t, message, "kubectl rollout resume deployment/orders.")
	assert.NotContains(t, message, "-n ")
}
