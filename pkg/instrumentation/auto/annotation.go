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
	injectAnnotations   map[string]struct{}
	monitor             *Monitor
	cfg                 *AnnotationConfig
}

// IsManaged returns if AnnotationMutators would ever mutate the object.
func (m *AnnotationMutators) IsManaged(obj client.Object) bool {
	return len(m.cfg.GetObjectLanguagesToAnnotate(obj)) > 0
}

// RestartNamespace sets the restartedAtAnnotation for each of the namespace's supported resources and patches them.
func (m *AnnotationMutators) RestartNamespace(ctx context.Context, namespace *corev1.Namespace, mutatedAnnotations map[string]string) {
	m.rangeObjectList(ctx, &appsv1.DeploymentList{}, client.InNamespace(namespace.Name),
		chainCallbacks(m.shouldRestartFunc(mutatedAnnotations), m.patchFunc(ctx, setRestartAnnotation)))
	m.rangeObjectList(ctx, &appsv1.DaemonSetList{}, client.InNamespace(namespace.Name),
		chainCallbacks(m.shouldRestartFunc(mutatedAnnotations), m.patchFunc(ctx, setRestartAnnotation)))
	m.rangeObjectList(ctx, &appsv1.StatefulSetList{}, client.InNamespace(namespace.Name),
		chainCallbacks(m.shouldRestartFunc(mutatedAnnotations), m.patchFunc(ctx, setRestartAnnotation)))
}

// MutateAndPatchAll runs the mutators for each of the supported resources and patches them.
func (m *AnnotationMutators) MutateAndPatchAll(ctx context.Context) {
	m.rangeObjectList(ctx, &appsv1.DeploymentList{}, &client.ListOptions{}, m.patchFunc(ctx, m.mutateObject))
	m.rangeObjectList(ctx, &appsv1.DaemonSetList{}, &client.ListOptions{}, m.patchFunc(ctx, m.mutateObject))
	m.rangeObjectList(ctx, &appsv1.StatefulSetList{}, &client.ListOptions{}, m.patchFunc(ctx, m.mutateObject))
	m.rangeObjectList(ctx, &corev1.NamespaceList{}, &client.ListOptions{},
		chainCallbacks(m.patchFunc(ctx, m.mutateObject), m.restartNamespaceFunc(ctx)),
	)
}

// MutateObject modifies annotations for a single object using the configured mutators.
func (m *AnnotationMutators) MutateObject(obj client.Object) (any, bool) {
	return m.mutateObject(obj, nil)
}

// mutateObject modifies annotations for a single object using the configured mutators.
func (m *AnnotationMutators) mutateObject(obj client.Object, _ any) (any, bool) {
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
		return nil, false
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
			fn(&item, nil)
		}
	case *appsv1.DeploymentList:
		for _, item := range l.Items {
			fn(&item, nil)
		}
	case *appsv1.DaemonSetList:
		for _, item := range l.Items {
			fn(&item, nil)
		}
	case *appsv1.StatefulSetList:
		for _, item := range l.Items {
			fn(&item, nil)
		}
	}
}

func (m *AnnotationMutators) mutate(name string, mutators map[string]instrumentation.AnnotationMutator, obj metav1.Object) (map[string]string, bool) {
	mutator, ok := mutators[name]
	if !ok {
		mutator = m.defaultMutator
	}
	mutatedAnnotations := mutator.Mutate(obj)
	return mutatedAnnotations, len(mutatedAnnotations) != 0
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
		injectAnnotations:   buildInjectAnnotations(typeSet),
		cfg:                 &cfg,
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
// The insert mutation is configured to modify for any missing annotation key.
// The remove mutation is configured to only modify if all annotation keys are present.
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

// buildInjectAnnotations returns the set of inject annotations corresponding to the instrumentation types
func buildInjectAnnotations(instTypeSet instrumentation.TypeSet) map[string]struct{} {
	ret := map[string]struct{}{}
	for instType := range instTypeSet {
		ret[instrumentation.InjectAnnotationKey(instType)] = struct{}{}
	}
	return ret
}

// AnnotateKey joins the auto-annotate annotation prefix with the provided instrumentation.Type.
func AnnotateKey(instType instrumentation.Type) string {
	return autoAnnotatePrefix + strings.ToLower(string(instType))
}
