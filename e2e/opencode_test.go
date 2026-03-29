// Copyright Contributors to the KubeOpenCode project

// OpenCode E2E tests use real OpenCode with free models (opencode/big-pickle).
// These tests validate actual AI task execution, not just Kubernetes resource creation.
//
// Free models do not require an API key — OpenCode uses apiKey: "public" automatically.
//
// Run with: make e2e-test-label LABEL="opencode"
// Requires: opencode agent image loaded in Kind cluster (make e2e-agent-build && make e2e-agent-load)

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/controller"
)

// opencodeConfig returns an OpenCode config JSON string using the free big-pickle model.
func opencodeConfig() string {
	return `{
  "$schema": "https://opencode.ai/config.json",
  "model": "opencode/big-pickle",
  "small_model": "opencode/big-pickle",
  "share": "disabled",
  "autoupdate": false
}`
}

var _ = Describe("OpenCode E2E Tests", Label(LabelOpenCode), func() {

	Context("Server Mode - Real OpenCode Agent", func() {
		var (
			agentName     string
			agentKey      types.NamespacedName
			deploymentKey types.NamespacedName
		)

		BeforeEach(func() {
			agentName = uniqueName("oc-server")
			agentKey = types.NamespacedName{Name: agentName, Namespace: testNS}
			deploymentKey = types.NamespacedName{
				Name:      controller.ServerDeploymentName(agentName),
				Namespace: testNS,
			}

			By("Creating server-mode Agent with OpenCode free model")
			config := opencodeConfig()
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					AgentImage:         opencodeImage,
					ExecutorImage:      opencodeImage, // OpenCode image has deps needed by opencode serve
					AttachImage:        opencodeImage, // Use same image for attach pods in e2e (no separate attach image)
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
					Config: &config,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for server Deployment to be ready")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, deploymentKey, deployment); err != nil {
					return false
				}
				return deployment.Status.ReadyReplicas > 0
			}, serverTimeout, interval).Should(BeTrue(), "Server Deployment should have ready replicas")

			By("Waiting for Agent serverStatus to be populated")
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil && a.Status.ServerStatus.URL != ""
			}, serverTimeout, interval).Should(BeTrue())
		})

		AfterEach(func() {
			By("Cleaning up Agent")
			agent := &kubeopenv1alpha1.Agent{}
			if err := k8sClient.Get(ctx, agentKey, agent); err == nil {
				Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			}
		})

		It("should have OPENCODE_PERMISSION set to allow-all on server pod", func() {
			By("Verifying server Deployment env vars")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())

			serverContainer := deployment.Spec.Template.Spec.Containers[0]
			envMap := make(map[string]string)
			for _, env := range serverContainer.Env {
				envMap[env.Name] = env.Value
			}

			// Config has no "permission" field, so OPENCODE_PERMISSION should be set
			Expect(envMap).Should(HaveKey("OPENCODE_PERMISSION"))
			Expect(envMap["OPENCODE_PERMISSION"]).Should(Equal(`{"*":"allow"}`))

			// Config should point to the config file
			Expect(envMap).Should(HaveKey("OPENCODE_CONFIG"))

			// SCC compatibility
			Expect(envMap).Should(HaveKey("HOME"))
			Expect(envMap).Should(HaveKey("SHELL"))
		})

		It("should execute a simple task to completion", func() {
			taskName := uniqueName("oc-task")
			description := "What is 2 + 2? Reply with just the number, nothing else."

			By("Creating Task")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task Pod to be created")
			var pod *corev1.Pod
			Eventually(func() bool {
				var err error
				pod, err = getPodForTask(ctx, testNS, taskName)
				return err == nil && pod != nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Task Pod uses --attach command")
			agentContainer := pod.Spec.Containers[0]
			Expect(agentContainer.Command).Should(HaveLen(3))
			Expect(agentContainer.Command[2]).Should(ContainSubstring("--attach"))
			Expect(agentContainer.Command[2]).Should(ContainSubstring(fmt.Sprintf("%s.%s.svc", agentName, testNS)))

			By("Waiting for Task to complete (with extended timeout for AI execution)")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() string {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return string(t.Status.Phase)
			}, serverTimeout, interval).Should(Equal(string(kubeopenv1alpha1.TaskPhaseCompleted)),
				"Task should complete successfully with free model")

			By("Verifying Task Pod logs contain OpenCode output")
			logs := getPodLogs(ctx, testNS, pod.Name)
			if logs != "" {
				GinkgoWriter.Printf("Task Pod logs:\n%s\n", logs)
				// OpenCode output should contain model info
				Expect(logs).Should(ContainSubstring("big-pickle"))
			}

			By("Cleaning up Task")
			Expect(k8sClient.Delete(ctx, &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: taskName, Namespace: testNS},
			})).Should(Succeed())
		})

		It("should handle multiple sequential tasks on the same server", func() {
			tasks := []struct {
				name        string
				description string
			}{
				{uniqueName("oc-multi-1"), "What is 1 + 1? Reply with just the number."},
				{uniqueName("oc-multi-2"), "What is 3 + 3? Reply with just the number."},
			}

			for _, tc := range tasks {
				By(fmt.Sprintf("Creating Task %s", tc.name))
				desc := tc.description
				task := &kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tc.name,
						Namespace: testNS,
					},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
						Description: &desc,
					},
				}
				Expect(k8sClient.Create(ctx, task)).Should(Succeed())

				By(fmt.Sprintf("Waiting for Task %s to complete", tc.name))
				taskKey := types.NamespacedName{Name: tc.name, Namespace: testNS}
				Eventually(func() bool {
					t := &kubeopenv1alpha1.Task{}
					if err := k8sClient.Get(ctx, taskKey, t); err != nil {
						return false
					}
					return t.Status.Phase == kubeopenv1alpha1.TaskPhaseCompleted ||
						t.Status.Phase == kubeopenv1alpha1.TaskPhaseFailed
				}, serverTimeout, interval).Should(BeTrue(),
					fmt.Sprintf("Task %s should reach terminal state", tc.name))

				By(fmt.Sprintf("Cleaning up Task %s", tc.name))
				Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			}

			By("Verifying server is still running after multiple tasks")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())
			Expect(deployment.Status.ReadyReplicas).Should(BeNumerically(">=", 1))
		})
	})
})
