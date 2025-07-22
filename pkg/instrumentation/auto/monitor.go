// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
)

var excludedNamespaces = []string{"kube-system", "amazon-cloudwatch"}

const (
	ByLabel              = "IndexByLabel"
	informerResyncPeriod = 10 * time.Minute
)

// InstrumentationAnnotator is the highest level abstraction used to annotate kubernetes resources for instrumentation
type InstrumentationAnnotator interface {
	MutateObject(oldObj client.Object, obj client.Object) any
	GetLogger() logr.Logger
	GetReader() client.Reader
	GetWriter() client.Writer
	MutateAndPatchAll(ctx context.Context)
}

type Monitor struct {
	serviceInformer     cache.SharedIndexInformer
	ctx                 context.Context
	config              MonitorConfig
	k8sInterface        kubernetes.Interface
	clientReader        client.Reader
	clientWriter        client.Writer
	logger              logr.Logger
	deploymentInformer  cache.SharedIndexInformer
	daemonsetInformer   cache.SharedIndexInformer
	statefulsetInformer cache.SharedIndexInformer
}

func (m *Monitor) MutateAndPatchAll(ctx context.Context) {
	if m.config.RestartPods {
		MutateAndPatchWorkloads(m, ctx)
	}
	MutateAndPatchNamespaces(m, ctx, m.config.RestartPods)
}

func (m *Monitor) GetLogger() logr.Logger {
	return m.logger
}

func (m *Monitor) GetReader() client.Reader {
	return m.clientReader
}

func (m *Monitor) GetWriter() client.Writer {
	return m.clientWriter
}

// NewMonitor is used to create an InstrumentationMutator that supports AutoMonitor.
func NewMonitor(ctx context.Context, config MonitorConfig, k8sClient kubernetes.Interface, w client.Writer, r client.Reader, logger logr.Logger) *Monitor {
	// Config default values
	if len(config.Languages) == 0 {
		logger.V(1).Info("Setting languages to default", "languages", instrumentation.SupportedTypes)
		config.Languages = instrumentation.SupportedTypes
	}

	logger.V(1).Info("AutoMonitor starting...")
	serviceFactory := informers.NewSharedInformerFactoryWithOptions(k8sClient, informerResyncPeriod)
	workloadFactory := informers.NewSharedInformerFactoryWithOptions(k8sClient, informerResyncPeriod)

	serviceInformer := serviceFactory.Core().V1().Services().Informer()
	err := serviceInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return obj, fmt.Errorf("error transforming service: %s not a service", obj)
		}
		// Return only the fields we need
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svc.Name,
				Namespace: svc.Namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: svc.Spec.Selector,
			},
		}, nil
	})
	if err != nil {
		logger.Error(err, "Setting service informer failed")
	}

	// create deployment informer
	deploymentInformer, err := createDeploymentInformer(workloadFactory)
	if err != nil {
		logger.Error(err, "Creating deployment informer failed")
	}

	// create daemonset informer
	daemonsetInformer, err := createDaemonsetInformer(workloadFactory)
	if err != nil {
		logger.Error(err, "Creating daemonset informer failed")
	}
	// create statefulset informer
	statefulSetInformer, err := createStatefulsetInformer(workloadFactory)
	if err != nil {
		logger.Error(err, "Creating statefulset informer failed")
	}

	warnNonNamespacedNames(config.Exclude, logger)

	m := &Monitor{
		serviceInformer:     serviceInformer,
		ctx:                 ctx,
		config:              config,
		k8sInterface:        k8sClient,
		clientReader:        r,
		clientWriter:        w,
		logger:              logger,
		deploymentInformer:  deploymentInformer,
		daemonsetInformer:   daemonsetInformer,
		statefulsetInformer: statefulSetInformer,
	}

	_, err = serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			m.onServiceEvent(nil, obj.(*corev1.Service))
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			m.onServiceEvent(oldObj.(*corev1.Service), obj.(*corev1.Service))
		},
		DeleteFunc: func(obj interface{}) {
			m.onServiceEvent(obj.(*corev1.Service), nil)
		},
	})
	if err != nil {
		logger.Error(err, "failed to start auto monitor")
		return nil
	}

	// initialize workload factory before service factory so workloads are available during onServiceEvent calls when
	// service informer is initialized
	factories := []informers.SharedInformerFactory{workloadFactory, serviceFactory}

	for _, factory := range factories {
		factory.Start(ctx.Done())
		synced := factory.WaitForCacheSync(ctx.Done())
		for v, ok := range synced {
			if !ok {
				logger.Error(fmt.Errorf("caches failed to sync: %v", v), "bad cache sync")
			}
		}
	}

	logger.V(1).Info("Initialization complete!")
	return m
}

