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
	k8sClient client.Client
	clientset *kubernetes.Clientset
	ctx       context.Context
	cancel    context.CancelFunc
	scheme    *runtime.Scheme
	testNS    string
	echoImage string

	// OpenCode test configuration (for LabelOpenCode tests)
	opencodeImage string // OpenCode agent image (init container)
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

	// Default OpenCode agent image (init container that copies opencode binary)
	defaultOpenCodeImage = "quay.io/kubeopencode/kubeopencode-agent-opencode:latest"

	// Test ServiceAccount name for e2e tests
	testServiceAccount = "kubeopencode-e2e-agent"
)

// Test labels for selective execution
// Usage: make e2e-test-label LABEL="task"
const (
	LabelTask     = "task"
	LabelAgent    = "agent"
	LabelServer   = "server"
	LabelOpenCode = "opencode" // Tests using real OpenCode with free models

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

	// Setup OpenCode test configuration
	opencodeImage = os.Getenv("E2E_OPENCODE_IMAGE")
	if opencodeImage == "" {
		opencodeImage = defaultOpenCodeImage
	}

	GinkgoWriter.Printf("E2E test setup complete. Namespace: %s, Echo Image: %s, OpenCode Image: %s\n", testNS, echoImage, opencodeImage)
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

	// Delete all AgentTemplates in test namespace
	templates := &kubeopenv1alpha1.AgentTemplateList{}
	if err := k8sClient.List(ctx, templates, client.InNamespace(testNS)); err == nil {
		for _, t := range templates.Items {
			_ = k8sClient.Delete(ctx, &t)
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
