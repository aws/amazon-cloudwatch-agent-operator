// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/allocation"
	"github.com/aws/amazon-cloudwatch-agent-operator/cmd/amazon-cloudwatch-agent-target-allocator/collectornamespace"
)

const (
	watcherTimeout = 15 * time.Minute
)

var (
	collectorsDiscovered = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "amazon_cloudwatch_agent_allocator_collectors_discovered",
		Help: "Number of collectors discovered.",
	})
)

type Client struct {
	log       logr.Logger
	k8sClient kubernetes.Interface
	close     chan struct{}
	// namespace scopes the agent pod watch. It must not be empty: client-go
	// treats "" as all namespaces.
	namespace string
}

func NewClient(logger logr.Logger, kubeConfig *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return &Client{}, err
	}

	log := logger.WithValues("component", "amazon-cloudwatch-agent-target-allocator")
	return &Client{
		log:       log,
		k8sClient: clientset,
		close:     make(chan struct{}),
		namespace: collectornamespace.Resolve(log),
	}, nil
}

func (k *Client) Watch(ctx context.Context, labelMap map[string]string, fn func(collectors map[string]*allocation.Collector)) error {
	collectorMap := map[string]*allocation.Collector{}

	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap).String(),
	}
	resourceVersion, err := k.relist(ctx, opts, collectorMap, fn)
	if err != nil {
		k.log.Error(err, "Pod failure")
		os.Exit(1)
	}

	for {
		if !k.restartWatch(ctx, opts, resourceVersion, collectorMap, fn) {
			return nil
		}
		// Rebuild the collector set from a fresh List before every new watch.
		// A pod deleted while no watch was open never gets a Deleted event, so
		// without this it would stay registered. Under per-node it would keep
		// its node's targets after its replacement starts.
		if resourceVersion, err = k.relist(ctx, opts, collectorMap, fn); err != nil {
			k.log.Error(err, "unable to list collector pods")
			return nil
		}
	}
}

// relist replaces the contents of collectorMap with the active collector pods
// from a fresh List, calls fn, and returns the List's resourceVersion so the
// next watch starts exactly where the List ended.
func (k *Client) relist(ctx context.Context, opts metav1.ListOptions, collectorMap map[string]*allocation.Collector, fn func(collectors map[string]*allocation.Collector)) (string, error) {
	pods, err := k.k8sClient.CoreV1().Pods(k.namespace).List(ctx, opts)
	if err != nil {
		return "", err
	}
	for name := range collectorMap {
		delete(collectorMap, name)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if isActiveCollector(pod) {
			collectorMap[pod.Name] = allocation.NewCollector(pod.Name, pod.Spec.NodeName)
		}
	}
	collectorsDiscovered.Set(float64(len(collectorMap)))
	fn(collectorMap)
	return pods.ResourceVersion, nil
}

// isActiveCollector reports whether an agent pod should receive targets: it is
// scheduled to a node, not being deleted, Running, and Ready. A pod that is
// crash looping, failing to pull its image, or Failed (for example evicted)
// must not be registered: under per-node it would hold its node's targets, and
// they would go unscraped instead of being placed by the fallback strategy.
// The initial List and watch events both use this check.
func isActiveCollector(pod *v1.Pod) bool {
	if pod.GetObjectMeta().GetDeletionTimestamp() != nil || pod.Spec.NodeName == "" {
		return false
	}
	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == v1.PodReady {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}

func (k *Client) restartWatch(ctx context.Context, opts metav1.ListOptions, resourceVersion string, collectorMap map[string]*allocation.Collector, fn func(collectors map[string]*allocation.Collector)) bool {
	// add timeout to the context before calling Watch
	ctx, cancel := context.WithTimeout(ctx, watcherTimeout)
	defer cancel()
	opts.ResourceVersion = resourceVersion
	watcher, err := k.k8sClient.CoreV1().Pods(k.namespace).Watch(ctx, opts)
	if err != nil {
		k.log.Error(err, "unable to create collector pod watcher")
		return false
	}
	k.log.Info("Successfully started a collector pod watcher")
	if msg := runWatch(ctx, k, watcher.ResultChan(), collectorMap, fn); msg != "" {
		k.log.Info("Collector pod watch event stopped " + msg)
		return false
	}

	return true
}

func runWatch(ctx context.Context, k *Client, c <-chan watch.Event, collectorMap map[string]*allocation.Collector, fn func(collectors map[string]*allocation.Collector)) string {
	for {
		collectorsDiscovered.Set(float64(len(collectorMap)))
		select {
		case <-k.close:
			return "kubernetes client closed"
		case <-ctx.Done():
			return "" // this means that the watcher most likely timed out
		case event, ok := <-c:
			if !ok {
				k.log.Info("No event found. Restarting watch routine")
				return ""
			}

			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				k.log.Info("No pod found in event Object. Restarting watch routine")
				return ""
			}

			switch event.Type { //nolint:exhaustive
			case watch.Added, watch.Modified:
				// Register or refresh the collector while it is active, and drop
				// it as soon as it stops being active: marked for deletion, not
				// Running, or not Ready. Dropping it lets its node's targets be
				// placed on the node's replacement agent or by the fallback
				// instead of going unscraped. Handling Modified also picks up
				// the node of a pod that was Added before it was scheduled, and a
				// pod that becomes Ready again.
				if isActiveCollector(pod) {
					collectorMap[pod.Name] = allocation.NewCollector(pod.Name, pod.Spec.NodeName)
				} else {
					delete(collectorMap, pod.Name)
				}
			case watch.Deleted:
				delete(collectorMap, pod.Name)
			}
			fn(collectorMap)
		}
	}
}

func (k *Client) Close() {
	close(k.close)
}
