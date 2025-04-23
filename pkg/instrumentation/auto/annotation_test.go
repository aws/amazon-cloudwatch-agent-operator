// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		"SingleAnnotation": {
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
					AnnotateKey(instrumentation.TypeJava):                         defaultAnnotationValue,
				},
				"manual-auto": {
					instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue,
					AnnotateKey(instrumentation.TypeJava):                         defaultAnnotationValue,
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
			fakeClient := fake.NewClientBuilder().WithLists(&corev1.NamespaceList{Items: namespaces}).Build()
			mutators := NewAnnotationMutators(
				fakeClient,
				fakeClient,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			MutateAndPatchAll(mutators, ctx, false)
			gotNamespaces := &corev1.NamespaceList{}
			require.NoError(t, fakeClient.List(ctx, gotNamespaces))
			for _, gotNamespace := range gotNamespaces.Items {
				annotations, ok := testCase.want[gotNamespace.Name]
				assert.True(t, ok)
				assert.Equalf(t, annotations, gotNamespace.GetAnnotations(), "Failed for %s", gotNamespace.Name)
			}
		})
	}
}

func TestAnnotationMutators_Namespaces_Restart(t *testing.T) {
	cfg := AnnotationConfig{
		Java: AnnotationResources{
			Namespaces:  []string{"default"},
			Deployments: []string{"default/deployment-no-restart"},
		},
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	defaultDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      "deployment",
		},
	}
	deploymentNoRestart := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      "deployment-no-restart",
		},
	}
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      "daemonset",
		},
	}
	daemonSetNoRestart := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      "daemonset-no-restart",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue,
					},
				},
			},
		},
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      "statefulset",
		},
	}
	otherDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "other",
			Name:      "deployment",
		},
	}
	namespacedRestartExpectedResources := []client.Object{defaultDeployment, daemonSet, statefulSet}
	namespacedRestartNotExpectedResources := []client.Object{
		deploymentNoRestart, // explicitly auto-annotated instrumented resource should not be restarted
		daemonSetNoRestart,  // manually instrumented resource should not be restarted
		otherDeployment,     // resource in non-configured namespace should not be restarted/updated
	}
	fakeClient := fake.NewFakeClient(namespace, defaultDeployment, deploymentNoRestart, daemonSet, daemonSetNoRestart,
		statefulSet, otherDeployment)
	mutators := NewAnnotationMutators(
		fakeClient,
		fakeClient,
		logr.Logger{},
		cfg,
		instrumentation.NewTypeSet(instrumentation.TypeJava),
	)
	mutators.MutateAndPatchAll(context.Background())
	ctx := context.Background()
	for _, namespacedResource := range namespacedRestartExpectedResources {
		assert.NoError(t, fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedResource), namespacedResource))
		obj := getAnnotationObjectMeta(namespacedResource)
		assert.NotNil(t, obj)
		annotations := obj.GetAnnotations()
		assert.NotNil(t, annotations)
		assert.NotEmpty(t, annotations[restartedAtAnnotation])
	}
	for _, namespacedResource := range namespacedRestartNotExpectedResources {
		assert.NoError(t, fakeClient.Get(ctx, client.ObjectKeyFromObject(namespacedResource), namespacedResource))
		obj := getAnnotationObjectMeta(namespacedResource)
		assert.NotNil(t, obj)
		annotations := obj.GetAnnotations()
		if annotations != nil {
			assert.Empty(t, annotations[restartedAtAnnotation])
		}
	}
}

