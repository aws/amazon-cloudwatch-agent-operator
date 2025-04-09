package auto

import (
	"context"
	"encoding/json"
	"github.com/aws/amazon-cloudwatch-agent-operator/pkg/instrumentation"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake2 "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

const defaultNs = "default"

var workloadTypes = []struct {
	name          string
	create        func(name, namespace string, labels, annotations map[string]string) client.Object
	get           func(clientset kubernetes.Interface, namespace, name string) (client.Object, error)
	getWithClient func(c client.Reader, ns, name string) (client.Object, error)
}{
	{
		name: "Deployment",
		create: func(name, ns string, labels, annotations map[string]string) client.Object {
			return newTestDeployment(name, ns, labels, annotations)
		},
		get: func(c kubernetes.Interface, ns, name string) (client.Object, error) {
			return c.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
		},
		getWithClient: func(c client.Reader, ns, name string) (client.Object, error) {
			obj := &appsv1.Deployment{}
			err := c.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, obj)
			return obj, err
		},
	},
	{
		name: "StatefulSet",
		create: func(name, ns string, labels, annotations map[string]string) client.Object {
			return newTestStatefulSet(name, ns, labels, annotations)
		},
		get: func(c kubernetes.Interface, ns, name string) (client.Object, error) {
			return c.AppsV1().StatefulSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
		},
		getWithClient: func(c client.Reader, ns, name string) (client.Object, error) {
			obj := &appsv1.StatefulSet{}
			err := c.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, obj)
			return obj, err
		},
	},
	{
		name: "DaemonSet",
		create: func(name, ns string, labels, annotations map[string]string) client.Object {
			return newTestDaemonSet(name, ns, labels, annotations)
		},
		get: func(c kubernetes.Interface, ns, name string) (client.Object, error) {
			return c.AppsV1().DaemonSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
		},
		getWithClient: func(c client.Reader, ns, name string) (client.Object, error) {
			obj := &appsv1.DaemonSet{}
			err := c.Get(context.TODO(), client.ObjectKey{Namespace: ns, Name: name}, obj)
			return obj, err
		},
	},
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
	// todo test irregardless of order
	var s []string
	err = json.Unmarshal(res, &s)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"nodejs", "python"}, s)
}

