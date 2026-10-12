// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/allocation"
	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/collectornamespace"
)

// Without OTELCOL_NAMESPACE the agent pod watch must not become cluster-wide,
// and must use the same namespace the Prometheus CR watcher resolves.
func TestNewClientNamespaceWhenEnvUnset(t *testing.T) {
	for _, env := range []string{"", "  \n"} {
		t.Setenv(collectornamespace.EnvVar, env)
		c, err := NewClient(logr.Discard(), &rest.Config{Host: "https://127.0.0.1:65535"})
		require.NoError(t, err)
		assert.NotEmpty(t, c.namespace, "env=%q", env)
		assert.Equal(t, collectornamespace.Resolve(logr.Discard()), c.namespace, "env=%q", env)
	}
}

func TestNewClientNamespaceFromEnv(t *testing.T) {
	t.Setenv(collectornamespace.EnvVar, " agent-ns\n")
	c, err := NewClient(logr.Discard(), &rest.Config{Host: "https://127.0.0.1:65535"})
	require.NoError(t, err)
	assert.Equal(t, "agent-ns", c.namespace)
}

// The pod watch only reports agent pods from the client's namespace.
func TestWatchIsScopedToNamespace(t *testing.T) {
	labelMap := map[string]string{"app.kubernetes.io/instance": "amazon-cloudwatch.cloudwatch-agent"}
	pod := func(ns, name string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labelMap},
			Spec:       v1.PodSpec{NodeName: "node-" + name},
			Status:     v1.PodStatus{Phase: v1.PodRunning},
		}
	}
	c := &Client{
		log:       logr.Discard(),
		k8sClient: fake.NewSimpleClientset(pod("agent-ns", "agent-a"), pod("other-ns", "agent-b")),
		close:     make(chan struct{}),
		namespace: "agent-ns",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan map[string]*allocation.Collector, 4)
	go func() {
		_ = c.Watch(ctx, labelMap, func(collectors map[string]*allocation.Collector) { got <- collectors })
	}()
	select {
	case collectors := <-got:
		assert.Contains(t, collectors, "agent-a")
		assert.NotContains(t, collectors, "agent-b", "pod in another namespace must not be discovered")
	case <-time.After(5 * time.Second):
		t.Fatal("no collectors reported")
	}
	c.Close()
}