func createDaemonsetInformer(workloadFactory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	daemonsetInformer := workloadFactory.Apps().V1().DaemonSets().Informer()
	err := daemonsetInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		daemonset, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			return obj, fmt.Errorf("error transforming daemonset: %s not a daemonset", obj)
		}
		return &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonset.Name,
				Namespace: daemonset.Namespace,
			},
			Spec: appsv1.DaemonSetSpec{
				Template: daemonset.Spec.Template,
			},
		}, nil
	})
	if err != nil {
		return nil, err
	}

	err = daemonsetInformer.AddIndexers(map[string]cache.IndexFunc{
		ByLabel: func(obj interface{}) ([]string, error) {
			return []string{labels.SelectorFromSet(obj.(*appsv1.DaemonSet).Spec.Template.Labels).String()}, nil
		},
	})
	return daemonsetInformer, err
}

func createStatefulsetInformer(workloadFactory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	statefulSetInformer := workloadFactory.Apps().V1().StatefulSets().Informer()
	err := statefulSetInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		statefulSet, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return obj, fmt.Errorf("error transforming statefulset: %s not a statefulset", obj)
		}
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulSet.Name,
				Namespace: statefulSet.Namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: statefulSet.Spec.Template,
			},
		}, nil
	})
	if err != nil {
		return nil, err
	}

	err = statefulSetInformer.AddIndexers(map[string]cache.IndexFunc{
		ByLabel: func(obj interface{}) ([]string, error) {
			return []string{labels.SelectorFromSet(obj.(*appsv1.StatefulSet).Spec.Template.Labels).String()}, nil
		},
	})
	return statefulSetInformer, err
}

func createDeploymentInformer(workloadFactory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	deploymentInformer := workloadFactory.Apps().V1().Deployments().Informer()
	err := deploymentInformer.SetTransform(func(obj interface{}) (interface{}, error) {
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return obj, fmt.Errorf("error transforming deployment: %s not a deployment", obj)
		}
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: deployment.Spec.Template,
			},
		}, nil
	})
	if err != nil {
		return nil, err
	}
	err = deploymentInformer.AddIndexers(map[string]cache.IndexFunc{
		ByLabel: func(obj interface{}) ([]string, error) {
			s := labels.SelectorFromSet(obj.(*appsv1.Deployment).Spec.Template.Labels).String()
			return []string{s}, nil
		},
	})

	return deploymentInformer, err
}

