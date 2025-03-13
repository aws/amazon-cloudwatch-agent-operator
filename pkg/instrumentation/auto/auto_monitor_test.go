package auto

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestMonitor_Selected(t *testing.T) {
	logger.Info("Starting testmonitor tests")

	allTypes := instrumentation.NewTypeSet(instrumentation.SupportedTypes()...)
	tests := []struct {
		name        string
		service     corev1.Service
		oldWorkload client.Object
		workload    client.Object
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
			name:    "Should match when languages are empty",
			service: newTestService("svc-7", map[string]string{"app": "test"}),
			workload: newTestDeployment("deploy-7", map[string]string{
				"app": "test",
			}),
			config: MonitorConfig{
				MonitorAllServices: true,
			},
			shouldMatch: true,
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

			m := NewMonitor(ctx, tt.config, clientSet)
			mutatedAnnotations := m.MutateObject(tt.workload, tt.workload)

			assert.Equal(t, tt.shouldMatch, mutatedAnnotations,
				"Expected workload matching to be %v but got %v", tt.shouldMatch, mutatedAnnotations)
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
		},
		Spec: appsv1.DeploymentSpec{
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
		},
		Spec: appsv1.DaemonSetSpec{
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
		},
		Spec: appsv1.StatefulSetSpec{
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

func TestUnmarshal(t *testing.T) {
	j := []byte(`["java", "nodejs", "python"]`)
	set := instrumentation.TypeSet{}
	err := set.UnmarshalJSON(j)
	assert.NoError(t, err)
	assert.Equal(t, instrumentation.TypeSet{instrumentation.TypeNodeJS: nil, instrumentation.TypeJava: nil, instrumentation.TypePython: nil}, set)
}

func TestMarshal(t *testing.T) {
	types := instrumentation.TypeSet{instrumentation.TypeNodeJS: nil, instrumentation.TypePython: nil}
	res, err := types.MarshalJSON()
	assert.NoError(t, err)
	AssertJsonEqual(t, []byte(`["nodejs","python"]`), res)
}

func AssertJsonEqual(t *testing.T, expectedJson []byte, actualJson []byte) {
	var obj1, obj2 interface{}

	err := json.Unmarshal(expectedJson, &obj1)
	assert.NoError(t, err)

	err = json.Unmarshal(actualJson, &obj2)
	assert.NoError(t, err)

	assert.Equal(t, obj1, obj2)
}

func Test_isWorkloadPodTemplateMutated(t *testing.T) {
	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "nginx:1.14.2"}},
				},
			},
		},
	}
	tests := []struct {
		name      string
		oldObject client.Object
		object    client.Object
		want      bool
	}{
		{"nil objects", nil, nil, true},
		{"identical deployments", deploy.DeepCopy(), deploy.DeepCopy(), false},
		{"changed pod template", deploy.DeepCopy(), &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Image: "nginx:1.15.0"}},
					},
				},
			},
		}, true},
		{"non-workload", &corev1.ConfigMap{}, &corev1.ConfigMap{}, true},
		{"create (oldObject nil)", nil, deploy.DeepCopy(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isWorkloadPodTemplateMutated(tt.oldObject, tt.object))
		})
	}
}

func Test_getPodTemplate(t *testing.T) {
	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Image: "nginx"}},
		},
	}

	tests := []struct {
		name string
		obj  client.Object
		want *corev1.PodTemplateSpec
	}{
		{"deployment", &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: template}}, &template},
		{"statefulset", &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: template}}, &template},
		{"daemonset", &appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: template}}, &template},
		{"other", &corev1.Pod{}, nil},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, getPodTemplate(tt.obj))
		})
	}
}

func Test_mutate(t *testing.T) {
	tests := []struct {
		name               string
		obj                client.Object
		languagesToMonitor instrumentation.TypeSet
		wantObjAnnotations map[string]string
		wantMutated        map[string]string
	}{
		{
			name: "deployment - java only",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("java"),
		},
		{
			name: "deployment - java and python",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{
				"java":   struct{}{},
				"python": struct{}{},
			},
			wantObjAnnotations: mergeMaps(buildAnnotations("java"), buildAnnotations("python")),
			wantMutated:        mergeMaps(buildAnnotations("java"), buildAnnotations("python")),
		},
		{
			name: "remove python instrumentation",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: buildAnnotations("python"),
						},
					},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("python"),
		},
		{
			name: "remove one of two languages",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: mergeMaps(buildAnnotations("python"), buildAnnotations("java")),
						},
					},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("python"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMutated := mutate(tt.obj, tt.languagesToMonitor)
			assert.Equal(t, tt.wantObjAnnotations, tt.obj.GetAnnotations())
			assert.Equal(t, tt.wantMutated, gotMutated)
		})
	}
}
func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
