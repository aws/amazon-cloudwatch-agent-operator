package auto

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake2 "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

const defaultNs = "default"

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
	// todo test irregardless of order
	var s []string
	err = json.Unmarshal(res, &s)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"nodejs", "python"}, s)
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
			assert.Equal(t, tt.want, safeToMutate(tt.oldObject, tt.object, tt.autoRestart))
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
			config:                      createConfig(true, []string{"namespace-1"}, nil, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices true, excluded service",
			config:                      createConfig(true, nil, []string{"namespace-1/svc-16"}, false),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
	}

	workloadTypes := []struct {
		name   string
		create func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error)
	}{
		{
			name: "Deployment",
			create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
				deployment := newTestDeployment("workload-16", ns, selector, nil)
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
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					// Setup fresh clients for each workload test
					fakeClient := fake2.NewFakeClient()
					clientset := fake.NewSimpleClientset()
					ctx := context.TODO()

					logger := testr.New(t)
					monitor := NewMonitor(ctx, tt.config, clientset, fakeClient, fakeClient, logger)

					// Create service
					service := newTestService("svc-16", tt.serviceNs, tt.serviceSelector)

					// Setup test environment
					serviceNamespace := createNamespace(t, clientset, ctx, service.Namespace)
					if tt.deploymentNs != serviceNamespace.Name {
						createNamespace(t, clientset, ctx, tt.deploymentNs)
					}

					// Create service
					_, err := monitor.k8sInterface.CoreV1().Services(service.Namespace).Create(ctx, service, metav1.CreateOptions{})
					assert.NoError(t, err)

					// Create workload
					workloadObj, err := workload.create(clientset, ctx, tt.deploymentNs, tt.deploymentSelector)
					assert.NoError(t, err)
					// need to wait until service informer is updated
					err = waitForInformerUpdate(monitor, func(numKeys int) bool { return numKeys > 0 })
					assert.NoError(t, err)

					// Test
					mutatedAnnotations := monitor.MutateObject(nil, workloadObj)
					assert.Equal(t, tt.expectedWorkloadAnnotations, mutatedAnnotations)
				})
			}
		})
	}
}

func waitForInformerUpdate(monitor *Monitor, isValid func(int) bool) error {
	return wait.PollImmediate(1*time.Millisecond, 5*time.Millisecond, func() (bool, error) {
		return isValid(len(monitor.serviceInformer.GetStore().ListKeys())), nil
	})
}