func Test_safeToMutate(t *testing.T) {
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
		name                       string
		oldObject                  client.Object
		object                     client.Object
		restartPods                bool
		restartPodsCustomSelectors bool
		want                       bool
	}{
		{"identical deployments", deploy.DeepCopy(), deploy.DeepCopy(), false, false, false},
		{"identical deployments, auto restart", deploy.DeepCopy(), deploy.DeepCopy(), true, false, true}, //should try and mutate in case deployment should no longer have annotations and mutators need to run to remove annotations
		{"changed pod template", deploy.DeepCopy(), &mutatedDeploy, false, false, true},
		{"non-workload", &corev1.ConfigMap{}, &corev1.ConfigMap{}, false, false, false},
		{"non-workload, auto restart", &corev1.ConfigMap{}, &corev1.ConfigMap{Data: map[string]string{"test": "test"}}, true, false, false},
		{"create (oldObject nil)", nil, deploy.DeepCopy(), false, false, true},
		{"namespace, auto restart false", nil, &namespace, false, false, true},
		{"namespace, auto restart true", nil, &namespace, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//noinspection GoDeprecation
			assert.Equal(t, tt.want, safeToMutate(tt.oldObject, tt.object, tt.restartPods))
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

type MutateObjectTest struct {
	name                        string
	config                      MonitorConfig
	deploymentNs                string
	serviceNs                   string
	deploymentSelector          map[string]string
	serviceSelector             map[string]string
	expectedWorkloadAnnotations map[string]string
}

var none = AnnotationConfig{}

func TestMonitor_MutateObject(t *testing.T) {
	annotated := buildAnnotations(instrumentation.TypeJava)
	tests := []MutateObjectTest{
		{
			name:                        "same namespace, same selector, monitorallservices true, not excluded",
			config:                      simpleConfig(true, false, none, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "same"},
			serviceSelector:             map[string]string{"app": "same"},
			expectedWorkloadAnnotations: annotated,
		},
		{
			name:                        "different namespace, same selector, monitorallservices true, not excluded",
			config:                      simpleConfig(true, false, none, none),
			deploymentNs:                "namespace-2",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "same"},
			serviceSelector:             map[string]string{"app": "same"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, different selector, monitorallservices true, not excluded",
			config:                      simpleConfig(true, false, none, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-2"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices false, not excluded",
			config:                      simpleConfig(false, false, none, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices true, excluded namespace",
			config:                      simpleConfig(true, false, none, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name:                        "same namespace, same selector, monitorallservices true, excluded service",
			config:                      simpleConfig(true, false, none, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-1",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-1"},
			expectedWorkloadAnnotations: map[string]string{},
		},
		{
			name: "different namespace, different selector, monitorallservices false, custom selected workload",
			config: simpleConfig(true, false, AnnotationConfig{Java: AnnotationResources{
				Namespaces:   nil,
				Deployments:  []string{"namespace-1/workload"},
				DaemonSets:   []string{"namespace-1/workload"},
				StatefulSets: []string{"namespace-1/workload"},
			}}, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-2",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-2"},
			expectedWorkloadAnnotations: annotated,
		},
		{
			name: "different namespace, different selector, monitorallservices false, custom selected namespace of workload",
			config: simpleConfig(true, false, AnnotationConfig{Java: AnnotationResources{
				Namespaces:   []string{"namespace-1"},
				Deployments:  nil,
				DaemonSets:   nil,
				StatefulSets: nil,
			}}, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-2",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-2"},
			expectedWorkloadAnnotations: map[string]string{}, // empty because even though it should be custom selected, it is modified on the pod level for namespaces, so the pod template is not updated
		},
		{
			name: "different namespace, different selector, monitorallservices false, custom selected namespace of service, not workload",
			config: simpleConfig(true, false, AnnotationConfig{Java: AnnotationResources{
				Namespaces:   []string{"namespace-2"},
				Deployments:  nil,
				DaemonSets:   nil,
				StatefulSets: nil,
			}}, none),
			deploymentNs:                "namespace-1",
			serviceNs:                   "namespace-2",
			deploymentSelector:          map[string]string{"app": "different-1"},
			serviceSelector:             map[string]string{"app": "different-2"},
			expectedWorkloadAnnotations: map[string]string{}, // empty because even though it should be custom selected, it is modified on the pod level for namespaces, so the pod template is not updated
		},
	}

	workloadTypes := []struct {
		name   string
		create func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error)
	}{
		{
			name: "Deployment",
			create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
				deployment := newTestDeployment("workload", ns, selector, nil)
				return clientset.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
			},
		},
		{
			name: "StatefulSet",
			create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
				statefulset := newTestStatefulSet("workload", ns, selector, nil)
				return clientset.AppsV1().StatefulSets(ns).Create(ctx, statefulset, metav1.CreateOptions{})
			},
		},
		{
			name: "DaemonSet",
			create: func(clientset *fake.Clientset, ctx context.Context, ns string, selector map[string]string) (client.Object, error) {
				daemonset := newTestDaemonSet("workload", ns, selector, nil)
				return clientset.AppsV1().DaemonSets(ns).Create(ctx, daemonset, metav1.CreateOptions{})
			},
		},
	}

	for _, workload := range workloadTypes {
		t.Run(workload.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					//t.Parallel()
					// Setup fresh clients for each workload test
					fakeClient := fake2.NewFakeClient()
					clientset := fake.NewSimpleClientset()
					ctx := context.TODO()

					logger := testr.New(t)
					monitor := NewMonitor(ctx, tt.config, clientset, fakeClient, fakeClient, logger)

					// Create service
					service := newTestService("svc", tt.serviceNs, tt.serviceSelector)

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
	for _, wt := range workloadTypes {
		t.Run(wt.name, func(t *testing.T) {
			t.Run("auto restart true, delete and then restart operator", func(t *testing.T) {
				userAnnotations := map[string]string{"test": "blah"}
				annotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
				workload := wt.create("deployment", defaultNs, nil, annotations)

				clientset := fake.NewSimpleClientset(workload)
				c := fake2.NewFakeClient(workload)
				var config MonitorConfig = simpleConfig(true, true, none, none)
				var k8sInterface kubernetes.Interface = clientset
				var logger logr.Logger = testr.New(t)
				monitor := NewMonitor(context.TODO(), config, k8sInterface, c, c, logger)
				MutateAndPatchAll(monitor, context.TODO())
				updatedWorkload, err := wt.getWithClient(c, defaultNs, workload.GetName())
				assert.NoError(t, err)
				assert.Equal(t, userAnnotations, getPodTemplate(updatedWorkload).GetAnnotations())
			})

			t.Run("auto restart true, delete while operator running", func(t *testing.T) {
				userAnnotations := map[string]string{"test": "blah"}
				annotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
				labels := map[string]string{"app": "test"}
				service := newTestService("service", defaultNs, labels)
				workload := wt.create("workload", defaultNs, labels, annotations)

				clientset := fake.NewSimpleClientset(service, workload)
				c := fake2.NewFakeClient(service, workload)
				var config MonitorConfig = simpleConfig(true, true, none, none)
				var k8sInterface kubernetes.Interface = clientset
				var logger logr.Logger = testr.New(t)
				monitor := NewMonitor(context.TODO(), config, k8sInterface, c, c, logger)
				MutateAndPatchAll(monitor, context.TODO())

				err := clientset.CoreV1().Services(defaultNs).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
				assert.NoError(t, err)
				err = waitForInformerUpdate(monitor, func(numKeys int) bool { return numKeys == 0 })
				assert.NoError(t, err)

				updatedWorkload, err := wt.get(clientset, defaultNs, workload.GetName())
				assert.NoError(t, err)
				assert.Equal(t, userAnnotations, getPodTemplate(updatedWorkload).GetAnnotations())
			})

			t.Run("auto restart false, delete and then restart operator", func(t *testing.T) {
				userAnnotations := map[string]string{"test": "blah"}
				originalAnnotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
				workload := wt.create("workload", defaultNs, nil, originalAnnotations)

				clientset := fake.NewSimpleClientset(workload)
				c := fake2.NewFakeClient(workload)
				var config MonitorConfig = simpleConfig(true, false, none, none)
				var k8sInterface kubernetes.Interface = clientset
				var logger logr.Logger = testr.New(t)
				monitor := NewMonitor(context.TODO(), config, k8sInterface, c, c, logger)
				MutateAndPatchAll(monitor, context.TODO())

				updatedWorkload, err := wt.get(clientset, defaultNs, workload.GetName())
				assert.NoError(t, err)
				assert.Equal(t, originalAnnotations, getPodTemplate(updatedWorkload).GetAnnotations())
			})

			t.Run("auto restart false, delete while operator running", func(t *testing.T) {
				userAnnotations := map[string]string{"test": "blah"}
				originalAnnotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
				labels := map[string]string{"app": "test"}
				service := newTestService("service", defaultNs, labels)
				workload := wt.create("workload", defaultNs, labels, originalAnnotations)

				clientset := fake.NewSimpleClientset(service, workload)
				c := fake2.NewFakeClient(service, workload)
				var config MonitorConfig = simpleConfig(true, false, none, none)
				var k8sInterface kubernetes.Interface = clientset
				var logger logr.Logger = testr.New(t)
				monitor := NewMonitor(context.TODO(), config, k8sInterface, c, c, logger)
				MutateAndPatchAll(monitor, context.TODO())

				err := clientset.CoreV1().Services(defaultNs).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
				assert.NoError(t, err)
				err = waitForInformerUpdate(monitor, func(numKeys int) bool { return numKeys == 0 })
				assert.NoError(t, err)

				updatedWorkload, err := wt.get(clientset, defaultNs, workload.GetName())
				assert.NoError(t, err)
				assert.Equal(t, originalAnnotations, getPodTemplate(updatedWorkload).GetAnnotations())
			})
		})
	}
}

func Test_OptOutByDisablingMonitorAllServices(t *testing.T) {
	for _, wt := range workloadTypes {
		t.Run(wt.name, func(t *testing.T) {
			t.Run("auto restart true", func(t *testing.T) {
				userAnnotations := map[string]string{"test": "blah"}
				annotations := mergeMaps(buildAnnotations(instrumentation.TypeJava), userAnnotations)
				labels := map[string]string{"app": "test"}
				service := newTestService("service", defaultNs, labels)
				workload := wt.create("workload", defaultNs, labels, annotations)

				clientset := fake.NewSimpleClientset(service, workload)
				c := fake2.NewFakeClient(service, workload)
				var config MonitorConfig = simpleConfig(false, true, none, none)
				var k8sInterface kubernetes.Interface = clientset
				var logger logr.Logger = testr.New(t)
				monitor := NewMonitor(context.TODO(), config, k8sInterface, c, c, logger)
				MutateAndPatchAll(monitor, context.TODO())

				updatedWorkload, err := wt.get(clientset, defaultNs, workload.GetName())
				assert.NoError(t, err)
				assert.Equal(t, userAnnotations, getPodTemplate(updatedWorkload).GetAnnotations())

			})
		})
	}
}

func Test_mutate(t *testing.T) {
	tests := []struct {
		name               string
		podAnnotations     map[string]string
		languagesToMonitor instrumentation.TypeSet
		wantObjAnnotations map[string]string
		wantMutated        map[string]string
		shouldInsert       bool
	}{
		{
			name:               "java only",
			podAnnotations:     nil,
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("java"),
			shouldInsert:       true,
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
			shouldInsert:       true,
		},
		{
			name:               "remove python instrumentation",
			podAnnotations:     buildAnnotations("python"),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("python"),
			shouldInsert:       true,
		},
		{
			name:               "remove one of two languages",
			podAnnotations:     mergeMaps(buildAnnotations("python"), buildAnnotations("java")),
			languagesToMonitor: instrumentation.TypeSet{"java": struct{}{}},
			wantObjAnnotations: buildAnnotations("java"),
			wantMutated:        buildAnnotations("python"),
			shouldInsert:       true,
		},
		{
			name:               "manually specified annotation is not touched",
			podAnnotations:     map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{instrumentation.InjectAnnotationKey(instrumentation.TypeJava): defaultAnnotationValue},
			wantMutated:        map[string]string{},
			shouldInsert:       true,
		},
		{
			name:               "remove all",
			podAnnotations:     buildAnnotations("java"),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{},
			wantMutated:        buildAnnotations("java"),
			shouldInsert:       true,
		},
		{
			name:               "remove only language annotations",
			podAnnotations:     mergeAnnotations(buildAnnotations("java"), map[string]string{"test": "test"}),
			languagesToMonitor: instrumentation.TypeSet{},
			wantObjAnnotations: map[string]string{"test": "test"},
			wantMutated:        buildAnnotations("java"),
			shouldInsert:       true,
		},
		{
			name:               "respects isWorkloadAutoMonitored",
			podAnnotations:     mergeAnnotations(buildAnnotations("python"), map[string]string{"test": "test"}),
			languagesToMonitor: instrumentation.TypeSet{"python": struct{}{}, "java": struct{}{}},
			wantObjAnnotations: map[string]string{"test": "test"},
			wantMutated:        buildAnnotations("python"),
			shouldInsert:       false,
		},
	}

	for _, workload := range workloadTypes {
		t.Run(workload.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					obj := workload.create("workload", "default", nil, tt.podAnnotations).DeepCopyObject().(client.Object)
					// TODO test different isWorkloadAutoMonitored values
					gotMutated := mutate(obj, tt.languagesToMonitor)
					assert.Equal(t, tt.wantObjAnnotations, getPodTemplate(obj).GetAnnotations())
					assert.Equal(t, tt.wantMutated, gotMutated)
				})
			}
		})
	}
}

