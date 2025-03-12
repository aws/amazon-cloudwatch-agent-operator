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
}

type NoopMonitor struct{}

func (n NoopMonitor) MutateObject(oldObj client.Object, obj client.Object) map[string]string {
	return map[string]string{}
}

func (n NoopMonitor) AnyCustomSelectorDefined() bool {
	return false
}

func NewMonitor(ctx context.Context, config MonitorConfig, k8sInterface kubernetes.Interface) *Monitor {
	logger.Info("AutoMonitor starting...")

	factory := informers.NewSharedInformerFactory(k8sInterface, 10*time.Minute)

	serviceInformer := factory.Core().V1().Services().Informer()
	m := &Monitor{serviceInformer: serviceInformer, ctx: ctx, config: config}

	_, err := serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		if config.AutoRestart {
			list, err := k8sInterface.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				logger.Error(err, "failed to list namespaces")
			}

			// TODO: optimize this by trying to resolve workloads via endpoint slices
			for _, namespace := range list.Items {
				for _, deployment := range listAllDeployments(k8sInterface, namespace, ctx).Items {
					m.MutateObject(nil, &deployment)
				}
				for _, deployment := range listAllStatefulSets(k8sInterface, namespace, ctx).Items {
					m.MutateObject(nil, &deployment)
				}
				for _, deployment := range listAllDaemonSets(k8sInterface, namespace, ctx).Items {
					m.MutateObject(nil, &deployment)
				}
				m.MutateObject(nil, &namespace)
			}
		}
	}})
	if err != nil {
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
	logger.Info("AutoMonitor enabled!")

	return m
}

func listAllDeployments(k8sInterface kubernetes.Interface, namespace corev1.Namespace, ctx context.Context) *appsv1.DeploymentList {
	list, err := k8sInterface.AppsV1().Deployments(namespace.Name).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "AutoMonitor failed to list deployments")
	}
	return list
}

func listAllStatefulSets(k8sInterface kubernetes.Interface, namespace corev1.Namespace, ctx context.Context) *appsv1.StatefulSetList {
	list, err := k8sInterface.AppsV1().StatefulSets(namespace.Name).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "AutoMonitor failed to list statefulsets")
	}
	return list
}

func listAllDaemonSets(k8sInterface kubernetes.Interface, namespace corev1.Namespace, ctx context.Context) *appsv1.DaemonSetList {
	list, err := k8sInterface.AppsV1().DaemonSets(namespace.Name).List(ctx, metav1.ListOptions{})
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

// MutateObject adds all enabled languages in config. Should only be run if selected by auto monitor or custom selector
func (m Monitor) MutateObject(oldObj client.Object, obj client.Object) map[string]string {
	// todo: handle edge case where a workload is annotated because a service exposed it, and the service is removed. aka add to Service OnDelete
	// continue only if restart is enabled or if workload pod template has been mutated
	if !m.config.AutoRestart && isWorkload(obj) && !isWorkloadPodTemplateMutated(oldObj, obj) {
		return nil
	}
	// custom selector takes precedence over service selector
	if customSelectLanguages, selected := m.CustomSelected(obj); selected {
		logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is specified in custom selector", obj.GetName(), customSelectLanguages))
		mutate(obj, customSelectLanguages)
	}

	if !m.config.MonitorAllServices {
		return nil
	}

	if m.excluded(obj) {
		return nil
	}

	objectLabels := getTemplateSpecLabels(obj)
	for _, informerObj := range m.serviceInformer.GetStore().List() {
		service := informerObj.(*corev1.Service)
		if service.Spec.Selector == nil || len(service.Spec.Selector) == 0 {
			continue
		}
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)

		if serviceSelector.Matches(objectLabels) {
			logger.Info(fmt.Sprintf("setting %s instrumentation annotations to %s because it is owned by service %s", obj.GetName(), m.config.Languages, service.Name))
			return mutate(obj, m.config.Languages)
		}
	}
	return nil
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

func (m Monitor) CustomSelected(obj client.Object) (instrumentation.TypeSet, bool) {
	languages := m.config.CustomSelector.GetObjectLanguagesToAnnotate(obj)
	return languages, len(languages) > 0
}

// excluded returns whether a Namespace or a Service is excluded from AutoMonitor.
func (m Monitor) excluded(obj client.Object) bool {
	switch obj.GetObjectKind().GroupVersionKind().Kind {
	case "Namespace":
		return slices.Contains(m.config.Exclude.Namespaces, obj.GetName())
	case "Service":
		return slices.Contains(m.config.Exclude.Services, namespacedName(obj))
	}
	return false
}

func (m Monitor) AnyCustomSelectorDefined() bool {
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
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	return kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet"
}

// isWorkloadPodTemplateMutated
func isWorkloadPodTemplateMutated(oldObject client.Object, object client.Object) bool {
	oldTemplate, newTemplate := getPodTemplate(oldObject), getPodTemplate(object)
	if oldTemplate == nil || newTemplate == nil {
		return true
	}
	return !reflect.DeepEqual(oldTemplate, newTemplate)
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
