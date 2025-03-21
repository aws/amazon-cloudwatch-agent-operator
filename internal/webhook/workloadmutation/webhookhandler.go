// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workloadmutation contains the webhook that injects annotations into daemon-sets, deployments and stateful-sets.
package workloadmutation

import (
	"context"
	"encoding/json"
	"errors"
	v1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

// +kubebuilder:webhook:path=/mutate-v1-workload,mutating=true,failurePolicy=ignore,groups="apps",resources=daemonsets;deployments;statefulsets,verbs=create;update,versions=v1,name=mworkload.kb.io,sideEffects=none,admissionReviewVersions=v1
// +kubebuilder:rbac:groups="apps",resources=daemonsets;deployments;statefulsets,verbs=get;list;watch

var _ WebhookHandler = (*workloadMutationWebhook)(nil)
var logger = ctrl.Log.WithName("workload_webhook")

// WebhookHandler is a webhook handler that analyzes new daemon-sets and injects appropriate annotations into it.
type WebhookHandler interface {
	admission.Handler
}

// the implementation.
type workloadMutationWebhook struct {
	decoder *admission.Decoder
	monitor auto.MonitorInterface
}

// NewWebhookHandler creates a new WorkloadWebhookHandler.
func NewWebhookHandler(decoder *admission.Decoder, monitor auto.MonitorInterface) WebhookHandler {
	return &workloadMutationWebhook{
		decoder: decoder,
		monitor: monitor,
	}
}

func (p *workloadMutationWebhook) Handle(_ context.Context, req admission.Request) admission.Response {
	var oldObj, obj client.Object
	switch objectKind := req.Kind.Kind; objectKind {
	case "DaemonSet":
		oldObj = &appsv1.DaemonSet{}
		obj = &appsv1.DaemonSet{}
	case "Deployment":
		oldObj = &appsv1.Deployment{}
		obj = &appsv1.Deployment{}
	case "StatefulSet":
		oldObj = &appsv1.StatefulSet{}
		obj = &appsv1.StatefulSet{}
	default:
		return admission.Errored(http.StatusBadRequest, errors.New("failed to unmarshal request object"))
	}

	if err := p.decoder.Decode(req, obj); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// populate old object
	if req.Operation == v1.Update {
		if err := p.decoder.DecodeRaw(req.OldObject, oldObj); err != nil {
			logger.Error(err, "failed to unmarshal old object")
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	p.monitor.MutateObject(oldObj, obj)

	marshaledObject, err := json.Marshal(obj)
	if err != nil {
		res := admission.Errored(http.StatusInternalServerError, err)
		res.Allowed = true
		return res
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObject)
}
