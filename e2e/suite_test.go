// Copyright Contributors to the KubeOpenCode project

// Package e2e contains end-to-end tests for KubeOpenCode
package e2e

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var (
	k8sClient  client.Client
	clientset  *kubernetes.Clientset
	ctx        context.Context
	cancel     context.CancelFunc
	scheme     *runtime.Scheme
	testNS     string
	platformNS string
	echoImage  string
)

const (
	// Timeout for e2e tests (longer than integration tests)
	timeout = time.Minute * 5

	// Interval for polling
	interval = time.Second * 2

	// Default test namespace
	defaultTestNS = "kubeopencode-e2e-test"

	// Default echo agent image
	defaultEchoImage = "quay.io/kubeopencode/kubeopencode-agent-echo:latest"

	// Test ServiceAccount name for e2e tests
	testServiceAccount = "kubeopencode-e2e-agent"
)

// Test labels for selective execution
// Usage: make e2e-test-label LABEL="task"
const (
	LabelTask   = "task"
	LabelAgent  = "agent"
	LabelServer = "server"

	// Default platform namespace for cross-namespace tests
	defaultPlatformNS = "kubeopencode-platform-e2e"

	// Extended timeout for server mode tests
	serverTimeout = time.Minute * 10
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubeOpenCode E2E Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("Setting up test configuration")

	// Get test namespace from env or use default
	testNS = os.Getenv("E2E_TEST_NAMESPACE")
	if testNS == "" {
		testNS = defaultTestNS
	}

	// Get echo agent image from env or use default
	echoImage = os.Getenv("E2E_ECHO_IMAGE")
	if echoImage == "" {
		echoImage = defaultEchoImage
	}

	// Get platform namespace from env or use default
	platformNS = os.Getenv("E2E_PLATFORM_NAMESPACE")
	if platformNS == "" {
		platformNS = defaultPlatformNS
	}

	By("Connecting to Kubernetes cluster")

	// Use kubeconfig from env or default location
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// Try in-cluster config
		config, err = ctrl.GetConfig()
		Expect(err).NotTo(HaveOccurred(), "Failed to get Kubernetes config")
	}
	Expect(config).NotTo(BeNil())

	// Create scheme with all required types
	scheme = runtime.NewScheme()
	err = kubeopenv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = batchv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = appsv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = rbacv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// Create controller-runtime client
	k8sClient, err = client.New(config, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Create clientset for pod logs and other operations
	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())

	By("Creating test namespace")
	ns := &corev1.Namespace{}
	ns.Name = testNS
	err = k8sClient.Create(ctx, ns)
	if err != nil && !isAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	By("Creating test ServiceAccount")
	sa := &corev1.ServiceAccount{}
	sa.Name = testServiceAccount
	sa.Namespace = testNS
	err = k8sClient.Create(ctx, sa)
	if err != nil && !isAlreadyExistsGeneric(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	By("Creating platform namespace for cross-namespace tests")
	platformNSObj := &corev1.Namespace{}
	platformNSObj.Name = platformNS
	err = k8sClient.Create(ctx, platformNSObj)
	if err != nil && !isAlreadyExistsGeneric(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	By("Creating ServiceAccount in platform namespace")
	platformSA := &corev1.ServiceAccount{}
	platformSA.Name = testServiceAccount
	platformSA.Namespace = platformNS
	err = k8sClient.Create(ctx, platformSA)
	if err != nil && !isAlreadyExistsGeneric(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	By("Creating RBAC for cross-namespace Pod creation")
	// Create ClusterRole for cross-namespace Pod creation
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeopencode-e2e-cross-ns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "configmaps", "secrets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
	err = k8sClient.Create(ctx, clusterRole)
	if err != nil && !isAlreadyExistsGeneric(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// Create ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeopencode-e2e-cross-ns",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      testServiceAccount,
				Namespace: platformNS,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kubeopencode-e2e-cross-ns",
		},
	}
	err = k8sClient.Create(ctx, clusterRoleBinding)
	if err != nil && !isAlreadyExistsGeneric(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	By("Verifying controller is running")
	// Check that the controller deployment exists and is ready
	Eventually(func() bool {
		pods := &corev1.PodList{}
		err := k8sClient.List(ctx, pods, client.InNamespace("kubeopencode-system"), client.MatchingLabels{
			"app.kubernetes.io/name":      "kubeopencode",
			"app.kubernetes.io/component": "controller",
		})
		if err != nil {
			return false
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				return true
			}
		}
		return false
	}, timeout, interval).Should(BeTrue(), "Controller should be running")

	GinkgoWriter.Printf("E2E test setup complete. Namespace: %s, Echo Image: %s\n", testNS, echoImage)
})

