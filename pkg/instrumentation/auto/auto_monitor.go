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
	"k8s.io/utils/strings/slices"
	"os"
	"time"
)

type Monitor struct {
	serviceInformer cache.SharedIndexInformer
	ctx             context.Context
	logger          logr.Logger
	config          MonitorConfig
}

func NewMonitor(ctx context.Context, logger logr.Logger, config MonitorConfig, k8sInterface kubernetes.Interface) *Monitor {

	factory := informers.NewSharedInformerFactory(k8sInterface, 10*time.Minute)

	serviceInformer := factory.Core().V1().Services().Informer()
	factory.Start(ctx.Done()) // runs in background
	synced := factory.WaitForCacheSync(ctx.Done())
	for v, ok := range synced {
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "caches failed to sync: %v", v)
			panic("TODO: handle bad cache sync")
		}
	}

	return &Monitor{serviceInformer: serviceInformer, ctx: ctx, logger: logger, config: config}
}

// ShouldBeMonitored returns whether obj is selected by either custom selector or auto monitor
func (m Monitor) ShouldBeMonitored(obj metav1.Object) bool {
	// TODO: check if in custom selector
	// note: custom selector does not respect MonitorAllServices
	if m.customSelected(obj) {
		return true
	}

	if !m.config.MonitorAllServices {
		return false
	}

	objectLabels := labels.Set(obj.GetLabels())
	// if object is not workload, return err
	for _, informerObj := range m.serviceInformer.GetStore().List() {
		service := informerObj.(*corev1.Service)
		serviceSelector := labels.SelectorFromSet(service.Spec.Selector)

		m.logger.V(2).Info("AutoMonitor: testing serviceSelector", "serviceSelector", serviceSelector.String(), "objectLabels", objectLabels.String())
		if serviceSelector.Matches(objectLabels) {
			m.logger.V(2).Info("AutoMonitor: matched!", "service", service, "object", obj.GetName())
			return true
		}

		// remove if none matched, not in custom selector
	}
	return false
}

// Mutate adds all enabled languages in config. Should only be run if selected by auto monitor or custom selector
func (m Monitor) Mutate(obj metav1.Object) map[string]string {
	// TODO: only create if automonitor enabled
	annotations := obj.GetAnnotations()
	allMutatedAnnotations := make(map[string]string)
	for _, language := range instrumentation.AllTypes() {
		insertMutation, removeMutation := buildMutations(language)
		var mutatedAnnotations map[string]string
		if _, ok := m.config.Languages[language]; ok {
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

func (m Monitor) customSelected(obj metav1.Object) bool {
	objName := namespacedName(obj)
	var resourceList []string

	switch obj.(type) {
	case *appsv1.Deployment:
		for _, t := range instrumentation.AllTypes() {
			resourceList = append(resourceList, m.config.CustomSelector.getResources(t).Deployments...)
		}
	case *appsv1.StatefulSet:
		for _, t := range instrumentation.AllTypes() {
			resourceList = append(resourceList, m.config.CustomSelector.getResources(t).StatefulSets...)
		}
	case *appsv1.DaemonSet:
		for _, t := range instrumentation.AllTypes() {
			resourceList = append(resourceList, m.config.CustomSelector.getResources(t).DaemonSets...)
		}
	}

	return slices.Contains(resourceList, objName)
}
