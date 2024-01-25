// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

func TestAnnotationMutators_Namespaces(t *testing.T) {
	testCases := map[string]struct {
		typeSet    instrumentation.TypeSet
		namespaces map[string]map[string]string
		cfg        AnnotationConfig
		want       map[string]map[string]string
	}{
		"SkipManualAnnotations": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava),
			namespaces: map[string]map[string]string{
				"manual-inject": {
					instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue,
				},
				"manual-auto": {
					AnnotateKey(instrumentation.TypeJava): defaultAnnotationValue,
				},
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					Namespaces: []string{"manual-inject", "manual-auto"},
				},
			},
			want: map[string]map[string]string{
				"manual-inject": {
					instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue,
				},
				"manual-auto": {
					AnnotateKey(instrumentation.TypeJava): defaultAnnotationValue,
				},
			},
		},
		"RemoveAuto": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava, instrumentation.TypePython),
			namespaces: map[string]map[string]string{
				"remove-auto-java": buildAnnotations(instrumentation.TypeJava),
				"remove-auto-python": mergeAnnotations(
					buildAnnotations(instrumentation.TypePython),
					map[string]string{"keep-other-field": "remains"},
				),
				"keep-auto-java": mergeAnnotations(
					buildAnnotations(instrumentation.TypeJava),
					buildAnnotations(instrumentation.TypePython),
				),
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					Namespaces: []string{"keep-auto-java"},
				},
			},
			want: map[string]map[string]string{
				"remove-auto-java":   nil,
				"remove-auto-python": {"keep-other-field": "remains"},
				"keep-auto-java":     buildAnnotations(instrumentation.TypeJava),
			},
		},
		"AddAuto": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava),
			namespaces: map[string]map[string]string{
				"add-auto-java": buildAnnotations(instrumentation.TypePython),
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					Namespaces: []string{"add-auto-java"},
				},
			},
			want: map[string]map[string]string{
				"add-auto-java": mergeAnnotations(
					buildAnnotations(instrumentation.TypeJava),
					buildAnnotations(instrumentation.TypePython),
				),
			},
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			var namespaces []corev1.Namespace
			for name, annotations := range testCase.namespaces {
				namespaces = append(namespaces, corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Annotations: annotations,
					},
				})
			}
			ctx := context.Background()
			client := fake.NewClientBuilder().WithLists(&corev1.NamespaceList{Items: namespaces}).Build()
			mutators := NewAnnotationMutators(
				client,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			mutators.MutateAll(ctx)
			gotNamespaces := &corev1.NamespaceList{}
			require.NoError(t, client.List(ctx, gotNamespaces))
			for _, gotNamespace := range gotNamespaces.Items {
				annotations, ok := testCase.want[gotNamespace.Name]
				assert.True(t, ok)
				assert.Equal(t, annotations, gotNamespace.GetAnnotations())
			}
		})
	}
}

func TestAnnotationMutators_Deployments(t *testing.T) {
	testCases := map[string]struct {
		typeSet     instrumentation.TypeSet
		deployments map[string]map[string]string
		cfg         AnnotationConfig
		want        map[string]map[string]string
	}{
		"AddRemoveAuto": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava),
			deployments: map[string]map[string]string{
				"test/add-auto-java":    nil,
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": buildAnnotations(instrumentation.TypeJava),
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					Deployments: []string{"test/add-auto-java", "test/keep-auto-java"},
				},
			},
			want: map[string]map[string]string{
				"test/add-auto-java":    buildAnnotations(instrumentation.TypeJava),
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": nil,
			},
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			var deployments []appv1.Deployment
			for name, annotations := range testCase.deployments {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				deployments = append(deployments, appv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			client := fake.NewClientBuilder().WithLists(&appv1.DeploymentList{Items: deployments}).Build()
			mutators := NewAnnotationMutators(
				client,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			mutators.MutateAll(ctx)
			gotDeployments := &appv1.DeploymentList{}
			require.NoError(t, client.List(ctx, gotDeployments))
			for _, gotDeployment := range gotDeployments.Items {
				annotations, ok := testCase.want[namespacedName(gotDeployment.GetObjectMeta())]
				assert.True(t, ok)
				assert.Equal(t, annotations, gotDeployment.Spec.Template.GetAnnotations())
			}
		})
	}
}

