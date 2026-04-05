// Copyright Contributors to the KubeOpenCode project

// Package e2e contains end-to-end tests for KubeOpenCode
package e2e

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	"k8s.io/apimachinery/pkg/types"
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

	// agentImage is the OpenCode init container image used to copy the opencode
	// binary to /tools. All E2E Agent/AgentTemplate specs must set AgentImage
	// to this value to avoid the controller falling back to DefaultAgentImage
	// (which uses :latest and triggers PullAlways in Kind).
	agentImage string

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

	// Default echo agent image (uses :dev tag to get IfNotPresent pull policy in Kind)
	defaultEchoImage = "quay.io/kubeopencode/kubeopencode-agent-echo:dev"

	// Default OpenCode agent image / init container (uses :dev tag to get IfNotPresent pull policy in Kind)
	defaultAgentImage = "quay.io/kubeopencode/kubeopencode-agent-opencode:dev"

	// Default OpenCode agent image (init container that copies opencode binary)
	defaultOpenCodeImage = "quay.io/kubeopencode/kubeopencode-agent-opencode:dev"

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
	LabelCronTask = "crontask"

	// Extended timeout for Deployment readiness tests
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

	// Get agent image (init container) from env or use default
	agentImage = os.Getenv("E2E_AGENT_IMAGE")
	if agentImage == "" {
		agentImage = defaultAgentImage
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

	GinkgoWriter.Printf("E2E test setup complete. Namespace: %s, Echo Image: %s, Agent Image: %s, OpenCode Image: %s\n", testNS, echoImage, agentImage, opencodeImage)
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

	// Delete all CronTasks in test namespace
	cronTasks := &kubeopenv1alpha1.CronTaskList{}
	if err := k8sClient.List(ctx, cronTasks, client.InNamespace(testNS)); err == nil {
		for _, ct := range cronTasks.Items {
			_ = k8sClient.Delete(ctx, &ct)
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
	return fmt.Sprintf("%s-%s-%04d", prefix, time.Now().Format("150405"), rand.IntN(10000)) //nolint:gosec // Test helper, no security requirement
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

// dumpTaskDiagnostics prints diagnostic information for a Task to help debug
// flaky test failures (e.g., Pod startup timeouts in CI).
func dumpTaskDiagnostics(testCtx context.Context, namespace, taskName string) {
	GinkgoWriter.Printf("\n=== DIAGNOSTICS for Task %s/%s ===\n", namespace, taskName)

	// Dump Task status
	task := &kubeopenv1alpha1.Task{}
	taskKey := types.NamespacedName{Name: taskName, Namespace: namespace}
	if err := k8sClient.Get(testCtx, taskKey, task); err != nil {
		GinkgoWriter.Printf("Failed to get Task: %v\n", err)
	} else {
		GinkgoWriter.Printf("Task Phase: %s\n", task.Status.Phase)
		for _, c := range task.Status.Conditions {
			GinkgoWriter.Printf("  Condition: %s=%s (reason=%s, message=%s)\n",
				c.Type, c.Status, c.Reason, c.Message)
		}
	}

	// Dump Pod status
	pod, err := getPodForTask(testCtx, namespace, taskName)
	switch {
	case err != nil:
		GinkgoWriter.Printf("Failed to get Pod: %v\n", err)
	case pod == nil:
		GinkgoWriter.Println("No Pod found for Task")
	default:
		GinkgoWriter.Printf("Pod %s Phase: %s\n", pod.Name, pod.Status.Phase)
		for _, cs := range pod.Status.InitContainerStatuses {
			GinkgoWriter.Printf("  Init Container %s: ready=%v", cs.Name, cs.Ready)
			if cs.State.Waiting != nil {
				GinkgoWriter.Printf(" waiting(reason=%s, message=%s)", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
			if cs.State.Terminated != nil {
				GinkgoWriter.Printf(" terminated(exitCode=%d, reason=%s)", cs.State.Terminated.ExitCode, cs.State.Terminated.Reason)
			}
			GinkgoWriter.Println()
		}
		for _, cs := range pod.Status.ContainerStatuses {
			GinkgoWriter.Printf("  Container %s: ready=%v", cs.Name, cs.Ready)
			if cs.State.Waiting != nil {
				GinkgoWriter.Printf(" waiting(reason=%s, message=%s)", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
			if cs.State.Terminated != nil {
				GinkgoWriter.Printf(" terminated(exitCode=%d, reason=%s)", cs.State.Terminated.ExitCode, cs.State.Terminated.Reason)
			}
			GinkgoWriter.Println()
		}

		// Dump Pod Events
		events, err := clientset.CoreV1().Events(namespace).List(testCtx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", pod.Name),
		})
		if err == nil && len(events.Items) > 0 {
			GinkgoWriter.Println("  Events:")
			for _, e := range events.Items {
				GinkgoWriter.Printf("    %s %s: %s\n", e.Reason, e.Type, e.Message)
			}
		}
	}
	GinkgoWriter.Println("=== END DIAGNOSTICS ===")
}

// waitForTaskPhase waits for a Task to reach the expected phase, and dumps
// diagnostics if it times out or the Task fails unexpectedly.
func waitForTaskPhase(namespace, taskName string, expectedPhase kubeopenv1alpha1.TaskPhase) {
	taskKey := types.NamespacedName{Name: taskName, Namespace: namespace}
	succeeded := false
	defer func() {
		if !succeeded {
			dumpTaskDiagnostics(ctx, namespace, taskName)
		}
	}()
	Eventually(func() kubeopenv1alpha1.TaskPhase {
		t := &kubeopenv1alpha1.Task{}
		if err := k8sClient.Get(ctx, taskKey, t); err != nil {
			return ""
		}
		return t.Status.Phase
	}, timeout, interval).Should(Equal(expectedPhase))
	succeeded = true
}