func (m *Monitor) onServiceEvent(oldService *corev1.Service, service *corev1.Service) {
	if !m.config.RestartPods {
		return
	}
	for _, resource := range m.listServiceDeployments(oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource).(map[string]string)
		if len(mutatedAnnotations) == 0 {
			continue
		}

		data, err := getAnnotationsPatch(resource.Spec.Template.Annotations)

		if err != nil {
			m.logger.Error(err, "Failed to marshal resource")
		}
		deployment, err := m.k8sInterface.AppsV1().Deployments(resource.GetNamespace()).Patch(m.ctx, resource.Name, types.JSONPatchType, data, metav1.PatchOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update deployment", "deployment", resource.Name)
		}
		m.logger.V(1).Info("Updated deployment", "deployment", deployment)
	}
	for _, resource := range m.listServiceStatefulSets(oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource).(map[string]string)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		data, err := getAnnotationsPatch(resource.Spec.Template.Annotations)
		if err != nil {
			m.logger.Error(err, "Failed to marshal resource")
		}

		_, err = m.k8sInterface.AppsV1().StatefulSets(resource.GetNamespace()).Patch(m.ctx, resource.Name, types.JSONPatchType, data, metav1.PatchOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update statefulset", "statefulset", resource.Name)
		}
	}
	for _, resource := range m.listServiceDaemonSets(oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource).(map[string]string)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		data, err := getAnnotationsPatch(resource.Spec.Template.Annotations)
		if err != nil {
			m.logger.Error(err, "Failed to marshal resource")
		}
		_, err = m.k8sInterface.AppsV1().DaemonSets(resource.GetNamespace()).Patch(m.ctx, resource.Name, types.JSONPatchType, data, metav1.PatchOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update daemonset", "daemonset", resource.Name)
		}
	}
}

func getAnnotationsPatch(annotations map[string]string) ([]byte, error) {
	return json.Marshal([]interface{}{
		map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/template/metadata/annotations",
			"value": annotations,
		},
	})
}

func (m *Monitor) listServiceDeployments(services ...*corev1.Service) []appsv1.Deployment {
	var deployments []appsv1.Deployment
	for _, service := range services {
		if service == nil {
			continue
		}
		s := labels.SelectorFromSet(service.Spec.Selector).String()
		informerList, err := m.deploymentInformer.GetIndexer().ByIndex(ByLabel, s)
		if err != nil {
			m.logger.Error(err, "failed to list deployment for service", "service", service.Name)
		}
		for _, obj := range informerList {
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				continue
			}
			deployments = append(deployments, *deployment)
		}
	}
	return deployments
}

func (m *Monitor) listServiceStatefulSets(services ...*corev1.Service) []appsv1.StatefulSet {
	var statefulSets []appsv1.StatefulSet
	for _, service := range services {
		if service == nil {
			continue
		}

		s := labels.SelectorFromSet(service.Spec.Selector).String()
		informerList, err := m.statefulsetInformer.GetIndexer().ByIndex(ByLabel, s)
		if err != nil {
			m.logger.Error(err, "failed to list statefulsets for service", "service", service.Name)
		}

		for _, obj := range informerList {
			statefulSet, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				continue
			}
			statefulSets = append(statefulSets, *statefulSet)
		}
	}
	return statefulSets
}

func (m *Monitor) listServiceDaemonSets(services ...*corev1.Service) []appsv1.DaemonSet {
	var daemonSets []appsv1.DaemonSet
	for _, service := range services {
		if service == nil {
			continue
		}

		s := labels.SelectorFromSet(service.Spec.Selector).String()
		informerList, err := m.daemonsetInformer.GetIndexer().ByIndex(ByLabel, s)
		if err != nil {
			m.logger.Error(err, "failed to list daemonsets for service", "service", service.Name)
		}

		for _, obj := range informerList {
			daemonSet, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				continue
			}
			daemonSets = append(daemonSets, *daemonSet)
		}
	}
	return daemonSets
}

func getTemplateSpecLabels(obj metav1.Object) labels.Set {
	// Check if the object implements the type assertion for PodTemplateSpec
	switch t := obj.(type) {
	case *appsv1.Deployment:
		return t.Spec.Template.Labels
	case *appsv1.StatefulSet:
		return t.Spec.Template.Labels
	case *appsv1.DaemonSet:
		return t.Spec.Template.Labels
	default:
		// Return empty labels.Set if the object type is not supported
		return labels.Set{}
	}
}