func TestAnnotationMutators_DaemonSets(t *testing.T) {
	testCases := map[string]struct {
		typeSet    instrumentation.TypeSet
		daemonSets map[string]map[string]string
		cfg        AnnotationConfig
		want       map[string]map[string]string
	}{
		"AddKeepRemoveAuto": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava),
			daemonSets: map[string]map[string]string{
				"test/add-auto-java":    nil,
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": buildAnnotations(instrumentation.TypeJava),
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					DaemonSets: []string{"test/add-auto-java", "test/keep-auto-java"},
				},
			},
			want: map[string]map[string]string{
				"test/add-auto-java":    buildAnnotations(instrumentation.TypeJava),
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": nil,
			},
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			var daemonSets []appv1.DaemonSet
			for name, annotations := range testCase.daemonSets {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				daemonSets = append(daemonSets, appv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			client := fake.NewClientBuilder().WithLists(&appv1.DaemonSetList{Items: daemonSets}).Build()
			mutators := NewAnnotationMutators(
				client,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			mutators.MutateAll(ctx)
			gotDaemonSets := &appv1.DaemonSetList{}
			require.NoError(t, client.List(ctx, gotDaemonSets))
			for _, gotDaemonSet := range gotDaemonSets.Items {
				annotations, ok := testCase.want[namespacedName(gotDaemonSet.GetObjectMeta())]
				assert.True(t, ok)
				assert.Equal(t, annotations, gotDaemonSet.Spec.Template.GetAnnotations())
			}
		})
	}
}

func TestAnnotationMutators_StatefulSets(t *testing.T) {
	testCases := map[string]struct {
		typeSet      instrumentation.TypeSet
		statefulSets map[string]map[string]string
		cfg          AnnotationConfig
		want         map[string]map[string]string
	}{
		"AddRemoveAuto": {
			typeSet: instrumentation.NewTypeSet(instrumentation.TypeJava),
			statefulSets: map[string]map[string]string{
				"test/add-auto-java":    nil,
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": buildAnnotations(instrumentation.TypeJava),
			},
			cfg: AnnotationConfig{
				Java: AnnotationResources{
					StatefulSets: []string{"test/add-auto-java", "test/keep-auto-java"},
				},
			},
			want: map[string]map[string]string{
				"test/add-auto-java":    buildAnnotations(instrumentation.TypeJava),
				"test/keep-auto-java":   buildAnnotations(instrumentation.TypeJava),
				"test/remove-auto-java": nil,
			},
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			var statefulSets []appv1.StatefulSet
			for name, annotations := range testCase.statefulSets {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				statefulSets = append(statefulSets, appv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			client := fake.NewClientBuilder().WithLists(&appv1.StatefulSetList{Items: statefulSets}).Build()
			mutators := NewAnnotationMutators(
				client,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			mutators.MutateAll(ctx)
			gotStatefulSets := &appv1.StatefulSetList{}
			require.NoError(t, client.List(ctx, gotStatefulSets))
			for _, gotStatefulSet := range gotStatefulSets.Items {
				annotations, ok := testCase.want[namespacedName(gotStatefulSet.GetObjectMeta())]
				assert.True(t, ok)
				assert.Equal(t, annotations, gotStatefulSet.Spec.Template.GetAnnotations())
			}
		})
	}
}

func TestAnnotateKey(t *testing.T) {
	testCases := []struct {
		instType instrumentation.Type
		want     string
	}{
		{instType: instrumentation.TypeJava, want: "cloudwatch.aws.amazon.com/auto-annotate-java"},
		{instType: instrumentation.TypeGo, want: "cloudwatch.aws.amazon.com/auto-annotate-go"},
		{instType: "unsupported", want: "cloudwatch.aws.amazon.com/auto-annotate-unsupported"},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.want, AnnotateKey(testCase.instType))
	}
}

func mergeAnnotations(annotationMaps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, annotationMap := range annotationMaps {
		for key, value := range annotationMap {
			merged[key] = value
		}
	}
	return merged
}