func getAnnotationObjectMeta(obj client.Object) metav1.Object {
	switch o := obj.(type) {
	case *corev1.Namespace:
		return o.GetObjectMeta()
	case *appsv1.Deployment:
		return o.Spec.Template.GetObjectMeta()
	case *appsv1.DaemonSet:
		return o.Spec.Template.GetObjectMeta()
	case *appsv1.StatefulSet:
		return o.Spec.Template.GetObjectMeta()
	default:
		return nil
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
			var deployments []appsv1.Deployment
			for name, annotations := range testCase.deployments {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				deployments = append(deployments, appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			fakeClient := fake.NewClientBuilder().WithLists(&appsv1.DeploymentList{Items: deployments}).Build()
			mutators := NewAnnotationMutators(
				fakeClient,
				fakeClient,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			MutateAndPatchAll(mutators, ctx, false)
			gotDeployments := &appsv1.DeploymentList{}
			require.NoError(t, fakeClient.List(ctx, gotDeployments))
			for _, gotDeployment := range gotDeployments.Items {
				name := namespacedName(gotDeployment.GetObjectMeta())
				annotations, ok := testCase.want[name]
				assert.True(t, ok)
				assert.Equalf(t, annotations, gotDeployment.Spec.Template.GetAnnotations(), "Failed for %s", name)
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
			var daemonSets []appsv1.DaemonSet
			for name, annotations := range testCase.daemonSets {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				daemonSets = append(daemonSets, appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			fakeClient := fake.NewClientBuilder().WithLists(&appsv1.DaemonSetList{Items: daemonSets}).Build()
			mutators := NewAnnotationMutators(
				fakeClient,
				fakeClient,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			MutateAndPatchAll(mutators, ctx, false)
			gotDaemonSets := &appsv1.DaemonSetList{}
			require.NoError(t, fakeClient.List(ctx, gotDaemonSets))
			for _, gotDaemonSet := range gotDaemonSets.Items {
				name := namespacedName(gotDaemonSet.GetObjectMeta())
				annotations, ok := testCase.want[name]
				assert.True(t, ok)
				assert.Equalf(t, annotations, gotDaemonSet.Spec.Template.GetAnnotations(), "Failed for %s", name)
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
			var statefulSets []appsv1.StatefulSet
			for name, annotations := range testCase.statefulSets {
				var namespace string
				namespace, name, _ = strings.Cut(name, "/")
				statefulSets = append(statefulSets, appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				})
			}
			ctx := context.Background()
			fakeClient := fake.NewClientBuilder().WithLists(&appsv1.StatefulSetList{Items: statefulSets}).Build()
			mutators := NewAnnotationMutators(
				fakeClient,
				fakeClient,
				logr.Logger{},
				testCase.cfg,
				testCase.typeSet,
			)
			MutateAndPatchAll(mutators, ctx, false)
			gotStatefulSets := &appsv1.StatefulSetList{}
			require.NoError(t, fakeClient.List(ctx, gotStatefulSets))
			for _, gotStatefulSet := range gotStatefulSets.Items {
				name := namespacedName(gotStatefulSet.GetObjectMeta())
				annotations, ok := testCase.want[name]
				assert.True(t, ok)
				assert.Equalf(t, annotations, gotStatefulSet.Spec.Template.GetAnnotations(), "Failed for %s", name)
			}
		})
	}
}

type mockClient struct {
	mock.Mock
	client.Writer
	client.Reader
}

func (c *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := c.Called(ctx, list, opts)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (c *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := c.Called(ctx, obj, opts)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (c *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func TestAnnotationMutators_ClientErrors(t *testing.T) {
	err := errors.New("test error")
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	cfg := AnnotationConfig{
		Java: AnnotationResources{
			Namespaces: []string{"test"},
		},
	}
	errClient := new(mockClient)
	errClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(err)
	errClient.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(err)
	fakeClient := fake.NewClientBuilder().WithLists(&corev1.NamespaceList{Items: []corev1.Namespace{namespace}}).Build()
	mutators := NewAnnotationMutators(
		fakeClient,
		errClient,
		logr.Logger{},
		cfg,
		instrumentation.NewTypeSet(instrumentation.TypeJava),
	)
	MutateAndPatchAll(mutators, context.Background(), false)
	errClient.AssertCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)
	mutators.clientWriter = errClient
	mutators.clientReader = fakeClient
	MutateAndPatchAll(mutators, context.Background(), false)
	errClient.AssertCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
