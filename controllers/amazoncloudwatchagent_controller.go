// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package controllers contains the main controller, where the reconciliation starts.
package controllers

import (
	"context"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/config"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	collectorStatus "github.com/aws/amazon-cloudwatch-agent-operator/internal/status/collector"
)

// AmazonCloudWatchAgentReconciler reconciles a AmazonCloudWatchAgent object.
type AmazonCloudWatchAgentReconciler struct {
	client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
	config   config.Config
}

// Params is the set of options to build a new AmazonCloudWatchAgentReconciler.
type Params struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Log      logr.Logger
	Config   config.Config
}

func (r *AmazonCloudWatchAgentReconciler) getParams(instance v1alpha1.AmazonCloudWatchAgent) (manifests.Params, error) {
	p := manifests.Params{
		Config:   r.config,
		Client:   r.Client,
		OtelCol:  instance,
		Log:      r.log,
		Scheme:   r.scheme,
		Recorder: r.recorder,
	}

	// generate the target allocator CR from the collector CR
	targetAllocator, err := collector.TargetAllocator(p)
	if err != nil {
		return p, err
	}
	p.TargetAllocator = targetAllocator
	return p, nil
}

// NewReconciler creates a new reconciler for AmazonCloudWatchAgent objects.
func NewReconciler(p Params) *AmazonCloudWatchAgentReconciler {
	r := &AmazonCloudWatchAgentReconciler{
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
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=amazoncloudwatchagents/finalizers,verbs=get;update;patch

// Reconcile the current state of an AmazonCloudWatchAgent collector resource with the desired state.
func (r *AmazonCloudWatchAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("amazoncloudwatchagent", req.NamespacedName)

	var instance v1alpha1.AmazonCloudWatchAgent
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch AmazonCloudWatchAgent")
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

	if instance.Spec.ManagementState == v1alpha1.ManagementStateUnmanaged {
		log.Info("Skipping reconciliation for unmanaged AmazonCloudWatchAgent resource", "name", req.String())
		// Stop requeueing for unmanaged AmazonCloudWatchAgent custom resources
		return ctrl.Result{}, nil
	}

	params, err := r.getParams(instance)
	if err != nil {
		log.Error(err, "Failed to create manifest.Params")
		return ctrl.Result{}, err
	}

	desiredObjects, buildErr := BuildCollector(params)
	if buildErr != nil {
		return ctrl.Result{}, buildErr
	}
	err = reconcileDesiredObjects(ctx, r.Client, log, &params.OtelCol, params.Scheme, desiredObjects...)
	return collectorStatus.HandleReconcileStatus(ctx, log, params, err)
}

// SetupWithManager tells the manager what our controller is interested in.
func (r *AmazonCloudWatchAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AmazonCloudWatchAgent{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.StatefulSet{})

	return builder.Complete(r)
}
