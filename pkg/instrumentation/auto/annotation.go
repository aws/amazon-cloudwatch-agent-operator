// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

const (
	autoAnnotatePrefix     = "cloudwatch.aws.amazon.com/auto-annotate-"
	defaultAnnotationValue = "true"
)

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;patch
// +kubebuilder:rbac:groups="apps",resources=daemonsets;deployments;statefulsets,verbs=list;patch

// AnnotationMutators contains functions that can be used to mutate annotations
// on all supported objects based on the configured mutators.
type AnnotationMutators struct {
	clientWriter        client.Writer
	clientReader        client.Reader
	logger              logr.Logger
	namespaceMutators   map[string]instrumentation.AnnotationMutator
	deploymentMutators  map[string]instrumentation.AnnotationMutator
	daemonSetMutators   map[string]instrumentation.AnnotationMutator
	statefulSetMutators map[string]instrumentation.AnnotationMutator
	defaultMutator      instrumentation.AnnotationMutator
}

// RestartNamespace sets the restartedAtAnnotation for each of the namespace's supported resources and patches them.
func (m *AnnotationMutators) RestartNamespace(ctx context.Context, namespace *corev1.Namespace) {
	restartAndPatchFunc := m.patchFunc(ctx, setRestartAnnotation)
	m.rangeObjectList(ctx, &appsv1.DeploymentList{}, client.InNamespace(namespace.Name), restartAndPatchFunc)
	m.rangeObjectList(ctx, &appsv1.DaemonSetList{}, client.InNamespace(namespace.Name), restartAndPatchFunc)
	m.rangeObjectList(ctx, &appsv1.StatefulSetList{}, client.InNamespace(namespace.Name), restartAndPatchFunc)
}

// MutateAndPatchAll runs the mutators for each of the supported resources and patches them.
func (m *AnnotationMutators) MutateAndPatchAll(ctx context.Context) {
	mutateAndPatchFunc := m.patchFunc(ctx, m.MutateObject)
	m.rangeObjectList(ctx, &corev1.NamespaceList{}, &client.ListOptions{},
		chainCallbacks(mutateAndPatchFunc, m.restartNamespaceFunc(ctx)),
	)
	m.rangeObjectList(ctx, &appsv1.DeploymentList{}, &client.ListOptions{}, mutateAndPatchFunc)
	m.rangeObjectList(ctx, &appsv1.DaemonSetList{}, &client.ListOptions{}, mutateAndPatchFunc)
	m.rangeObjectList(ctx, &appsv1.StatefulSetList{}, &client.ListOptions{}, mutateAndPatchFunc)
}

// MutateObject modifies annotations for a single object using the configured mutators.
func (m *AnnotationMutators) MutateObject(obj client.Object) bool {
	switch o := obj.(type) {
	case *corev1.Namespace:
		return m.mutate(o.GetName(), m.namespaceMutators, o.GetObjectMeta())
	case *appsv1.Deployment:
		return m.mutate(namespacedName(o.GetObjectMeta()), m.deploymentMutators, o.Spec.Template.GetObjectMeta())
	case *appsv1.DaemonSet:
		return m.mutate(namespacedName(o.GetObjectMeta()), m.daemonSetMutators, o.Spec.Template.GetObjectMeta())
	case *appsv1.StatefulSet:
		return m.mutate(namespacedName(o.GetObjectMeta()), m.statefulSetMutators, o.Spec.Template.GetObjectMeta())
	default:
		return false
	}
}

func (m *AnnotationMutators) rangeObjectList(ctx context.Context, list client.ObjectList, option client.ListOption, fn objectCallbackFunc) {
	if err := m.clientReader.List(ctx, list, option); err != nil {
		m.logger.Error(err, "Unable to list objects",
			"kind", fmt.Sprintf("%T", list),
		)
		return
	}
	switch l := list.(type) {
	case *corev1.NamespaceList:
		for _, item := range l.Items {
			fn(&item)
		}
	case *appsv1.DeploymentList:
		for _, item := range l.Items {
			fn(&item)
		}
	case *appsv1.DaemonSetList:
		for _, item := range l.Items {
			fn(&item)
		}
	case *appsv1.StatefulSetList:
		for _, item := range l.Items {
			fn(&item)
		}
	}
}

func (m *AnnotationMutators) mutate(name string, mutators map[string]instrumentation.AnnotationMutator, obj metav1.Object) bool {
	mutator, ok := mutators[name]
	if !ok {
		mutator = m.defaultMutator
	}
	return mutator.Mutate(obj)
}

