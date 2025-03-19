package auto

import (
	"context"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

var logger = logf.Log.WithName("auto_monitor")

type MonitorInterface interface {
	MutateObject(oldObj client.Object, obj client.Object) map[string]string
	AnyCustomSelectorDefined() bool
}

type Monitor struct {
	serviceInformer cache.SharedIndexInformer
	ctx             context.Context
	config          MonitorConfig
	k8sInterface    kubernetes.Interface
	customSelectors *AnnotationMutators
}

type NoopMonitor struct{}

func (n NoopMonitor) MutateObject(_ client.Object, _ client.Object) map[string]string {
	return map[string]string{}
}

func (n NoopMonitor) AnyCustomSelectorDefined() bool {
	return false
}

func NewMonitor(ctx context.Context, config MonitorConfig, k8sInterface kubernetes.Interface, c client.Client, r client.Reader) *Monitor {
	logger.Info("AutoMonitor starting...")
	// todo, throw warning if exclude config service is not namespaced (doesn't contain `/`)
	// todo: informers.WithTransform() as option to only store what parts of service are needed
	factory := informers.NewSharedInformerFactoryWithOptions(k8sInterface, 10*time.Minute)
	serviceInformer := factory.Core().V1().Services().Informer()

	m := &Monitor{serviceInformer: serviceInformer, ctx: ctx, config: config, k8sInterface: k8sInterface, customSelectors: NewAnnotationMutators(c, r, logger, config.CustomSelector, config.Languages)}
	_, err := serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		if serviceInformer.HasSynced() {
			m.onServiceAdd(obj)
		} else {
			logger.Info(fmt.Sprintf("Service %v has not synced yet, this is first sync. skipping annotation", obj))
		}
	}})
	if err != nil {
		logger.Error(err, "failed to start auto monitor")
		return nil
	}
	factory.Start(ctx.Done())
	synced := factory.WaitForCacheSync(ctx.Done())
	for v, ok := range synced {
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "caches failed to sync: %v", v)
			// TODO: handle bad cache sync
			panic("TODO: handle bad cache sync")
		}
	}
	logger.Info("Enabled!")

	if m.config.AutoRestart {
		logger.Info("Auto restarting custom selector resources")
		m.customSelectors.MutateAndPatchAll(ctx)
		// update all existing services
		logger.Info("Auto restarting service resources, except for excludedServices or services in excludedNamespaces", "excludedServices", m.config.Exclude.Services, "excludedNamespaces", m.config.Exclude.Namespaces)
		list, err := k8sInterface.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil
		}

		for _, service := range list.Items {
			m.onServiceAdd(service)
		}
	} else {
		logger.Info("Auto restart disabled. To instrument workloads, restart the workloads exposed by a service.")
	}
	logger.Info("Initialization complete!")
	return m
}

func (m *Monitor) onServiceAdd(obj interface{}) {
	service := obj.(*corev1.Service)
	if service.Spec.Selector == nil || len(service.Spec.Selector) == 0 {
		return
	}

	// we should not execute this code on start up because it needs to iterate over all services in MutateObject,
	if !m.config.AutoRestart || m.excludedService(service) {
		return
	}
	namespace := service.GetNamespace()
	for _, resource := range listServiceDeployments(m.k8sInterface, service, m.ctx).Items {
		mutatedAnnotations := m.MutateServiceWorkload(&resource, service)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().Deployments(namespace).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update deployment")
		}
	}
	for _, resource := range listServiceStatefulSets(m.k8sInterface, service, m.ctx).Items {
		mutatedAnnotations := m.MutateServiceWorkload(&resource, service)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().StatefulSets(namespace).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update statefulset")
		}
	}
	for _, resource := range listServiceDaemonSets(m.k8sInterface, service, m.ctx).Items {
		mutatedAnnotations := m.MutateServiceWorkload(&resource, service)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().DaemonSets(namespace).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update daemonset")
		}
	}
}

func listServiceDeployments(k8sInterface kubernetes.Interface, service *corev1.Service, ctx context.Context) *appsv1.DeploymentList {
	list, err := k8sInterface.AppsV1().Deployments(service.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(service.Spec.Selector),
	})
	if err != nil {
		logger.Error(err, "AutoMonitor failed to list deployments")
	}
	return list
}

func listServiceStatefulSets(k8sInterface kubernetes.Interface, service *corev1.Service, ctx context.Context) *appsv1.StatefulSetList {
	list, err := k8sInterface.AppsV1().StatefulSets(service.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(service.Spec.Selector),
	})
	if err != nil {
		logger.Error(err, "AutoMonitor failed to list statefulsets")
	}
	return list
}

func listServiceDaemonSets(k8sInterface kubernetes.Interface, service *corev1.Service, ctx context.Context) *appsv1.DaemonSetList {
	list, err := k8sInterface.AppsV1().DaemonSets(service.GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: labels.FormatLabels(service.Spec.Selector),
	})
	if err != nil {
		logger.Error(err, "AutoMonitor failed to list DaemonSets")
	}
	return list
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
	case *appsv1.ReplicaSet:
		return t.Spec.Template.Labels
	default:
		// Return empty labels.Set if the object type is not supported
		return labels.Set{}
	}
}

