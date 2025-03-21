// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workloadmutation

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/go-logr/logr"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation/auto"
)

var (
	k8sClient client.Client
)

func TestHandle(t *testing.T) {
	for _, tt := range []struct {
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
			name: "invalid empty daemon-set payload",
			req: func() admission.Request {
				ds := appsv1.DaemonSet{}
				encoded, err := json.Marshal(ds)
				require.NoError(t, err)

				return admission.Request{
					AdmissionRequest: admv1.AdmissionRequest{
						Namespace: "testing",
						Object: runtime.RawExtension{
							Raw: encoded,
						},
					},
				}
			}(),
			expected: http.StatusBadRequest,
			allowed:  false,
		},
		{
			name: "invalid pod payload",
			req: func() admission.Request {
				pod := corev1.Pod{}
				encoded, err := json.Marshal(pod)
				require.NoError(t, err)

				return admission.Request{
					AdmissionRequest: admv1.AdmissionRequest{
						Namespace: "testing",
						Object: runtime.RawExtension{
							Raw: encoded,
						},
					},
				}
			}(),
			expected: http.StatusBadRequest,
			allowed:  false,
		},
		{
			name: "valid workload payload",
			req: func() admission.Request {
				ds := appsv1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						Kind: "DaemonSet",
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{},
							},
						},
					},
				}
				encoded, err := json.Marshal(ds)
				require.NoError(t, err)

				return admission.Request{
					AdmissionRequest: admv1.AdmissionRequest{
						Kind: metav1.GroupVersionKind{
							Kind: "DaemonSet",
						},
						Namespace: "testing",
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
		t.Run(tt.name, func(t *testing.T) {
			// prepare
			decoder := admission.NewDecoder(scheme.Scheme)
			autoAnnotationConfig := auto.AnnotationConfig{
				Java: auto.AnnotationResources{
					Namespaces: []string{"keep-auto-java"},
				},
			}
			mutators := auto.NewAnnotationMutators(
				k8sClient,
				k8sClient,
				logr.Logger{},
				autoAnnotationConfig,
				instrumentation.NewTypeSet(instrumentation.TypeJava),
			)
			injector := NewWebhookHandler(decoder, mutators)

			// test
			res := injector.Handle(context.Background(), tt.req)

			// verify
			assert.Equal(t, tt.allowed, res.Allowed)
			if !tt.allowed {
				assert.NotNil(t, res.AdmissionResponse.Result)
				assert.Equal(t, tt.expected, res.AdmissionResponse.Result.Code)
			}
		})
	}
}
