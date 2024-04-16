// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package controllers contains the main controller, where the reconciliation starts.
package controllers

import (
	"context"
	"strings"

	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/collector/adapters"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests/dcgmexporter"
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

const (
	acceleratedComputeMetrics = "accelerated_compute_metrics"
	amazonCloudWatchNamespace = "amazon-cloudwatch"
	amazonCloudWatchAgentName = "cloudwatch-agent"
)

// DcgmExporterReconciler reconciles a DcgmExporter object.
type DcgmExporterReconciler struct {
	client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
	log      logr.Logger
	config   config.Config
}

func (r *DcgmExporterReconciler) getParams(instance v1alpha1.DcgmExporter) manifests.Params {
	return manifests.Params{
		Config:   r.config,
		Client:   r.Client,
		DcgmExp:  instance,
		Log:      r.log,
		Scheme:   r.scheme,
		Recorder: r.recorder,
	}
}

// NewReconciler creates a new reconciler for DcgmExporter objects.
func NewDcgmExporterReconciler(p Params) *DcgmExporterReconciler {
	r := &DcgmExporterReconciler{
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
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=dcgmexporters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=dcgmexporters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cloudwatch.aws.amazon.com,resources=dcgmexporters/finalizers,verbs=get;update;patch

// Reconcile the current state of an OpenTelemetry collector resource with the desired state.
func (r *DcgmExporterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("DcgmExporter", req.NamespacedName)

	var instance v1alpha1.DcgmExporter
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch DcgmExporter")
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

	if !r.enabledByAgentConfig(ctx, log) {
		log.Info("enhanced_container_insights or accelerated_compute_metrics is disabled")
		return ctrl.Result{}, nil
	}

	params := r.getParams(instance)

	desiredObjects, buildErr := BuildDcgmExporter(params)
	if buildErr != nil {
		return ctrl.Result{}, buildErr
	}
	err := reconcileDesiredObjects(ctx, r.Client, log, &params.DcgmExp, params.Scheme, desiredObjects...)
	return collectorStatus.HandleReconcileStatus(ctx, log, params, err)
}

// BuildDcgmExporter returns the generation and collected errors of all manifests for a given instance.
func BuildDcgmExporter(params manifests.Params) ([]client.Object, error) {
	builders := []manifests.Builder{
		dcgmexporter.Build,
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
func (r *DcgmExporterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DcgmExporter{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{})

	return builder.Complete(r)
}

func (r *DcgmExporterReconciler) enabledByAgentConfig(ctx context.Context, log logr.Logger) bool {
	agentResource := getAmazonCloudWatchAgentResource(ctx, r.Client)
	// missing feature flag means it's on by default
	featureConfigExists := strings.Contains(agentResource.Spec.Config, acceleratedComputeMetrics)
	conf, err := adapters.ConfigStructFromJSONString(agentResource.Spec.Config)
	if err == nil {
		if conf.Logs.LogMetricsCollected.Kubernetes.EnhancedContainerInsights {
			return !featureConfigExists || conf.Logs.LogMetricsCollected.Kubernetes.AcceleratedComputeMetrics
		} else {
			// disable when enhanced container insights is disabled
			return false
		}
	} else {
		log.Error(err, "Failed to unmarshall agent configuration")
	}

	return true
}

func getAmazonCloudWatchAgentResource(ctx context.Context, c client.Client) v1alpha1.AmazonCloudWatchAgent {
	cr := &v1alpha1.AmazonCloudWatchAgent{}

	_ = c.Get(ctx, client.ObjectKey{
		Namespace: amazonCloudWatchNamespace,
		Name:      amazonCloudWatchAgentName,
	}, cr)

	return *cr
}