// MutateServiceWorkload mutates the annotations of the workload's object without iterating over
// TODO: remove?
func (m *Monitor) MutateServiceWorkload(obj client.Object, service *corev1.Service) map[string]string {
	if customSelectLanguages, selected := m.CustomSelected(obj); selected {
		logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is specified in custom selector", obj.GetName(), customSelectLanguages))
		return mutate(obj, customSelectLanguages)
	}

	if !m.config.MonitorAllServices {
		return map[string]string{}
	}
	if m.excludedNamespace(obj.GetNamespace()) {
		return map[string]string{}
	}

	if m.excludedService(service) {
		return map[string]string{}
	}

	logger.Info(fmt.Sprintf("start up: setting %s instrumentation annotations to %s because it is owned by service %s", obj.GetName(), m.config.Languages, service.Name))
	return mutate(obj, m.config.Languages)
}

// MutateObject adds all enabled languages in config. Should only be run if selected by auto monitor or custom selector
func (m *Monitor) MutateObject(oldObj client.Object, obj client.Object) map[string]string {
	// todo: handle edge case where a workload is annotated because a service exposed it, and the service is removed. aka add to Service OnDelete
	// custom selector takes precedence over service selector
	if customSelectLanguages, selected := m.CustomSelected(obj); selected {
		logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is specified in custom selector", obj.GetName(), customSelectLanguages))
		return mutate(obj, customSelectLanguages)
	}

	if !allowedToMutate(oldObj, obj, m.config.AutoRestart) {
		return map[string]string{}
	}

	if !m.config.MonitorAllServices {
		return map[string]string{}
	}

	if m.excludedNamespace(obj.GetNamespace()) {
		return map[string]string{}
	}

	objectLabels := getTemplateSpecLabels(obj)
	for _, informerObj := range m.serviceInformer.GetStore().List() {
		service := informerObj.(*corev1.Service)
		if m.excludedService(service) {
			continue
		}
		if service.Spec.Selector == nil || len(service.Spec.Selector) == 0 || service.GetNamespace() != obj.GetNamespace() {
			continue
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)

		if serviceSelector.Matches(objectLabels) {
			logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is owned by service %s", obj.GetName(), m.config.Languages, service.Name))
			return mutate(obj, m.config.Languages)
		}
	}
	return map[string]string{}
}

// mutate obj. If object is a workload, mutate the pod template. otherwise, mutate the object's annotations itself.
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
		annotations = make(map[string]string)
	}

	allMutatedAnnotations := make(map[string]string)
	for _, language := range instrumentation.SupportedTypes() {
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

func (m *Monitor) CustomSelected(obj client.Object) (instrumentation.TypeSet, bool) {
	languages := m.config.CustomSelector.GetObjectLanguagesToAnnotate(obj)
	return languages, len(languages) > 0
}

// excludedService returns whether a Namespace or a Service is excludedService from AutoMonitor.
func (m *Monitor) excludedService(obj client.Object) bool {
	excluded := slices.Contains(m.config.Exclude.Services, namespacedName(obj)) || m.excludedNamespace(obj.GetNamespace())
	logger.Info(fmt.Sprintf("%s excluded? %v", namespacedName(obj), excluded))
	return excluded
}

func (m *Monitor) excludedNamespace(namespace string) bool {
	if strings.HasPrefix(namespace, "kube-") {
		return false
	}
	if strings.EqualFold(namespace, "amazon-cloudwatch") {
		return false
	}
	return slices.Contains(m.config.Exclude.Namespaces, namespace)
}

func (m *Monitor) AnyCustomSelectorDefined() bool {
	for _, t := range instrumentation.SupportedTypes() {
		resources := m.config.CustomSelector.getResources(t)
		if len(resources.DaemonSets) > 0 {
			return true
		}
		if len(resources.StatefulSets) > 0 {
			return true
		}
		if len(resources.Deployments) > 0 {
			return true
		}
		if len(resources.Namespaces) > 0 {
			return true
		}
	}
	return false
}

func isWorkload(obj client.Object) bool {
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

// allowedToMutate returns if object is already being mutated or if auto restart is enabled. does not guarantee that the object will be mutated.
func allowedToMutate(oldObject client.Object, object client.Object, autoRestart bool) bool {
	// mutating a namespace is always safe
	if isNamespace(object) {
		return true
	}
	// should only mutate workloads or namespaces
	if !isWorkload(object) {
		return false
	}

	if autoRestart {
		return true
	}
	oldTemplate, newTemplate := getPodTemplate(oldObject), getPodTemplate(object)
	return !reflect.DeepEqual(oldTemplate, newTemplate)
}

func isNamespace(object client.Object) bool {
	switch object.(type) {
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