var _ = AfterSuite(func() {
	By("Cleaning up test namespace")

	// Delete all Tasks in test namespace
	tasks := &kubeopenv1alpha1.TaskList{}
	if err := k8sClient.List(ctx, tasks, client.InNamespace(testNS)); err == nil {
		for _, task := range tasks.Items {
			_ = k8sClient.Delete(ctx, &task)
		}
	}

	// Delete all Agents in test namespace
	agents := &kubeopenv1alpha1.AgentList{}
	if err := k8sClient.List(ctx, agents, client.InNamespace(testNS)); err == nil {
		for _, a := range agents.Items {
			_ = k8sClient.Delete(ctx, &a)
		}
	}

	By("Cleaning up platform namespace")
	// Delete all Tasks in platform namespace
	if err := k8sClient.List(ctx, tasks, client.InNamespace(platformNS)); err == nil {
		for _, task := range tasks.Items {
			_ = k8sClient.Delete(ctx, &task)
		}
	}

	// Delete all Agents in platform namespace
	if err := k8sClient.List(ctx, agents, client.InNamespace(platformNS)); err == nil {
		for _, a := range agents.Items {
			_ = k8sClient.Delete(ctx, &a)
		}
	}

	// Delete Server Mode Deployments and Services in platform namespace
	deployments := &appsv1.DeploymentList{}
	if err := k8sClient.List(ctx, deployments, client.InNamespace(platformNS)); err == nil {
		for _, d := range deployments.Items {
			_ = k8sClient.Delete(ctx, &d)
		}
	}

	services := &corev1.ServiceList{}
	if err := k8sClient.List(ctx, services, client.InNamespace(platformNS)); err == nil {
		for _, s := range services.Items {
			_ = k8sClient.Delete(ctx, &s)
		}
	}

	// Wait for resources to be cleaned up
	time.Sleep(5 * time.Second)

	// Delete namespace if it was created by the test
	if testNS == defaultTestNS {
		ns := &corev1.Namespace{}
		ns.Name = testNS
		_ = k8sClient.Delete(ctx, ns)
	}

	// Delete platform namespace if it was created by the test
	if platformNS == defaultPlatformNS {
		ns := &corev1.Namespace{}
		ns.Name = platformNS
		_ = k8sClient.Delete(ctx, ns)
	}

	// Clean up RBAC resources
	_ = k8sClient.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "kubeopencode-e2e-cross-ns"}})
	_ = k8sClient.Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "kubeopencode-e2e-cross-ns"}})

	cancel()
	GinkgoWriter.Println("E2E test cleanup complete")
})

// isAlreadyExists checks if the error is an "already exists" error for namespace
func isAlreadyExists(err error) bool {
	return err != nil && err.Error() == "namespaces \""+testNS+"\" already exists"
}

// isAlreadyExistsGeneric checks if the error is an "already exists" error
func isAlreadyExistsGeneric(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "already exists")
}

// Helper function to generate unique names for test resources
func uniqueName(prefix string) string {
	return prefix + "-" + time.Now().Format("150405")
}

// getTaskCondition retrieves a specific condition from Task status
func getTaskCondition(task *kubeopenv1alpha1.Task, conditionType string) *metav1.Condition {
	for i := range task.Status.Conditions {
		if task.Status.Conditions[i].Type == conditionType {
			return &task.Status.Conditions[i]
		}
	}
	return nil
}

// getAgentCondition retrieves a specific condition from Agent status
func getAgentCondition(agent *kubeopenv1alpha1.Agent, conditionType string) *metav1.Condition {
	for i := range agent.Status.Conditions {
		if agent.Status.Conditions[i].Type == conditionType {
			return &agent.Status.Conditions[i]
		}
	}
	return nil
}

// getPodForTask retrieves the Pod created for a Task
func getPodForTask(testCtx context.Context, namespace, taskName string) (*corev1.Pod, error) {
	pods := &corev1.PodList{}
	err := k8sClient.List(testCtx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels{"kubeopencode.io/task": taskName})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, nil
	}
	return &pods.Items[0], nil
}
