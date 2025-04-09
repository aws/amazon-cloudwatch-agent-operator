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
	cfg                 AnnotationConfig
}

func (m *AnnotationMutators) GetAnnotationMutators() *AnnotationMutators {
	return m
}

func (m *AnnotationMutators) GetLogger() logr.Logger {
	return m.logger
}

func (m *AnnotationMutators) GetReader() client.Reader {
	return m.clientReader
}

func (m *AnnotationMutators) GetWriter() client.Writer {
	return m.clientWriter
}

// MutateObject modifies annotations for a single object using the configured mutators.
func (m *AnnotationMutators) MutateObject(_ client.Object, obj client.Object) map[string]string {
	mutatedAnnotations, _ := m.mutateObject(obj, nil)
	annotations, ok := mutatedAnnotations.(map[string]string)
	if !ok {
		m.logger.Error(nil, "could not cast annotations to map")
	}
	return annotations
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

func rangeObjectList(m InstrumentationAnnotator, ctx context.Context, list client.ObjectList, option client.ListOption, fn objectCallbackFunc) {
	if err := m.GetReader().List(ctx, list, option); err != nil {
		m.GetLogger().Error(err, "Unable to list objects",
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

func (m *AnnotationMutators) Empty() bool {
	return m.cfg.Empty()
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
	warnNonNamespacedNames(typeSet, cfg, logger)
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
		cfg:                 cfg,
	}
}

func warnNonNamespacedNames(typeSet instrumentation.TypeSet, cfg AnnotationConfig, logger logr.Logger) {
	for t := range typeSet {
		resources := cfg.getResources(t)
		for _, deployment := range resources.Deployments {
			if !strings.Contains(deployment, "/") {
				logger.Info("invalid deployment name, needs to be namespaced", "deployment", deployment)
			}
		}
		for _, daemonSet := range resources.DaemonSets {
			if !strings.Contains(daemonSet, "/") {
				logger.Info("invalid daemonSet name, needs to be namespaced", "daemonSet", daemonSet)
			}
		}
		for _, statefulSet := range resources.StatefulSets {
			if !strings.Contains(statefulSet, "/") {
				logger.Info("invalid statefulSet name, needs to be namespaced", "statefulSet", statefulSet)
			}
		}
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