func namespacedName(obj metav1.Object) string {
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}

// NewAnnotationMutators creates mutators based on the AnnotationConfig provided and enabled instrumentation.TypeSet.
// The default mutator, which is used for non-configured resources, removes all auto-annotated annotations in the type
// set.
func NewAnnotationMutators(
	clientWriter client.Writer,
	clientReader client.Reader,
	logger logr.Logger,
	cfg AnnotationConfig,
	typeSet instrumentation.TypeSet,
) *AnnotationMutators {
	builder := newMutatorBuilder(typeSet)
	return &AnnotationMutators{
		clientWriter:        clientWriter,
		clientReader:        clientReader,
		logger:              logger,
		namespaceMutators:   builder.buildMutators(getResources(cfg, typeSet, getNamespaces)),
		deploymentMutators:  builder.buildMutators(getResources(cfg, typeSet, getDeployments)),
		daemonSetMutators:   builder.buildMutators(getResources(cfg, typeSet, getDaemonSets)),
		statefulSetMutators: builder.buildMutators(getResources(cfg, typeSet, getStatefulSets)),
		defaultMutator:      instrumentation.NewAnnotationMutator(maps.Values(builder.removeMutations)),
	}
}

func getResources(
	cfg AnnotationConfig,
	typeSet instrumentation.TypeSet,
	resourceFn func(AnnotationResources) []string,
) map[instrumentation.Type][]string {
	resources := map[instrumentation.Type][]string{}
	for instType := range typeSet {
		resources[instType] = resourceFn(cfg.getResources(instType))
	}
	return resources
}

type mutatorBuilder struct {
	typeSet         instrumentation.TypeSet
	insertMutations map[instrumentation.Type]instrumentation.AnnotationMutation
	removeMutations map[instrumentation.Type]instrumentation.AnnotationMutation
}

func (b *mutatorBuilder) buildMutators(resources map[instrumentation.Type][]string) map[string]instrumentation.AnnotationMutator {
	mutators := map[string]instrumentation.AnnotationMutator{}
	typeSetByResource := map[string]instrumentation.TypeSet{}
	for instType, resourceNames := range resources {
		for _, resourceName := range resourceNames {
			typeSet, ok := typeSetByResource[resourceName]
			if !ok {
				typeSet = instrumentation.NewTypeSet()
			}
			typeSet[instType] = nil
			typeSetByResource[resourceName] = typeSet
		}
	}
	for resourceName, typeSet := range typeSetByResource {
		var mutations []instrumentation.AnnotationMutation
		for instType := range b.typeSet {
			if _, ok := typeSet[instType]; ok {
				mutations = append(mutations, b.insertMutations[instType])
			} else {
				mutations = append(mutations, b.removeMutations[instType])
			}
		}
		mutators[resourceName] = instrumentation.NewAnnotationMutator(mutations)
	}
	return mutators
}

func newMutatorBuilder(typeSet instrumentation.TypeSet) *mutatorBuilder {
	builder := &mutatorBuilder{
		typeSet:         typeSet,
		insertMutations: map[instrumentation.Type]instrumentation.AnnotationMutation{},
		removeMutations: map[instrumentation.Type]instrumentation.AnnotationMutation{},
	}
	for instType := range typeSet {
		builder.insertMutations[instType], builder.removeMutations[instType] = buildMutations(instType)
	}
	return builder
}

// buildMutations builds insert and remove annotation mutations for the instrumentation.Type.
// Both are configured to only modify the annotations if all annotation keys are missing or present respectively.
func buildMutations(instType instrumentation.Type) (instrumentation.AnnotationMutation, instrumentation.AnnotationMutation) {
	annotations := buildAnnotations(instType)
	return instrumentation.NewInsertAnnotationMutation(annotations),
		instrumentation.NewRemoveAnnotationMutation(maps.Keys(annotations))
}

// buildAnnotations creates an annotation map of the inject and auto-annotate keys.
func buildAnnotations(instType instrumentation.Type) map[string]string {
	return map[string]string{
		instrumentation.InjectAnnotationKey(instType): defaultAnnotationValue,
		AnnotateKey(instType):                         defaultAnnotationValue,
	}
}

// AnnotateKey joins the auto-annotate annotation prefix with the provided instrumentation.Type.
func AnnotateKey(instType instrumentation.Type) string {
	return autoAnnotatePrefix + strings.ToLower(string(instType))
}
