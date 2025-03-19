package auto

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake2 "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

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
	assertJsonEqual(t, []byte(`["nodejs","python"]`), res)
}

func Test_allowedToMutate(t *testing.T) {
	deploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "nginx:1"}},
				},
			},
		},
	}
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}
	mutatedDeploy := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "nginx:2"}},
				},
			},
		},
	}
	tests := []struct {
		name        string
		oldObject   client.Object
		object      client.Object
		autoRestart bool
		want        bool
	}{
		{"identical deployments", deploy.DeepCopy(), deploy.DeepCopy(), false, false},
		{"identical deployments, auto restart", deploy.DeepCopy(), deploy.DeepCopy(), true, true}, //should try and mutate in case deployment should no longer have annotations and mutators need to run to remove annotations
		{"changed pod template", deploy.DeepCopy(), &mutatedDeploy, false, true},
		{"non-workload", &corev1.ConfigMap{}, &corev1.ConfigMap{}, false, false},
		{"non-workload, auto restart", &corev1.ConfigMap{}, &corev1.ConfigMap{Data: map[string]string{"test": "test"}}, true, false},
		{"create (oldObject nil)", nil, deploy.DeepCopy(), false, true},
		{"namespace, auto restart false", nil, &namespace, false, true},
		{"namespace, auto restart true", nil, &namespace, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, allowedToMutate(tt.oldObject, tt.object, tt.autoRestart))
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

func TestMonitor_MutateObject(t *testing.T) {
	tests := []struct {
		name                        string
		config                      MonitorConfig
		deploymentNs                string
		serviceNs                   string
		deploymentSelector          map[string]string
		serviceSelector             map[string]string
		expectedWorkloadAnnotations map[string]string
	}{
		{
			name:                        "same namespace, same selector, monitorallservices true, not excluded",
			config:                      createConfig(true, nil, nil, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "same"},
			serviceSelector:             map[string]string{"app": "same"},
			expectedWorkloadAnnotations: buildAnnotations(instrumentation.TypeJava),
		},
		{
			name:                        "different namespace, same selector, monitorallservices true, not excluded",
			config:                      createConfig(true, nil, nil, false),
			deploymentNs:                "namespace-2",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "same"},
			serviceSelector:             map[string]string{"app": "same"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, different selector, monitorallservices true, not excluded",
			config:                      createConfig(true, nil, nil, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-2"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices false, not excluded",
			config:                      createConfig(false, nil, nil, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices true, excluded namespace",
			config:                      createConfig(false, []string{"namespace-1"}, nil, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices true, excluded service",
			config:                      createConfig(false, nil, []string{"namespace-1/svc-16"}, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test each workload type
			workloadTypes := []struct {
				name   string
				create func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error)
			}{
				{
					name: "Deployment",
					create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
						deployment := newTestDeployment("workload-16", ns, selector)
						return clientset.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
					},
				},
				{
					name: "StatefulSet",
					create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
						statefulset := newTestStatefulSet("workload-16", ns, selector)
						return clientset.AppsV1().StatefulSets(ns).Create(ctx, statefulset, metav1.CreateOptions{})
					},
				},
				{
					name: "DaemonSet",
					create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
						daemonset := newTestDaemonSet("workload-16", ns, selector)
						return clientset.AppsV1().DaemonSets(ns).Create(ctx, daemonset, metav1.CreateOptions{})
					},
				},
			}

			for _, workload := range workloadTypes {
				t.Run(workload.name, func(t *testing.T) {
					// Setup fresh clients for each workload test
					fakeClient := fake2.NewFakeClient()
					clientset := fake.NewSimpleClientset()
					ctx := context.TODO()

					monitor := NewMonitor(ctx, tt.config, clientset, fakeClient, fakeClient)

					// Create service
					service := newTestService("svc-16", tt.serviceNs, tt.serviceSelector)

					// Setup test environment
					serviceNamespace := createNamespace(t, clientset, ctx, service.Namespace)
					if tt.deploymentNs != serviceNamespace.Name {
						createNamespace(t, clientset, ctx, tt.deploymentNs)
					}

					// Create service
					_, err := clientset.CoreV1().Services(service.Namespace).Create(ctx, service, metav1.CreateOptions{})
					assert.NoError(t, err)

					// Create workload
					workloadObj, err := workload.create(clientset, ctx, tt.deploymentNs, tt.deploymentSelector)
					assert.NoError(t, err)

					// Test
					mutatedAnnotations := monitor.MutateObject(nil, workloadObj)
					assert.Equal(t, tt.expectedWorkloadAnnotations, mutatedAnnotations)
				})
			}
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
		{
			name: "manually specified annotation is not touched",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
						},
					},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
			wantMutated:        map[string]string{},
		},
		{
			name: "remove all",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: buildAnnotations("java"),
						},
					},
				},
			},
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("java"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMutated := mutate(tt.obj, tt.languagesToMonitor)
			switch tt.obj.(type) {
			case *appsv1.Deployment, *appsv1.DaemonSet, *appsv1.StatefulSet:
				assert.Equal(t, tt.wantObjAnnotations, getPodTemplate(tt.obj).GetAnnotations())
			default:
				assert.Equal(t, tt.wantObjAnnotations, tt.obj.GetAnnotations())
			}
			assert.Equal(t, tt.wantMutated, gotMutated)
		})
	}
}

func Test_StartupMutateObject(t *testing.T) {
	testService := newTestService("service-1", "default", map[string]string{"test": "test"})
	testDeployment := newTestDeployment("deployment-1", "default", map[string]string{"test": "test"})
	notMatchingService := newTestService("service-2", "default", map[string]string{"test2": "test2"})
	config := createConfig(true, nil, nil, true)
	clientset := fake.NewSimpleClientset(testService, testDeployment, notMatchingService)
	_ = NewMonitor(context.TODO(), config, clientset, fake2.NewFakeClient(), fake2.NewFakeClient())
	// todo finish
}

func assertJsonEqual(t *testing.T, expectedJson []byte, actualJson []byte) {
	var obj1, obj2 interface{}

	err := json.Unmarshal(expectedJson, &obj1)
	assert.NoError(t, err)

	err = json.Unmarshal(actualJson, &obj2)
	assert.NoError(t, err)

	assert.Equal(t, obj1, obj2)
}

func createNamespace(t *testing.T, clientset *fake.Clientset, ctx context.Context, namespaceName string) *corev1.Namespace {
	serviceNamespace, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}, metav1.CreateOptions{})
	assert.NoError(t, err)
	return serviceNamespace
}

func newTestService(name string, namespace string, selector map[string]string) *corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
		},
	}
	return service.DeepCopy()
}

func newTestDeployment(name string, namespace string, labels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

func newTestStatefulSet(name, namespace string, selector map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
			},
		},
	}
}

func newTestDaemonSet(name, namespace string, selector map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
			},
		},
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

func createConfig(monitorAll bool, excludedNs, excludedSvcs []string, autoRestart bool) MonitorConfig {
	return MonitorConfig{
		MonitorAllServices: monitorAll,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		AutoRestart:        autoRestart,
		Exclude: struct {
			Namespaces []string `json:"namespaces"`
			Services   []string `json:"services"`
		}{
			Namespaces: excludedNs,
			Services:   excludedSvcs,
		},
		CustomSelector: AnnotationConfig{},
	}
}
