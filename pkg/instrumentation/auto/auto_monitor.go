package auto

import (
	"context"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
	"strings"
	"time"
)

type MonitorInterface interface {
	MutateObject(oldObj client.Object, obj client.Object) map[string]string
	GetAnnotationMutators() *AnnotationMutators
	GetLogger() logr.Logger
	GetReader() client.Reader
	GetWriter() client.Writer
}

type Monitor struct {
	serviceInformer cache.SharedIndexInformer
	ctx             context.Context
	config          MonitorConfig
	k8sInterface    kubernetes.Interface
	clientReader    client.Reader
	clientWriter    client.Writer
	customSelectors *AnnotationMutators
	logger          logr.Logger
}

func (m *Monitor) GetAnnotationMutators() *AnnotationMutators {
	return m.customSelectors
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

type NoopMonitor struct {
	customSelectors *AnnotationMutators
}

func (n NoopMonitor) MutateObject(_ client.Object, _ client.Object) map[string]string {
	return map[string]string{}
}

func (n NoopMonitor) AnyCustomSelectorDefined() bool {
	return false
}

func NewMonitorWithExistingAnnotationMutator(ctx context.Context, config MonitorConfig, k8sInterface kubernetes.Interface, w client.Writer, r client.Reader, logger logr.Logger, mutators *AnnotationMutators) *Monitor {
	logger.Info("AutoMonitor starting...")
	// todo, throw warning if exclude config service is not namespaced (doesn't contain `/`)
	// todo: informers.WithTransform() as option to only store what parts of service are needed
	factory := informers.NewSharedInformerFactoryWithOptions(k8sInterface, 10*time.Minute)
	serviceInformer := factory.Core().V1().Services().Informer()

	m := &Monitor{serviceInformer: serviceInformer, ctx: ctx, config: config, k8sInterface: k8sInterface, customSelectors: mutators, clientReader: r, clientWriter: w}
	_, err := serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
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
		MutateAndPatchAll(m, ctx)
	} else {
		logger.Info("Auto restart disabled. To instrument workloads, restart the workloads exposed by a service.")
	}
	logger.Info("Initialization complete!")
	return m
}

func NewMonitor(ctx context.Context, config MonitorConfig, k8sInterface kubernetes.Interface, w client.Writer, r client.Reader, logger logr.Logger) *Monitor {
	return NewMonitorWithExistingAnnotationMutator(ctx, config, k8sInterface, w, r, logger, NewAnnotationMutators(w, r, logger, config.CustomSelector, instrumentation.NewTypeSet(instrumentation.SupportedTypes()...)))
}

func (m *Monitor) onServiceEvent(oldService *corev1.Service, service *corev1.Service) {
	if !m.config.AutoRestart {
		return
	}
	for _, resource := range m.listServiceDeployments(m.ctx, oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().Deployments(resource.GetNamespace()).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update deployment")
		}
	}
	for _, resource := range m.listServiceStatefulSets(m.ctx, oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().StatefulSets(resource.GetNamespace()).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update statefulset")
		}
	}
	for _, resource := range m.listServiceDaemonSets(m.ctx, oldService, service) {
		mutatedAnnotations := m.MutateObject(&resource, &resource)
		if len(mutatedAnnotations) == 0 {
			continue
		}
		_, err := m.k8sInterface.AppsV1().DaemonSets(resource.GetNamespace()).Update(m.ctx, &resource, metav1.UpdateOptions{})
		if err != nil {
			m.logger.Error(err, "failed to update daemonset")
		}
	}
}

func (m *Monitor) listServiceDeployments(ctx context.Context, services ...*corev1.Service) []appsv1.Deployment {
	var deployments []appsv1.Deployment
	for _, service := range services {
		if service == nil {
			continue
		}
		list, err := m.k8sInterface.AppsV1().Deployments(service.GetNamespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.logger.Error(err, "failed to list deployments")
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)
		trimmed := slices.DeleteFunc(list.Items, func(deployment appsv1.Deployment) bool {
			return !serviceSelector.Matches(getTemplateSpecLabels(&deployment))
		})
		deployments = append(deployments, trimmed...)
	}
	return deployments
}

func (m *Monitor) listServiceStatefulSets(ctx context.Context, services ...*corev1.Service) []appsv1.StatefulSet {
	var statefulSets []appsv1.StatefulSet
	for _, service := range services {
		if service == nil {
			continue
		}
		list, err := m.k8sInterface.AppsV1().StatefulSets(service.GetNamespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.logger.Error(err, "failed to list statefulsets")
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)
		trimmed := slices.DeleteFunc(list.Items, func(statefulSet appsv1.StatefulSet) bool {
			return !serviceSelector.Matches(getTemplateSpecLabels(&statefulSet))
		})
		statefulSets = append(statefulSets, trimmed...)
	}
	return statefulSets
}

func (m *Monitor) listServiceDaemonSets(ctx context.Context, services ...*corev1.Service) []appsv1.DaemonSet {
	var daemonSets []appsv1.DaemonSet
	for _, service := range services {
		if service == nil {
			continue
		}
		list, err := m.k8sInterface.AppsV1().DaemonSets(service.GetNamespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.logger.Error(err, "failed to list DaemonSets")
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)
		trimmed := slices.DeleteFunc(list.Items, func(daemonSet appsv1.DaemonSet) bool {
			return !serviceSelector.Matches(getTemplateSpecLabels(&daemonSet))
		})
		daemonSets = append(daemonSets, trimmed...)
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
func (m *Monitor) MutateObject(oldObj, obj client.Object) map[string]string {
	if !safeToMutate(oldObj, obj, m.config.AutoRestart) {
		return map[string]string{}
	}

	// todo: handle edge case where a workload is annotated because a service exposed it, and the service is removed. aka add to Service OnDelete
	// custom selector takes precedence over service selector
	if customSelectLanguages, selected := m.CustomSelected(obj); selected {
		m.logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is specified in custom selector", obj.GetName(), customSelectLanguages))
		return mutate(obj, customSelectLanguages, true)
	}

	return mutate(obj, m.config.Languages, m.shouldInsert(obj))
}

func (m *Monitor) shouldInsert(obj client.Object) bool {
	if !m.config.MonitorAllServices {
		return false
	}

	if m.excludedNamespace(obj.GetNamespace()) {
		return false
	}

	// determine if the object is currently selected by a service
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
			m.logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is owned by service %s", obj.GetName(), m.config.Languages, service.Name))
			return true
		}
	}
	return false
}

// mutate obj. If object is a workload, mutate the pod template. otherwise, mutate the object's annotations itself.
func mutate(object client.Object, languagesToMonitor instrumentation.TypeSet, shouldInsert bool) map[string]string {
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
	for _, language := range instrumentation.SupportedTypes() {
		insertMutation, removeMutation := buildMutations(language)
		var mutatedAnnotations map[string]string
		if _, ok := languagesToMonitor[language]; ok && shouldInsert {
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
	m.logger.Info(fmt.Sprintf("%s excluded? %v", namespacedName(obj), excluded))
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

// safeToMutate returns if object is already being mutated or if auto restart is enabled. does not guarantee that the object will be mutated.
func safeToMutate(oldObject client.Object, object client.Object, autoRestart bool) bool {
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
