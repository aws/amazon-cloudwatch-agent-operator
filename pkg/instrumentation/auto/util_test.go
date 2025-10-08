// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake2 "k8s.io/client-go/discovery/fake"
	"os"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateInstrumentationAnnotator(t *testing.T) {
	// Setup
	fakeClient := fakeclient.NewClientBuilder().Build()
	ctx := context.Background()
	logger := testr.New(t)

	tests := []struct {
		name                 string
		envDisableAnnotation bool
		envDisableMonitor    bool
		autoAnnotationConfig string
		autoMonitorConfig    string
		otelSetupExists      bool
		expectNilAnnotator   bool
		expectedType         string
	}{
		{
			name:                 "Both annotation and monitor disabled",
			envDisableAnnotation: true,
			envDisableMonitor:    true,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   true,
			expectedType:         "",
		},
		{
			name:                 "Annotation enabled, valid config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.AnnotationMutators",
		},
		{
			name:                 "Annotation disabled, Monitor enabled with monitorAllServices=true",
			envDisableAnnotation: true,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Annotation disabled, Monitor enabled with monitorAllServices=false",
			envDisableAnnotation: true,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{"monitorAllServices":false}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Invalid annotation config, valid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{invalid-json}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Empty annotation config, valid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{}`,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.Monitor",
		},
		{
			name:                 "Valid annotation config, invalid monitor config",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			autoAnnotationConfig: `{"java":{"deployments":["default/myapp"]}}`,
			autoMonitorConfig:    `{invalid-json}`,
			expectNilAnnotator:   false,
			expectedType:         "*auto.AnnotationMutators",
		},
		{
			name:                 "Disable auto monitor with otel setup found",
			envDisableAnnotation: false,
			envDisableMonitor:    false,
			otelSetupExists:      true,
			autoAnnotationConfig: ``,
			autoMonitorConfig:    `{"monitorAllServices":true}`,
			expectNilAnnotator:   true,
			expectedType:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.envDisableAnnotation {
				os.Setenv("DISABLE_AUTO_ANNOTATION", "true")
			} else {
				os.Unsetenv("DISABLE_AUTO_ANNOTATION")
			}

			if tt.envDisableMonitor {
				os.Setenv("DISABLE_AUTO_MONITOR", "true")
			} else {
				os.Unsetenv("DISABLE_AUTO_MONITOR")
			}

			fakeClientset := fake.NewSimpleClientset()
			if tt.otelSetupExists {
				fakeDiscovery, ok := fakeClientset.Discovery().(*fake2.FakeDiscovery)
				if !ok {
					t.Fatal("couldn't convert Discovery() to *FakeDiscovery")
				}

				fakeDiscovery.Resources = []*metav1.APIResourceList{
					{
						GroupVersion: "opentelemetry.io/v1alpha1",
						APIResources: []metav1.APIResource{
							{
								Name:       "instrumentations",
								Namespaced: true,
								Kind:       "Instrumentation",
								Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
							},
						},
					},
				}
			}
			// Call the function
			annotator := createInstrumentationAnnotatorWithClientset(tt.autoMonitorConfig, tt.autoAnnotationConfig, ctx, fakeClientset, fakeClient, fakeClient, logger)

			// Check results
			if tt.expectNilAnnotator {
				assert.Nil(t, annotator, "Expected nil annotator")
			} else {
				assert.NotNil(t, annotator, "Expected non-nil annotator")

				// Check type using type assertion
				actualType := fmt.Sprintf("%T", annotator)
				assert.Equal(t, tt.expectedType, actualType, "Unexpected annotator type")

				// Specific type assertions
				switch tt.expectedType {
				case "*auto.AnnotationMutators":
					_, ok := annotator.(*AnnotationMutators)
					assert.True(t, ok, "Expected annotator to be of type *AnnotationMutators")
				case "*auto.Monitor":
					_, ok := annotator.(*Monitor)
					assert.True(t, ok, "Expected annotator to be of type *Monitor")
				}
			}
		})
	}
}