func Test_OptOutByRemovingService(t *testing.T) {
	t.Run("auto restart true, delete and then restart operator", func(t *testing.T) {
		userAnnotations := map[string]string{"test": "blah"}
		annotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
		deployment := newTestDeployment("deployment", defaultNs, nil, annotations)
		objs := []runtime.Object{
			deployment,
		}

		clientset := fake.NewSimpleClientset(objs...)
		c := fake2.NewFakeClient(objs...)
		NewMonitor(context.TODO(), createConfig(true, nil, nil, true), clientset, c, c, testr.New(t))
		err := c.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
		assert.NoError(t, err)
		assert.Equal(t, userAnnotations, deployment.Spec.Template.Annotations)
	})

	t.Run("auto restart true, delete while operator running", func(t *testing.T) {
		userAnnotations := map[string]string{"test": "blah"}
		annotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
		labels := map[string]string{"app": "test"}
		service := newTestService("service", defaultNs, labels)
		deployment := newTestDeployment("deployment", defaultNs, labels, annotations)
		objs := []runtime.Object{
			service,
			deployment,
		}
		clientset := fake.NewSimpleClientset(objs...)
		c := fake2.NewFakeClient(objs...)
		monitor := NewMonitor(context.TODO(), createConfig(true, nil, nil, true), clientset, c, c, testr.New(t))
		err := clientset.CoreV1().Services(defaultNs).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		assert.NoError(t, err)
		err = waitForInformerUpdate(monitor, func(numKeys int) bool { return numKeys == 0 })
		assert.NoError(t, err)
		updatedDeployment, err := clientset.AppsV1().Deployments(defaultNs).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, userAnnotations, updatedDeployment.Spec.Template.Annotations)
	})

	t.Run("auto restart false, delete and then restart operator", func(t *testing.T) {
		userAnnotations := map[string]string{"test": "blah"}
		originalAnnotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
		deployment := newTestDeployment("deployment", defaultNs, nil, originalAnnotations)
		objs := []runtime.Object{
			deployment,
		}

		clientset := fake.NewSimpleClientset(objs...)
		c := fake2.NewFakeClient(objs...)
		NewMonitor(context.TODO(), createConfig(true, nil, nil, false), clientset, c, c, testr.New(t))
		err := c.Get(context.TODO(), client.ObjectKeyFromObject(deployment), deployment)
		assert.NoError(t, err)
		assert.Equal(t, originalAnnotations, deployment.Spec.Template.Annotations)
	})

	t.Run("auto restart false, delete while operator running", func(t *testing.T) {
		userAnnotations := map[string]string{"test": "blah"}
		originalAnnotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
		labels := map[string]string{"app": "test"}
		service := newTestService("service", defaultNs, labels)
		deployment := newTestDeployment("deployment", defaultNs, labels, originalAnnotations)
		objs := []runtime.Object{
			service,
			deployment,
		}
		clientset := fake.NewSimpleClientset(objs...)
		c := fake2.NewFakeClient(objs...)
		monitor := NewMonitor(context.TODO(), createConfig(true, nil, nil, false), clientset, c, c, testr.New(t))
		err := clientset.CoreV1().Services(defaultNs).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		assert.NoError(t, err)
		err = waitForInformerUpdate(monitor, func(numKeys int) bool { return numKeys == 0 })
		assert.NoError(t, err)
		updatedDeployment, err := clientset.AppsV1().Deployments(defaultNs).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, originalAnnotations, updatedDeployment.Spec.Template.Annotations)
	})
}

func Test_mutate(t *testing.T) {
	tests := []struct {
		name               string
		podAnnotations     map[string]string
		languagesToMonitor instrumentation.TypeSet
		wantObjAnnotations map[string]string
		wantMutated        map[string]string
	}{
		{
			name:               "java only",
			podAnnotations:     nil,
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("java"),
		},
		{
			name:           "java and python",
			podAnnotations: nil,
			languagesToMonitor: instrumentation.TypeSet{
				"java":   struct{}{},
				"python": struct{}{},
			},
			wantObjAnnotations: mergeMaps(buildAnnotations("java"), buildAnnotations("python")),
			wantMutated:        mergeMaps(buildAnnotations("java"), buildAnnotations("python")),
		},
		{
			name:               "remove python instrumentation",
			podAnnotations:     buildAnnotations("python"),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("python"),
		},
		{
			name:               "remove one of two languages",
			podAnnotations:     mergeMaps(buildAnnotations("python"), buildAnnotations("java")),
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("python"),
		},
		{
			name:               "manually specified annotation is not touched",
			podAnnotations:     map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
			wantMutated:        map[string]string{},
		},
		{
			name:               "remove all",
			podAnnotations:     buildAnnotations("java"),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("java"),
		},
		{
			name:               "remove only language annotations",
			podAnnotations:     mergeAnnotations(buildAnnotations("java"), map[string]string{"test": "test"}),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{"test": "test"},
			wantMutated:        buildAnnotations("java"),
		},
	}

	workloadTypes := []struct {
		name   string
		create func(annotations map[string]string) client.Object
	}{
		{
			name: "Deployment",
			create: func(annotations map[string]string) client.Object {
				return &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				}
			},
		},
		{
			name: "StatefulSet",
			create: func(annotations map[string]string) client.Object {
				return &appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				}
			},
		},
		{
			name: "DaemonSet",
			create: func(annotations map[string]string) client.Object {
				return &appsv1.DaemonSet{
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: annotations,
							},
						},
					},
				}
			},
		},
	}

	for _, workload := range workloadTypes {
		t.Run(workload.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					obj := workload.create(tt.podAnnotations).DeepCopyObject().(client.Object)
					// TODO test different shouldInsert values
					gotMutated := mutate(obj, tt.languagesToMonitor, true)
					assert.Equal(t, tt.wantObjAnnotations, getPodTemplate(obj).GetAnnotations())
					assert.Equal(t, tt.wantMutated, gotMutated)
				})
			}
		})
	}
}

