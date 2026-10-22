// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package controllers contains the main controller, where the reconciliation starts.
package controllers

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// GPUPresentLabel is stamped on a node that exposes an NVIDIA GPU capacity key.
	GPUPresentLabel = "cloudwatch.aws.amazon.com/gpu.present"
	// NeuronPresentLabel is stamped on a node that exposes an AWS Neuron capacity key.
	NeuronPresentLabel = "cloudwatch.aws.amazon.com/neuron.present"

	nvidiaGPUResource    corev1.ResourceName = "nvidia.com/gpu"
	neuronResource       corev1.ResourceName = "aws.amazon.com/neuron"
	neuronCoreResource   corev1.ResourceName = "aws.amazon.com/neuroncore"
	neuronDeviceResource corev1.ResourceName = "aws.amazon.com/neurondevice"
)

// relevantCapacityResources are the node capacity keys the labeler projects into labels.
var relevantCapacityResources = []corev1.ResourceName{
	nvidiaGPUResource,
	neuronResource,
	neuronCoreResource,
	neuronDeviceResource,
}

// NodeLabelerReconciler projects accelerator capacity present on a Node into
// fixed-key labels that a DaemonSet affinity can select on. The scheduler cannot
// match on status.capacity directly, so we mirror capacity presence into labels.
type NodeLabelerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;patch

// Reconcile stamps or removes the GPU/Neuron presence labels on a single Node so
// that they match the accelerator capacity the kubelet currently advertises.
func (r *NodeLabelerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("node", req.Name)

	var node corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch Node")
		}
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	gpuDesired := hasCapacityKey(&node, nvidiaGPUResource)
	neuronDesired := hasCapacityKey(&node, neuronResource) ||
		hasCapacityKey(&node, neuronCoreResource) ||
		hasCapacityKey(&node, neuronDeviceResource)

	// Build a JSON merge patch touching only metadata.labels. We intentionally do
	// NOT use MergeFrom on the fetched node object: that would send the full node
	// (including our stale copy of status) and could race the kubelet's frequent
	// status updates. A raw merge patch scoped to labels only touches what we own.
	labels := map[string]interface{}{}
	var changes []string
	if value, changed, action := labelChange(&node, GPUPresentLabel, gpuDesired); changed {
		labels[GPUPresentLabel] = value
		changes = append(changes, action+" "+GPUPresentLabel)
	}
	if value, changed, action := labelChange(&node, NeuronPresentLabel, neuronDesired); changed {
		labels[NeuronPresentLabel] = value
		changes = append(changes, action+" "+NeuronPresentLabel)
	}

	if len(labels) == 0 {
		log.V(1).Info("node capacity labels already up to date")
		return ctrl.Result{}, nil
	}

	patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{"labels": labels}})
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.Patch(ctx, &node, client.RawPatch(types.MergePatchType, patch)); err != nil {
		log.Error(err, "unable to patch node labels")
		return ctrl.Result{}, err
	}

	log.Info("updated node capacity labels", "changes", changes, "gpu.present", gpuDesired, "neuron.present", neuronDesired)
	return ctrl.Result{}, nil
}

// hasCapacityKey reports whether the node advertises the given capacity key.
//
// We test for the *presence* of the key, not a non-zero quantity: when a device
// plugin becomes unhealthy the kubelet keeps the capacity key but resets its
// quantity to 0. We still want dcgm-exporter scheduled onto such a GPU node so it
// can surface the degraded device, so presence of the key is the signal we use,
// regardless of the advertised quantity.
func hasCapacityKey(node *corev1.Node, name corev1.ResourceName) bool {
	_, ok := node.Status.Capacity[name]
	return ok
}

// labelChange decides how a single managed label must change to match desired.
// It returns the merge-patch value ("true" to set, nil to delete), whether a
// change is needed at all, and a short action string for logging.
func labelChange(node *corev1.Node, key string, desired bool) (value interface{}, changed bool, action string) {
	current, present := node.Labels[key]
	switch {
	case desired && current != "true":
		return "true", true, "add"
	case !desired && present:
		return nil, true, "remove"
	default:
		return nil, false, ""
	}
}

// nodeCapacityOrLabelPredicate limits reconciliation to events that can actually
// change a managed label: every Create (so existing nodes get labeled on startup)
// and only those Updates where a relevant capacity key or one of the two managed
// labels differs. This filters out the frequent, irrelevant node status churn
// (heartbeats, conditions, addresses). Delete and Generic are ignored.
func nodeCapacityOrLabelPredicate() predicate.Funcs {
	snapshot := func(node *corev1.Node) map[string]string {
		snap := map[string]string{}
		for _, name := range relevantCapacityResources {
			if _, ok := node.Status.Capacity[name]; ok {
				snap["cap/"+string(name)] = "present"
			}
		}
		snap["lbl/"+GPUPresentLabel] = node.Labels[GPUPresentLabel]
		snap["lbl/"+NeuronPresentLabel] = node.Labels[NeuronPresentLabel]
		return snap
	}
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, okOld := e.ObjectOld.(*corev1.Node)
			newNode, okNew := e.ObjectNew.(*corev1.Node)
			if !okOld || !okNew {
				return false
			}
			return !reflect.DeepEqual(snapshot(oldNode), snapshot(newNode))
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// SetupWithManager tells the manager what our controller is interested in.
func (r *NodeLabelerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}, builder.WithPredicates(nodeCapacityOrLabelPredicate())).
		Named("nodelabeler").
		Complete(r)
}
