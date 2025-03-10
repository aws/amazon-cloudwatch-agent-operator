package auto

import (
	"context"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
)

var logger = logf.Log.WithName("auto_monitor_tests")

func TestMonitor_Selected(t *testing.T) {
	logger.Info("Starting testmonitor tests")

	allTypes := instrumentation.NewTypeSet(instrumentation.AllTypes()...)
	tests := []struct {
		name        string
		service     corev1.Service
		workload    metav1.Object
		config      MonitorConfig
		shouldMatch bool
	}{
		{
			name:    "Should match Deployment with exact label match",
			service: newTestService("svc-1", map[string]string{"app": "test"}),
			workload: newTestDeployment("deploy-1", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: true,
		},
		{
			name:     "Should not match Deployment with no labels",
			service:  newTestService("svc-2", map[string]string{"app": "test"}),
			workload: newTestDeployment("deploy-2", map[string]string{}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: false,
		},
		{
			name:    "Should not match when MonitorAllServices is false",
			service: newTestService("svc-3", map[string]string{"app": "test"}),
			workload: newTestDeployment("deploy-3", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
			},
			shouldMatch: false,
		},
		{
			name:    "Should match Deployment with multiple matching labels",
			service: newTestService("svc-4", map[string]string{"app": "test", "env": "prod"}),
			workload: newTestDeployment("deploy-4", map[string]string{
				"app":   "test",
				"env":   "prod",
				"extra": "label",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: true,
		},
		{
			name:    "Should not match when service selector is empty",
			service: newTestService("svc-5", map[string]string{}),
			workload: newTestDeployment("deploy-5", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: false,
		},
		{
			name:    "Should match StatefulSet with partial label match",
			service: newTestService("svc-6", map[string]string{"app": "test"}),
			workload: newTestStatefulSet("sts-1", map[string]string{
				"app":   "test",
				"other": "value",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: true,
		},
		{
			name:    "Should not match when languages are empty",
			service: newTestService("svc-7", map[string]string{"app": "test"}),
			workload: newTestDeployment("deploy-7", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
			},
			shouldMatch: false,
		},
		{
			name:    "Should match DaemonSet with exact label match",
			service: newTestService("svc-8", map[string]string{"app": "test"}),
			workload: newTestDaemonSet("ds-1", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: true,
		},
		{
			name:    "Should not match DaemonSet with mismatched labels",
			service: newTestService("svc-9", map[string]string{"app": "test"}),
			workload: newTestDaemonSet("ds-2", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: false,
		},
		{
			name:    "Should match StatefulSet with exact label match",
			service: newTestService("svc-10", map[string]string{"app": "test"}),
			workload: newTestStatefulSet("sts-2", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: true,
		},
		{
			name:    "Should not match StatefulSet with mismatched labels",
			service: newTestService("svc-11", map[string]string{"app": "test"}),
			workload: newTestStatefulSet("sts-3", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
				Languages:          allTypes,
			},
			shouldMatch: false,
		},
		{
			name:    "Should match Deployment in custom selector regardless of MonitorAllServices",
			service: newTestService("svc-12", map[string]string{"app": "different"}),
			workload: newTestDeployment("custom-deploy-1", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					Java: AnnotationResources{
						Deployments: []string{"default/custom-deploy-1"},
					},
				},
			},
			shouldMatch: true,
		},
		{
			name:    "Should match StatefulSet in custom selector",
			service: newTestService("svc-13", map[string]string{"app": "different"}),
			workload: newTestStatefulSet("custom-sts-1", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					Python: AnnotationResources{
						StatefulSets: []string{"default/custom-sts-1"},
					},
				},
			},
			shouldMatch: true,
		},
		{
			name:    "Should match DaemonSet in custom selector",
			service: newTestService("svc-14", map[string]string{"app": "different"}),
			workload: newTestDaemonSet("custom-ds-1", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					NodeJS: AnnotationResources{
						DaemonSets: []string{"default/custom-ds-1"},
					},
				},
			},
			shouldMatch: true,
		},
		{
			name:    "Should not match when workload not in custom selector",
			service: newTestService("svc-15", map[string]string{"app": "different"}),
			workload: newTestDeployment("non-custom-deploy", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					Java: AnnotationResources{
						Deployments: []string{"default/different-deploy"},
					},
				},
			},
			shouldMatch: false,
		},
		{
			name:    "Should match when workload in custom selector for multiple languages",
			service: newTestService("svc-16", map[string]string{"app": "different"}),
			workload: newTestDeployment("multi-lang-deploy", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					Java: AnnotationResources{
						Deployments: []string{"default/multi-lang-deploy"},
					},
					Python: AnnotationResources{
						Deployments: []string{"default/multi-lang-deploy"},
					},
				},
			},
			shouldMatch: true,
		},
		{
			name:    "Should match when workload in custom selector with multiple resource types",
			service: newTestService("svc-17", map[string]string{"app": "different"}),
			workload: newTestDeployment("mixed-resources-deploy", map[string]string{
				"app": "different",
			}),
			config: MonitorConfig{
				MonitorAllServices: false,
				Languages:          allTypes,
				CustomSelector: AnnotationConfig{
					DotNet: AnnotationResources{
						Deployments:  []string{"default/mixed-resources-deploy"},
						DaemonSets:   []string{"default/some-ds"},
						StatefulSets: []string{"default/some-sts"},
					},
				},
			},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clientSet := fake.NewSimpleClientset()

			// Create service
			_, err := clientSet.CoreV1().Services("default").Create(ctx, &tt.service, metav1.CreateOptions{})
			require.NoError(t, err)

			// Create workload
			err = createWorkload(ctx, clientSet, tt.workload)
			require.NoError(t, err)

			m := NewMonitor(ctx, logger, tt.config, clientSet)
			matched := m.ShouldBeMonitored(tt.workload)

			assert.Equal(t, tt.shouldMatch, matched,
				"Expected workload matching to be %v but got %v", tt.shouldMatch, matched)
		})
	}
}

// Helper functions to create test resources
func newTestService(name string, selector map[string]string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
		},
	}
}

func newTestDeployment(name string, labels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func newTestDaemonSet(name string, labels map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func newTestStatefulSet(name string, labels map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func createWorkload(ctx context.Context, clientSet *fake.Clientset, workload metav1.Object) error {
	switch x := workload.(type) {
	case *appsv1.Deployment:
		_, err := clientSet.AppsV1().Deployments(x.GetNamespace()).Create(ctx, x, metav1.CreateOptions{})
		return err
	case *appsv1.DaemonSet:
		_, err := clientSet.AppsV1().DaemonSets(x.GetNamespace()).Create(ctx, x, metav1.CreateOptions{})
		return err
	case *appsv1.StatefulSet:
		_, err := clientSet.AppsV1().StatefulSets(x.GetNamespace()).Create(ctx, x, metav1.CreateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported workload type: %T", workload)
	}
}
