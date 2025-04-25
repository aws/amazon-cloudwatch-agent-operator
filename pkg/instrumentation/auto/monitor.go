package auto

import (
	"context"
	"errors"
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
	"time"
)

// InstrumentationAnnotator is the highest level abstraction used to annotate kubernetes resources for instrumentation
type InstrumentationAnnotator interface {
	MutateObject(oldObj client.Object, obj client.Object) map[string]string
	GetAnnotationMutators() *AnnotationMutators
	GetLogger() logr.Logger
	GetReader() client.Reader
	GetWriter() client.Writer
	MutateAndPatchAll(ctx context.Context)
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

func (m *Monitor) MutateAndPatchAll(ctx context.Context) {
	if m.config.RestartPods {
		MutateAndPatchAll(m, ctx, false)
	}
	// todo: what to do about updating namespace annotations? maybe update them here? or pass in variable to MutateAndPatchAll?
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

// NewMonitor is used to create an InstrumentationMutator that supports AutoMonitor.
func NewMonitor(ctx context.Context, config MonitorConfig, k8sInterface kubernetes.Interface, w client.Writer, r client.Reader, logger logr.Logger) *Monitor {
	// Config default values
	if len(config.Languages) == 0 {
		logger.Info("Setting languages to default")
		config.Languages = instrumentation.SupportedTypes()
	}

	logger.Info("AutoMonitor starting...")
	factory := informers.NewSharedInformerFactoryWithOptions(k8sInterface, 10*time.Minute, informers.WithTransform(func(obj interface{}) (interface{}, error) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return obj, errors.New(fmt.Sprintf("error transforming service: %s not a service", obj))
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
	}))
	serviceInformer := factory.Core().V1().Services().Informer()

	mutator := NewAnnotationMutators(w, r, logger, config.CustomSelector, instrumentation.SupportedTypes())

	m := &Monitor{serviceInformer: serviceInformer, ctx: ctx, config: config, k8sInterface: k8sInterface, customSelectors: mutator, clientReader: r, clientWriter: w}
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

	logger.Info("Initialization complete!")
	return m
}

func (m *Monitor) onServiceEvent(oldService *corev1.Service, service *corev1.Service) {
	if !m.config.RestartPods {
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
	if !safeToMutate(oldObj, obj, m.config.RestartPods) {
		return map[string]string{}
	}

	languagesToAnnotate := m.customSelectors.cfg.LanguagesOf(obj, false)
	if m.isWorkloadAutoMonitored(obj) {
		for l := range m.config.Languages {
			languagesToAnnotate[l] = nil
		}
	}

	for l := range m.config.Exclude.LanguagesOf(obj, true) {
		delete(languagesToAnnotate, l)
	}

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

	// determine if the object is currently selected by a service
	objectLabels := getTemplateSpecLabels(obj)
	for _, informerObj := range m.serviceInformer.GetStore().List() {
		service := informerObj.(*corev1.Service)
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
	for language := range instrumentation.SupportedTypes() {
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
// 2. MonitorAllServices is enabled AND the workload is already going to restart (aka, the pod template is already modified)
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
