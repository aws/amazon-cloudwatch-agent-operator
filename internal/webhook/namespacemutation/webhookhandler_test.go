// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacemutation

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

func TestHandle(t *testing.T) {
	// prepare
	client := fake.NewFakeClient()
	decoder := admission.NewDecoder(scheme.Scheme)
	autoAnnotationConfig := auto.AnnotationConfig{
		Java: auto.AnnotationResources{
			Namespaces: []string{"auto-java"},
		},
	}
	mutators := auto.NewAnnotationMutators(
		client,
		client,
		logr.Logger{},
		autoAnnotationConfig,
		instrumentation.NewTypeSet(instrumentation.TypeJava),
		nil,
	)
	h := NewWebhookHandler(decoder, mutators, nil)
	for _, testCase := range []struct {
		req      admission.Request
		name     string
		expected int32
		allowed  bool
	}{
		{
			name:     "empty payload",
			req:      admission.Request{},
			expected: http.StatusBadRequest,
			allowed:  false,
		},
		{
			name: "valid workload payload",
			req: func() admission.Request {
				ns := corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "auto-java",
					},
				}
				encoded, err := json.Marshal(ns)
				require.NoError(t, err)

				return admission.Request{
					AdmissionRequest: admv1.AdmissionRequest{
						Object: runtime.RawExtension{
							Raw: encoded,
						},
					},
				}
			}(),
			expected: http.StatusOK,
			allowed:  true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// test
			res := h.Handle(context.Background(), testCase.req)

			// verify
			assert.Equal(t, testCase.allowed, res.Allowed)
			if !testCase.allowed {
				assert.NotNil(t, res.AdmissionResponse.Result)
				assert.Equal(t, testCase.expected, res.AdmissionResponse.Result.Code)
			}
		})
	}
}