func Test_StartupAutoRestart(t *testing.T) {
	service := newTestService("service-1", defaultNs, map[string]string{"test": "test"})
	matchingDeployment := newTestDeployment("deployment-1", defaultNs, map[string]string{"test": "test"}, nil)
	nonMatchingDeployment := newTestDeployment("deployment-2", defaultNs, map[string]string{}, nil)
	customSelectedDeployment := newTestDeployment("deployment-3", defaultNs, map[string]string{}, nil)
	config := MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		AutoRestart:        true,
		CustomSelector: AnnotationConfig{
			AnnotationResources{}, AnnotationResources{Deployments: []string{namespacedName(customSelectedDeployment)}},
			AnnotationResources{}, AnnotationResources{},
		},
	}
	objs := []runtime.Object{service, matchingDeployment, nonMatchingDeployment, customSelectedDeployment}
	clientset := fake.NewSimpleClientset(objs...)
	fakeClient := fake2.NewFakeClient(objs...)
	m := NewMonitor(context.TODO(), config, clientset, fakeClient, fakeClient, testr.New(t))

	updatedMatchingDeployment, err := m.k8sInterface.AppsV1().Deployments(defaultNs).Get(context.TODO(), matchingDeployment.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, buildAnnotations(instrumentation.TypeJava), updatedMatchingDeployment.Spec.Template.GetAnnotations())
	updatedNonMatchingDeployment, err := m.k8sInterface.AppsV1().Deployments(defaultNs).Get(context.TODO(), nonMatchingDeployment.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Empty(t, updatedNonMatchingDeployment.Spec.Template.GetAnnotations())
	err = fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(customSelectedDeployment), customSelectedDeployment)
	assert.NoError(t, err)
	assert.Equal(t, buildAnnotations(instrumentation.TypePython), customSelectedDeployment.Spec.Template.GetAnnotations())
}

func Test_listServiceDeployments(t *testing.T) {
	testService := newTestService("service-1", defaultNs, map[string]string{"test": "test"})
	testDeployment := newTestDeployment("deployment-1", defaultNs, map[string]string{"test": "test"}, nil)
	notMatchingService := newTestService("service-2", defaultNs, map[string]string{"test2": "test2"})
	clientset := fake.NewSimpleClientset(testService, testDeployment, notMatchingService)
	m := Monitor{k8sInterface: clientset, logger: testr.New(t)}
	matchingServiceDeployments := m.listServiceDeployments(context.TODO(), testService)
	assert.Len(t, matchingServiceDeployments, 1)
	notMatchingServiceDeployments := m.listServiceDeployments(context.TODO(), notMatchingService)
	assert.Len(t, notMatchingServiceDeployments, 0)
}

// Helper functions

func assertJsonEqual(t *testing.T, expectedJson []byte, actualJson []byte) {
	var obj1, obj2 interface{}

	err := json.Unmarshal(expectedJson, &obj1)
	assert.NoError(t, err)

	err = json.Unmarshal(actualJson, &obj2)
	assert.NoError(t, err)

	assert.Equal(t, obj1, obj2)
}

func createNamespace(t *testing.T, clientset *fake.Clientset, ctx context.Context, namespaceName string) *corev1.Namespace {
	namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	serviceNamespace, err := clientset.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{})
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

func newTestDeployment(name string, namespace string, labels map[string]string, annotations map[string]string) *appsv1.Deployment {
	deployment := appsv1.Deployment{
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
					Labels:      labels,
					Annotations: annotations,
				},
			},
		},
	}
	return deployment.DeepCopy()
}

func newTestStatefulSet(name, namespace string, selector map[string]string) *appsv1.StatefulSet {
	statefulSet := appsv1.StatefulSet{
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
	return statefulSet.DeepCopy()
}

func newTestDaemonSet(name, namespace string, selector map[string]string) *appsv1.DaemonSet {
	daemonSet := appsv1.DaemonSet{
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
	return daemonSet.DeepCopy()
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
