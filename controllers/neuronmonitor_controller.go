// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package controllers contains the main controller, where the reconciliation starts.
package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1beta1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/neuronmonitor"
	neuronmonitorStatus "github.com/aws/amazon-cloudwatch-agent-operator/internal/status/neuronmonitor"
)

// NeuronMonitorReconciler reconciles a NeuronMonitor object.
type NeuronMonitorReconciler struct {
	client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
	config   config.Config
}

func (r *NeuronMonitorReconciler) getParams(instance v1beta1.NeuronMonitor) manifests.Params {
	return manifests.Params{
		Config:    r.config,
		Client:    r.Client,
		NeuronExp: instance,
		Log:       r.log,
		Scheme:    r.scheme,
		Recorder:  r.recorder,
	}
}

// NewReconciler creates a new reconciler for NeuronMonitor objects.
func NewNeuronMonitorReconciler(p Params) *NeuronMonitorReconciler {
	r := &NeuronMonitorReconciler{
		Client:   p.Client,
		log:      p.Log,
		scheme:   p.Scheme,
		config:   p.Config,
		recorder: p.Recorder,
	}
	return r
}

// +kubebuilder:rbac:groups="",resources=pods;configmaps;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=neuronmonitors,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=neuronmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=neuronmonitors/finalizers,verbs=get;update;patch

// Reconcile the current state of an OpenTelemetry collector resource with the desired state.
func (r *NeuronMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("NeuronMonitor", req.NamespacedName)

	var instance v1beta1.NeuronMonitor
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch NeuronMonitor")
		}

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// We have a deletion, short circuit and let the deletion happen
	if deletionTimestamp := instance.GetDeletionTimestamp(); deletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	params := r.getParams(instance)
	desiredObjects, buildErr := BuildNeuronMonitor(params)
	if buildErr != nil {
		return ctrl.Result{}, buildErr
	}

	if !enabledAcceleratedComputeByAgentConfig(ctx, r.Client, log) {
		log.Info("enhanced_container_insights or accelerated_compute_metrics is disabled")
		for _, obj := range desiredObjects {
			if err := r.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete resources", "resource", obj)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	err := reconcileDesiredObjects(ctx, r.Client, log, &params.NeuronExp, params.Scheme, desiredObjects...)
	return neuronmonitorStatus.HandleReconcileStatus(ctx, log, params, err)
}

// BuildNeuronMonitor returns the generation and collected errors of all manifests for a given instance.
func BuildNeuronMonitor(params manifests.Params) ([]client.Object, error) {
	builders := []manifests.Builder{
		neuronmonitor.Build,
	}
	var resources []client.Object
	for _, builder := range builders {
		objs, err := builder(params)
		if err != nil {
			return nil, err
		}
		resources = append(resources, objs...)
	}
	return resources, nil
}

// SetupWithManager tells the manager what our controller is interested in.
func (r *NeuronMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.NeuronMonitor{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{})

	return builder.Complete(r)
}