func Test_StartupRestartPods(t *testing.T) {
	service := newTestService("service-1", defaultNs, map[string]string{"test": "test"})
	matchingDeployment := newTestDeployment("deployment-1", defaultNs, map[string]string{"test": "test"}, nil)
	nonMatchingDeployment := newTestDeployment("deployment-2", defaultNs, map[string]string{}, nil)
	customSelectedDeployment := newTestDeployment("deployment-3", defaultNs, map[string]string{}, nil)
	config := MonitorConfig{
		MonitorAllServices: true,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		RestartPods:        true,
		CustomSelector: AnnotationConfig{
			AnnotationResources{}, AnnotationResources{Deployments: []string{namespacedName(customSelectedDeployment)}},
			AnnotationResources{}, AnnotationResources{},
		},
	}
	objs := []runtime.Object{service, matchingDeployment, nonMatchingDeployment, customSelectedDeployment}
	clientset := fake.NewSimpleClientset(objs...)
	fakeClient := fake2.NewFakeClient(objs...)
	var k8sInterface kubernetes.Interface = clientset
	var logger logr.Logger = testr.New(t)
	m := NewMonitor(context.TODO(), config, k8sInterface, fakeClient, fakeClient, logger)
	MutateAndPatchAll(m, context.TODO())
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

func newTestStatefulSet(name string, namespace string, labels map[string]string, annotations map[string]string) *appsv1.StatefulSet {
	statefulSet := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
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
	return statefulSet.DeepCopy()
}

func newTestDaemonSet(name string, namespace string, labels map[string]string, annotations map[string]string) *appsv1.DaemonSet {
	daemonSet := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
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

func simpleConfig(monitorAll bool, restartPods bool, customSelector AnnotationConfig, excluded AnnotationConfig) MonitorConfig {
	return MonitorConfig{
		MonitorAllServices: monitorAll,
		Languages:          instrumentation.NewTypeSet(instrumentation.TypeJava),
		RestartPods:        restartPods,
		Exclude:            excluded,
		CustomSelector:     customSelector,
	}
}
