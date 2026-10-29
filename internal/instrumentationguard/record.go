// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentationguard

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

// RecordAnnotationKey is the annotation on a workload's own metadata that silences the guard for
// that workload.
//
// The guard NEVER writes it: it is detect-only, so the only hand that puts this annotation on a
// workload is the customer's. Its one effect is the early exit in PodReconciler.Reconcile - a
// workload carrying it gets no further Events, which is what a customer who has already looked at
// the finding needs.
//
// It silences the guard only while injection is still turned off on the pod template, which is the
// state it was written about. If the inject annotations come back to "true" the guard reports
// again, however long the record has been sitting there: see injectionDisabledOnTemplate.
const RecordAnnotationKey = "cloudwatch.aws.amazon.com/instrumentation-guard"

// Record is the JSON shape of the RecordAnnotationKey annotation, for a customer or a tool that
// wants to say why the workload was silenced. Nothing in the operator writes it; the guard only
// asks whether the annotation is there (HasRecord), and every field here is optional as far as
// that question is concerned.
type Record struct {
	BackedOutAt metav1.Time `json:"backedOutAt"`
	// Reason is why auto-instrumentation was turned off on this workload, or why the guard was
	// silenced for it.
	Reason string `json:"reason"`
	// FailedImages is the auto-instrumentation image that was injected, per language.
	FailedImages map[instrumentation.Type]string `json:"failedImages"`
	// Previous holds each inject annotation's value on the pod template beforehand. A nil value,
	// i.e. JSON null, means the key was ABSENT, so an exact revert deletes it.
	Previous map[string]*string `json:"previous"`
}

// HasRecord reports whether the workload carries a guard record.
func HasRecord(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[RecordAnnotationKey]
	return ok
}

// injectionDisabledOnTemplate reports whether the workload's pod template still turns off every
// language that was injected into the pod the guard is looking at, i.e. whether the silencing the
// record annotation goes with is actually still in place.
//
// The record annotation alone is not that evidence. It is one annotation on the workload, and the
// inject annotations are others on the pod template, so the two drift apart in the ordinary course
// of running a cluster: `kubectl apply`, Argo CD and Flux all restore the annotations they manage
// and none of them removes an annotation it does not, so a workload whose manifest says
// inject-<lang>: "true" gets injection back while the record stays behind. Treating the record as
// permanent silence would have the guard say nothing about pods it is watching break.
//
// A language whose annotation is ABSENT counts as not disabled. Injection can be turned on from
// the namespace or by auto-monitor, so an absent annotation is not a customer saying no - and the
// pod in hand is proof that something is still injecting. The guard emits an Event either way, and
// an Event is all it does.
func injectionDisabledOnTemplate(obj client.Object, images map[instrumentation.Type]string) bool {
	template := podTemplate(obj)
	if template == nil {
		return false
	}
	for instType := range images {
		key := instrumentation.InjectAnnotationKey(instType)
		if key == "" {
			continue
		}
		if template.Annotations[key] != "false" {
			return false
		}
	}
	return true
}

// ReadRecord parses the workload's guard record. ok is false when the annotation is absent; a
// present but unparseable annotation is an error.
func ReadRecord(obj client.Object) (*Record, bool, error) {
	encoded, ok := obj.GetAnnotations()[RecordAnnotationKey]
	if !ok {
		return nil, false, nil
	}
	rec := &Record{}
	if err := json.Unmarshal([]byte(encoded), rec); err != nil {
		return nil, true, fmt.Errorf("failed to parse the %s annotation: %w", RecordAnnotationKey, err)
	}
	return rec, true, nil
}

// Marshal renders the record as the annotation value.
func (rec *Record) Marshal() (string, error) {
	encoded, err := json.Marshal(rec)
	if err != nil {
		return "", fmt.Errorf("failed to serialize the instrumentation guard record: %w", err)
	}
	return string(encoded), nil
}
