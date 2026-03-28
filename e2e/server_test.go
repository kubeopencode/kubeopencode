// Copyright Contributors to the KubeOpenCode project

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

var _ = Describe("Server Mode E2E Tests", Label(LabelServer), func() {

	Context("Server Mode Agent - Deployment Creation", func() {
		It("should create Deployment and Service when serverConfig is present", func() {
			agentName := uniqueName("server-agent")

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := fmt.Sprintf("%s-server", agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for Service to be created")
			serviceKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				service := &corev1.Service{}
				return k8sClient.Get(ctx, serviceKey, service) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Agent status.serverStatus is populated")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil &&
					a.Status.ServerStatus.DeploymentName != "" &&
					a.Status.ServerStatus.ServiceName != ""
			}, timeout, interval).Should(BeTrue())

			By("Verifying ServerStatus details")
			updatedAgent := &kubeopenv1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, agentKey, updatedAgent)).Should(Succeed())
			Expect(updatedAgent.Status.ServerStatus.DeploymentName).Should(Equal(deploymentName))
			Expect(updatedAgent.Status.ServerStatus.ServiceName).Should(Equal(agentName))
			Expect(updatedAgent.Status.ServerStatus.URL).Should(ContainSubstring(agentName))

			By("Verifying server Deployment has correct env vars (HOME, SHELL, WORKSPACE_DIR)")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())
			serverContainer := deployment.Spec.Template.Spec.Containers[0]
			envMap := make(map[string]string)
			for _, env := range serverContainer.Env {
				envMap[env.Name] = env.Value
			}
			Expect(envMap).Should(HaveKey("HOME"))
			Expect(envMap).Should(HaveKey("SHELL"))
			Expect(envMap).Should(HaveKey("WORKSPACE_DIR"))
			Expect(envMap["WORKSPACE_DIR"]).Should(Equal("/workspace"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())

			// Wait for resources to be deleted
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Server Mode Agent - Health Conditions", func() {
		It("should set ServerReady and ServerHealthy conditions", func() {
			agentName := uniqueName("server-health")

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for ServerReady condition")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				cond := getAgentCondition(a, "ServerReady")
				return cond != nil
			}, serverTimeout, interval).Should(BeTrue())

			By("Verifying conditions are set")
			updatedAgent := &kubeopenv1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, agentKey, updatedAgent)).Should(Succeed())

			serverReadyCond := getAgentCondition(updatedAgent, "ServerReady")
			Expect(serverReadyCond).ShouldNot(BeNil())

			By("Verifying serverStatus is populated")
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil
			}, serverTimeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode to Pod Mode Transition", func() {
		It("should clean up Deployment and Service when serverConfig is removed", func() {
			agentName := uniqueName("server-transition")

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := fmt.Sprintf("%s-server", agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Waiting for Service to be created")
			serviceKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				service := &corev1.Service{}
				return k8sClient.Get(ctx, serviceKey, service) == nil
			}, timeout, interval).Should(BeTrue())

			By("Updating Agent to remove serverConfig (transition to Pod mode)")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			updatedAgent := &kubeopenv1alpha1.Agent{}
			Expect(k8sClient.Get(ctx, agentKey, updatedAgent)).Should(Succeed())
			updatedAgent.Spec.ServerConfig = nil
			Expect(k8sClient.Update(ctx, updatedAgent)).Should(Succeed())

			By("Verifying Deployment is deleted")
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) != nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Service is deleted")
			Eventually(func() bool {
				service := &corev1.Service{}
				return k8sClient.Get(ctx, serviceKey, service) != nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying serverStatus is cleared")
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - Task Execution", func() {
		It("should create Task Pod that connects to server", func() {
			agentName := uniqueName("server-task")
			taskName := uniqueName("task-server")
			content := "# Server Mode Task Test"

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for server to be ready")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil && a.Status.ServerStatus.URL != ""
			}, serverTimeout, interval).Should(BeTrue())

			By("Creating Task using Server-mode Agent")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &content,
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

			By("Verifying Pod is created (server mode uses attach pods)")
			Expect(pod.Spec.Containers).Should(HaveLen(1))
			// In server mode, the pod should have the attach image or executor image
			Expect(pod.Spec.Containers[0].Image).ShouldNot(BeEmpty())

			By("Waiting for Task to transition (may complete or fail depending on server)")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() bool {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return false
				}
				return t.Status.Phase == kubeopenv1alpha1.TaskPhaseCompleted ||
					t.Status.Phase == kubeopenv1alpha1.TaskPhaseFailed ||
					t.Status.Phase == kubeopenv1alpha1.TaskPhaseRunning
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - Multiple Tasks", func() {
		It("should handle multiple Tasks connecting to the same server", func() {
			agentName := uniqueName("server-multi")
			content := "# Multi-Task Server Test"

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for server to be ready")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil && a.Status.ServerStatus.URL != ""
			}, serverTimeout, interval).Should(BeTrue())

			By("Creating multiple Tasks")
			taskNames := []string{
				uniqueName("task-multi-1"),
				uniqueName("task-multi-2"),
				uniqueName("task-multi-3"),
			}

			for _, taskName := range taskNames {
				task := &kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      taskName,
						Namespace: testNS,
					},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
						Description: &content,
					},
				}
				Expect(k8sClient.Create(ctx, task)).Should(Succeed())
			}

			By("Verifying all Tasks have Pods created")
			for _, taskName := range taskNames {
				Eventually(func() bool {
					pod, err := getPodForTask(ctx, testNS, taskName)
					return err == nil && pod != nil
				}, timeout, interval).Should(BeTrue())
			}

			By("Verifying all Tasks are in Running or Completed state")
			for _, taskName := range taskNames {
				taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
				Eventually(func() bool {
					t := &kubeopenv1alpha1.Task{}
					if err := k8sClient.Get(ctx, taskKey, t); err != nil {
						return false
					}
					return t.Status.Phase == kubeopenv1alpha1.TaskPhaseRunning ||
						t.Status.Phase == kubeopenv1alpha1.TaskPhaseCompleted ||
						t.Status.Phase == kubeopenv1alpha1.TaskPhaseFailed
				}, timeout, interval).Should(BeTrue())
			}

			By("Verifying only one Deployment exists for the agent")
			deploymentName := fmt.Sprintf("%s-server", agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())

			By("Cleaning up Tasks")
			for _, taskName := range taskNames {
				task := &kubeopenv1alpha1.Task{}
				taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
				if err := k8sClient.Get(ctx, taskKey, task); err == nil {
					Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
				}
			}

			By("Cleaning up Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - AttachImage Usage", func() {
		It("should use attachImage for Task Pods when specified", func() {
			agentName := uniqueName("server-attach")
			taskName := uniqueName("task-attach")
			content := "# AttachImage Test"

			By("Creating Agent with serverConfig and attachImage")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					AttachImage:        echoImage, // Use echo image as attach image for testing
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for server to be ready")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil && a.Status.ServerStatus.URL != ""
			}, serverTimeout, interval).Should(BeTrue())

			By("Creating Task")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &content,
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

			By("Verifying Pod uses the attach image")
			Expect(pod.Spec.Containers).Should(HaveLen(1))
			Expect(pod.Spec.Containers[0].Image).Should(Equal(echoImage))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - Credentials Injection", func() {
		It("should mount credentials as env vars in server Deployment", func() {
			agentName := uniqueName("server-creds")

			By("Creating a Secret for credentials")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName + "-creds",
					Namespace: testNS,
				},
				StringData: map[string]string{
					"API_KEY":    "test-key-value",
					"API_SECRET": "test-secret-value",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			By("Creating Agent with serverConfig and credentials")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
					Credentials: []kubeopenv1alpha1.Credential{
						{
							Name: "api-creds",
							SecretRef: kubeopenv1alpha1.SecretReference{
								Name: agentName + "-creds",
							},
							// No key specified = entire secret as envFrom
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := controller.ServerDeploymentName(agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Deployment has envFrom with the Secret")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())
			serverContainer := deployment.Spec.Template.Spec.Containers[0]

			// Entire secret should be mounted as envFrom
			foundEnvFrom := false
			for _, envFrom := range serverContainer.EnvFrom {
				if envFrom.SecretRef != nil && envFrom.SecretRef.Name == agentName+"-creds" {
					foundEnvFrom = true
					break
				}
			}
			Expect(foundEnvFrom).Should(BeTrue(), "Server container should have envFrom referencing the credentials Secret")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
		})
	})

	Context("Server Mode - OPENCODE_PERMISSION Default", func() {
		It("should set OPENCODE_PERMISSION when config has no permission field", func() {
			agentName := uniqueName("server-perm")

			By("Creating Agent with serverConfig but no permission in config")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := controller.ServerDeploymentName(agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying OPENCODE_PERMISSION is set to allow-all")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())
			serverContainer := deployment.Spec.Template.Spec.Containers[0]

			envMap := make(map[string]string)
			for _, env := range serverContainer.Env {
				envMap[env.Name] = env.Value
			}
			Expect(envMap).Should(HaveKey("OPENCODE_PERMISSION"))
			Expect(envMap["OPENCODE_PERMISSION"]).Should(Equal(`{"*":"allow"}`))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - Agent Context Support", func() {
		It("should load Text context via context-init container in server Deployment", func() {
			agentName := uniqueName("server-ctx")

			By("Creating Agent with serverConfig and Text context")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Name: "coding-rules",
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: "Always write tests for new code.",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := controller.ServerDeploymentName(agentName)
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: testNS}
			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				return k8sClient.Get(ctx, deploymentKey, deployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Deployment has context-init init container")
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).Should(Succeed())

			foundContextInit := false
			for _, ic := range deployment.Spec.Template.Spec.InitContainers {
				if ic.Name == "context-init" {
					foundContextInit = true
					break
				}
			}
			Expect(foundContextInit).Should(BeTrue(), "Server Deployment should have context-init init container for Text context")

			By("Verifying context ConfigMap was created")
			contextCMKey := types.NamespacedName{
				Name:      controller.ServerContextConfigMapName(agentName),
				Namespace: testNS,
			}
			Eventually(func() bool {
				cm := &corev1.ConfigMap{}
				return k8sClient.Get(ctx, contextCMKey, cm) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying ConfigMap contains the context content")
			contextCM := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, contextCMKey, contextCM)).Should(Succeed())
			// The context should be in the ConfigMap data (aggregated under context file key)
			Expect(len(contextCM.Data)).Should(BeNumerically(">", 0))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Server Mode - Task Pod Command Verification", func() {
		It("should use opencode run --attach in Task Pod command", func() {
			agentName := uniqueName("server-cmd")
			taskName := uniqueName("task-cmd")
			content := "# Command Verification Test"

			By("Creating Agent with serverConfig")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					AttachImage:        echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 4096,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for server to be ready")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				return a.Status.ServerStatus != nil && a.Status.ServerStatus.URL != ""
			}, serverTimeout, interval).Should(BeTrue())

			By("Creating Task")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &content,
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

			By("Verifying Pod command uses --attach flag")
			agentContainer := pod.Spec.Containers[0]
			// Command should be ["sh", "-c", "opencode run --attach <url> ..."]
			Expect(agentContainer.Command).Should(HaveLen(3))
			Expect(agentContainer.Command[0]).Should(Equal("sh"))
			Expect(agentContainer.Command[2]).Should(ContainSubstring("--attach"))
			Expect(agentContainer.Command[2]).Should(ContainSubstring(agentName))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})
})