// MutateObject adds all enabled languages in config. Should only be run if selected by auto monitor or custom selector
func (m *Monitor) MutateObject(oldObj client.Object, obj client.Object) any {
	if !safeToMutate(oldObj, obj, m.config.RestartPods) {
		return map[string]string{}
	}

	languagesToAnnotate := m.config.CustomSelector.LanguagesOf(obj, false)
	if m.isWorkloadAutoMonitored(obj) {
		for l := range m.config.Languages {
			languagesToAnnotate[l] = nil
		}
	}

	for l := range m.config.Exclude.LanguagesOf(obj, true) {
		delete(languagesToAnnotate, l)
	}

	m.logger.V(2).Info("languages to annotate", "objName", obj.GetName(), "languages", languagesToAnnotate)
	return mutate(obj, languagesToAnnotate)
}

// returns if workload is auto monitored (does not include custom selector)
func (m *Monitor) isWorkloadAutoMonitored(obj client.Object) bool {
	if isNamespace(obj) {
		return false
	}

	if !m.config.MonitorAllServices {
		return false
	}

	if slices.Contains(excludedNamespaces, obj.GetNamespace()) {
		return false
	}
	// determine if the object is currently selected by a service
	objectLabels := getTemplateSpecLabels(obj)
	for _, informerObj := range m.serviceInformer.GetStore().List() {
		service := informerObj.(*corev1.Service)
		if len(service.Spec.Selector) == 0 || service.GetNamespace() != obj.GetNamespace() {
			continue
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)

		if serviceSelector.Matches(objectLabels) {
			m.logger.V(2).Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is owned by service %s", obj.GetName(), m.config.Languages, service.Name))
			return true
		}
	}
	return false
}

// mutate if object is a workload, mutate the pod template. otherwise, mutate the object's annotations itself. It will add annotations if needsInstrumentation is true. Otherwise, it will remove instrumentation annotations.
func mutate(object client.Object, languagesToMonitor instrumentation.TypeSet) map[string]string {
	var obj metav1.Object
	podTemplate := getPodTemplate(object)
	if podTemplate != nil {
		obj = podTemplate
	} else {
		obj = object
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	allMutatedAnnotations := map[string]string{}
	for language := range instrumentation.SupportedTypes {
		insertMutation, removeMutation := buildMutations(language)
		var mutatedAnnotations map[string]string
		if _, ok := languagesToMonitor[language]; ok {
			mutatedAnnotations = insertMutation.Mutate(annotations)
		} else {
			mutatedAnnotations = removeMutation.Mutate(annotations)
		}
		for k, v := range mutatedAnnotations {
			allMutatedAnnotations[k] = v
		}
	}
	obj.SetAnnotations(annotations)
	return allMutatedAnnotations
}

// safeToMutate returns whether the customer consents to the operator updating their workload's pods. The user consents if any of the following conditions are true:
//
// 1. Auto restart enabled.
// 2. The user was already modifying the pod template spec (aka a restart would already be triggered)
func safeToMutate(oldWorkload client.Object, workload client.Object, restartPods bool) bool {
	// always ok to mutate namespace
	if isNamespace(workload) {
		return true
	}
	// should only mutate workloads or namespaces
	if !isMutableType(workload) {
		return false
	}

	if restartPods {
		return true
	}
	oldTemplate, newTemplate := getPodTemplate(oldWorkload), getPodTemplate(workload)
	return !reflect.DeepEqual(oldTemplate, newTemplate)
}

func isMutableType(obj client.Object) bool {
	if isNamespace(obj) {
		return true
	}
	switch obj.(type) {
	case *appsv1.Deployment:
		return true
	case *appsv1.StatefulSet:
		return true
	case *appsv1.DaemonSet:
		return true
	default:
		return false
	}
}

func isNamespace(obj client.Object) bool {
	switch obj.(type) {
	case *corev1.Namespace:
		return true
	}
	return false
}

func getPodTemplate(obj client.Object) *corev1.PodTemplateSpec {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return &o.Spec.Template
	case *appsv1.StatefulSet:
		return &o.Spec.Template
	case *appsv1.DaemonSet:
		return &o.Spec.Template
	default:
		return nil
	}
}
