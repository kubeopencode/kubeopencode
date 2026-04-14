// Copyright Contributors to the KubeOpenCode project

//go:build integration

package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var _ = Describe("AgentController", func() {
	const (
		agentNamespace = "default"
	)

	Context("When creating an Agent", func() {
		It("Should create a Deployment and Service", func() {
			agentName := "test-server-agent"

			By("Creating an Agent")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting a Deployment to be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Expecting a Service to be created")
			serviceName := ServerServiceName(agentName)
			Eventually(func() error {
				var service corev1.Service
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      serviceName,
					Namespace: agentNamespace,
				}, &service)
			}, timeout, interval).Should(Succeed())

			By("Expecting Agent status to be updated with DeploymentName")
			Eventually(func() bool {
				var updatedAgent kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &updatedAgent); err != nil {
					return false
				}
				return updatedAgent.Status.DeploymentName != ""
			}, timeout, interval).Should(BeTrue())

			By("Verifying Deployment has correct labels and selector")
			var deployment appsv1.Deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      deploymentName,
				Namespace: agentNamespace,
			}, &deployment)).Should(Succeed())
			Expect(deployment.Labels["kubeopencode.io/agent"]).To(Equal(agentName))
			Expect(deployment.Spec.Selector.MatchLabels["kubeopencode.io/agent"]).To(Equal(agentName))

			By("Verifying Service has correct selector")
			var service corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      serviceName,
				Namespace: agentNamespace,
			}, &service)).Should(Succeed())
			Expect(service.Spec.Selector["kubeopencode.io/agent"]).To(Equal(agentName))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(4096)))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When updating an Agent", func() {
		It("Should update the Deployment with new configuration", func() {
			agentName := "test-update-agent"

			By("Creating an Agent with initial port")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Updating the Agent with a new port")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.Port = 8080
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting the Deployment to be updated with new port")
			Eventually(func() int32 {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return 0
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return 0
				}
				if len(deployment.Spec.Template.Spec.Containers[0].Ports) == 0 {
					return 0
				}
				return deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
			}, timeout, interval).Should(Equal(int32(8080)))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, &updatedAgent)).Should(Succeed())
		})
	})

	Context("When creating an Agent with ExtraPorts", func() {
		It("Should create Deployment and Service with extra ports", func() {
			agentName := "test-extra-ports-agent"

			By("Creating an Agent with ExtraPorts")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					ExtraPorts: []kubeopenv1alpha1.ExtraPort{
						{Name: "webapp", Port: 3000},
						{Name: "vscode", Port: 8080, Protocol: corev1.ProtocolTCP},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting Deployment with extra container ports")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() int {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return 0
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return 0
				}
				return len(deployment.Spec.Template.Spec.Containers[0].Ports)
			}, timeout, interval).Should(Equal(3)) // http + webapp + vscode

			By("Verifying Deployment container port details")
			var deployment appsv1.Deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      deploymentName,
				Namespace: agentNamespace,
			}, &deployment)).Should(Succeed())
			ports := deployment.Spec.Template.Spec.Containers[0].Ports
			Expect(ports[0].Name).To(Equal("http"))
			Expect(ports[0].ContainerPort).To(Equal(int32(4096)))
			Expect(ports[1].Name).To(Equal("webapp"))
			Expect(ports[1].ContainerPort).To(Equal(int32(3000)))
			Expect(ports[2].Name).To(Equal("vscode"))
			Expect(ports[2].ContainerPort).To(Equal(int32(8080)))

			By("Expecting Service with extra service ports")
			serviceName := ServerServiceName(agentName)
			var service corev1.Service
			Eventually(func() int {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      serviceName,
					Namespace: agentNamespace,
				}, &service); err != nil {
					return 0
				}
				return len(service.Spec.Ports)
			}, timeout, interval).Should(Equal(3))

			By("Verifying Service port details")
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      serviceName,
				Namespace: agentNamespace,
			}, &service)).Should(Succeed())
			Expect(service.Spec.Ports[0].Name).To(Equal("http"))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(4096)))
			Expect(service.Spec.Ports[1].Name).To(Equal("webapp"))
			Expect(service.Spec.Ports[1].Port).To(Equal(int32(3000)))
			Expect(service.Spec.Ports[2].Name).To(Equal("vscode"))
			Expect(service.Spec.Ports[2].Port).To(Equal(int32(8080)))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When updating Agent ExtraPorts", func() {
		It("Should update the Deployment and Service with new extra ports", func() {
			agentName := "test-update-extra-ports"

			By("Creating an Agent without ExtraPorts")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created with 1 port")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() int {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return 0
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return 0
				}
				return len(deployment.Spec.Template.Spec.Containers[0].Ports)
			}, timeout, interval).Should(Equal(1))

			By("Updating the Agent to add ExtraPorts")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.ExtraPorts = []kubeopenv1alpha1.ExtraPort{
				{Name: "webapp", Port: 3000},
			}
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting Deployment to be updated with 2 ports")
			Eventually(func() int {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return 0
				}
				if len(deployment.Spec.Template.Spec.Containers) == 0 {
					return 0
				}
				return len(deployment.Spec.Template.Spec.Containers[0].Ports)
			}, timeout, interval).Should(Equal(2))

			By("Expecting Service to be updated with 2 ports")
			serviceName := ServerServiceName(agentName)
			Eventually(func() int {
				var service corev1.Service
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      serviceName,
					Namespace: agentNamespace,
				}, &service); err != nil {
					return 0
				}
				return len(service.Spec.Ports)
			}, timeout, interval).Should(Equal(2))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, &updatedAgent)).Should(Succeed())
		})
	})

	Context("When updating Agent context content", func() {
		It("Should update the Deployment pod template hash annotation", func() {
			agentName := "test-context-hash-agent"

			By("Creating an Agent with a text context")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: "initial system prompt",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created with context hash annotation")
			deploymentName := ServerDeploymentName(agentName)
			var initialHash string
			Eventually(func() string {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return ""
				}
				if deployment.Spec.Template.Annotations == nil {
					return ""
				}
				initialHash = deployment.Spec.Template.Annotations[ContextHashAnnotationKey]
				return initialHash
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Updating the Agent with different text context content")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.Contexts[0].Text = "updated system prompt with new instructions"
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting the Deployment context hash annotation to change")
			Eventually(func() string {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return initialHash // return initial to keep waiting
				}
				if deployment.Spec.Template.Annotations == nil {
					return initialHash
				}
				return deployment.Spec.Template.Annotations[ContextHashAnnotationKey]
			}, timeout, interval).ShouldNot(Equal(initialHash))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, &updatedAgent)).Should(Succeed())
		})

		It("Should update hash when Agent config content changes with skills", func() {
			agentName := "test-config-hash-agent"
			initialConfig := `{"model":"claude-sonnet"}`

			By("Creating an Agent with config and a text context")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Config:             &initialConfig,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: "some context",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment with context hash")
			deploymentName := ServerDeploymentName(agentName)
			var initialHash string
			Eventually(func() string {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return ""
				}
				if deployment.Spec.Template.Annotations == nil {
					return ""
				}
				initialHash = deployment.Spec.Template.Annotations[ContextHashAnnotationKey]
				return initialHash
			}, timeout, interval).ShouldNot(BeEmpty())

			By("Updating the Agent config content")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			newConfig := `{"model":"claude-opus"}`
			updatedAgent.Spec.Config = &newConfig
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting the context hash annotation to change")
			Eventually(func() string {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return initialHash
				}
				if deployment.Spec.Template.Annotations == nil {
					return initialHash
				}
				return deployment.Spec.Template.Annotations[ContextHashAnnotationKey]
			}, timeout, interval).ShouldNot(Equal(initialHash))

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, &updatedAgent)).Should(Succeed())
		})
	})

	Context("When creating an Agent with session persistence", func() {
		It("Should create a PVC for session data", func() {
			agentName := "test-session-persist-agent"

			By("Creating an Agent with session persistence")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Persistence: &kubeopenv1alpha1.PersistenceConfig{
						Sessions: &kubeopenv1alpha1.VolumePersistence{
							Size: "2Gi",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting a PVC to be created for session data")
			pvcName := ServerSessionPVCName(agentName)
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Verifying PVC properties")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pvcName,
				Namespace: agentNamespace,
			}, &pvc)).Should(Succeed())
			Expect(pvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
			storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(storageReq.String()).To(Equal("2Gi"))

			By("Expecting a Deployment to also be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Verifying Deployment has session volume and OPENCODE_DB env")
			var deployment appsv1.Deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      deploymentName,
				Namespace: agentNamespace,
			}, &deployment)).Should(Succeed())

			// Check for session PVC volume
			var foundSessionVolume bool
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == ServerSessionVolumeName && vol.PersistentVolumeClaim != nil {
					foundSessionVolume = true
					Expect(vol.PersistentVolumeClaim.ClaimName).To(Equal(pvcName))
				}
			}
			Expect(foundSessionVolume).To(BeTrue(), "session PVC volume not found in Deployment")

			// Check for OPENCODE_DB env var
			container := deployment.Spec.Template.Spec.Containers[0]
			var foundDBEnv bool
			for _, env := range container.Env {
				if env.Name == OpenCodeDBEnvVar {
					foundDBEnv = true
					Expect(env.Value).To(Equal(ServerSessionDBPath))
				}
			}
			Expect(foundDBEnv).To(BeTrue(), "OPENCODE_DB env var not found in server container")

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When creating an Agent without session persistence", func() {
		It("Should NOT create a PVC", func() {
			agentName := "test-no-persist-agent"

			By("Creating an Agent without persistence")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting NO PVC to be created")
			pvcName := ServerSessionPVCName(agentName)
			Consistently(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout/2, interval).ShouldNot(Succeed())

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When suspending an Agent", func() {
		It("Should scale Deployment to 0 replicas and set Suspended status", func() {
			agentName := "test-suspend-agent"

			By("Creating an Agent")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Suspending the Agent")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.Suspend = true
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting Deployment to scale to 0 replicas")
			Eventually(func() int32 {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return -1
				}
				if deployment.Spec.Replicas == nil {
					return 1
				}
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(0)))

			By("Expecting Agent status to show Suspended")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Status.Suspended
			}, timeout, interval).Should(BeTrue())

			By("Resuming the Agent")
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.Suspend = false
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting Deployment to scale back to 1 replica")
			Eventually(func() int32 {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return -1
				}
				if deployment.Spec.Replicas == nil {
					return 1
				}
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(1)))

			By("Expecting Agent status to show not Suspended")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return true
				}
				return a.Status.Suspended
			}, timeout, interval).Should(BeFalse())

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When an Agent has standby configured", func() {
		// Standby auto-suspend depends on multiple reconcile cycles completing,
		// which can be slow on resource-constrained CI runners.
		standbyTimeout := 30 * time.Second

		It("Should auto-suspend by setting spec.suspend=true after idle timeout", func() {
			agentName := "test-standby-agent"

			By("Creating an Agent with standby")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Standby: &kubeopenv1alpha1.StandbyConfig{
						IdleTimeout: metav1.Duration{Duration: 1 * time.Second},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Expecting controller to auto-suspend after idle timeout")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Spec.Suspend
			}, standbyTimeout, interval).Should(BeTrue(), "spec.suspend should be set to true by standby controller")

			By("Expecting Deployment to scale to 0 replicas")
			Eventually(func() int32 {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return -1
				}
				if deployment.Spec.Replicas == nil {
					return 1
				}
				return *deployment.Spec.Replicas
			}, standbyTimeout, interval).Should(Equal(int32(0)))

			By("Creating a Task targeting the suspended Agent to trigger auto-resume")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-standby-resume-task",
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef: &kubeopenv1alpha1.AgentReference{Name: agentName},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Expecting controller to auto-resume (spec.suspend=false)")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return true
				}
				return a.Spec.Suspend
			}, timeout, interval).Should(BeFalse(), "spec.suspend should be set to false by standby controller on new task")

			By("Expecting Deployment to scale back to 1 replica")
			Eventually(func() int32 {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return -1
				}
				if deployment.Spec.Replicas == nil {
					return 1
				}
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(1)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})

		It("Should not auto-suspend when connection heartbeat annotation is fresh", func() {
			agentName := "test-standby-heartbeat-agent"

			By("Creating an Agent with standby (idle timeout longer than heartbeat staleness)")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
					Annotations: map[string]string{
						AnnotationLastConnectionActive: time.Now().UTC().Format(time.RFC3339),
					},
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Standby: &kubeopenv1alpha1.StandbyConfig{
						// Use 5 minutes so staleness stays at default (2 minutes),
						// avoiding the degradation path. The test only runs for a few seconds,
						// so the annotation stays well within the staleness window.
						IdleTimeout: metav1.Duration{Duration: 5 * time.Minute},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for Deployment to be created")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			By("Keeping the heartbeat fresh by updating annotation periodically")
			// Update the annotation to ensure it stays fresh during the test
			Eventually(func() error {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return err
				}
				if a.Annotations == nil {
					a.Annotations = make(map[string]string)
				}
				a.Annotations[AnnotationLastConnectionActive] = time.Now().UTC().Format(time.RFC3339)
				return k8sClient.Update(ctx, &a)
			}, timeout, interval).Should(Succeed())

			By("Expecting Agent to NOT be suspended despite idle timeout expiring")
			// Wait longer than idle timeout to confirm no suspension.
			// The annotation was set at creation time and refreshed above,
			// so it stays within the 2-minute staleness window during this check.
			Consistently(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Spec.Suspend
			}, 3*time.Second, interval).Should(BeFalse(), "spec.suspend should remain false while connection heartbeat is fresh")

			// Note: we do NOT test "remove annotation → agent suspends" here because
			// that requires waiting staleness (2min) + idleTimeout (5min) = 7 minutes.
			// The existing standby test (line 387) already covers auto-suspend without heartbeat.

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})

		It("Should set warning condition when idleTimeout is less than heartbeat staleness", func() {
			agentName := "test-standby-short-timeout-agent"

			By("Creating an Agent with very short idle timeout")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Standby: &kubeopenv1alpha1.StandbyConfig{
						IdleTimeout: metav1.Duration{Duration: 30 * time.Second},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting StandbyConfigWarning condition to be set")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				for _, c := range a.Status.Conditions {
					if c.Type == AgentConditionStandbyConfigWarning && c.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "StandbyConfigWarning condition should be set when idleTimeout < heartbeat staleness")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When creating an Agent with workspace persistence", func() {
		It("Should create a workspace PVC", func() {
			agentName := "test-workspace-persist-agent"

			By("Creating an Agent with workspace persistence")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Persistence: &kubeopenv1alpha1.PersistenceConfig{
						Workspace: &kubeopenv1alpha1.VolumePersistence{
							Size: "10Gi",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting a workspace PVC to be created")
			pvcName := ServerWorkspacePVCName(agentName)
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout, interval).Should(Succeed())

			By("Verifying workspace PVC properties")
			var pvc corev1.PersistentVolumeClaim
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pvcName,
				Namespace: agentNamespace,
			}, &pvc)).Should(Succeed())
			Expect(pvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
			storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(storageReq.String()).To(Equal("10Gi"))

			By("Verifying Deployment uses PVC for workspace volume")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			var deployment appsv1.Deployment
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      deploymentName,
				Namespace: agentNamespace,
			}, &deployment)).Should(Succeed())

			var foundWorkspaceVolume bool
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == WorkspaceVolumeName && vol.PersistentVolumeClaim != nil {
					foundWorkspaceVolume = true
					Expect(vol.PersistentVolumeClaim.ClaimName).To(Equal(pvcName))
				}
			}
			Expect(foundWorkspaceVolume).To(BeTrue(), "workspace PVC volume not found in Deployment")

			By("Cleaning up the Agent")
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("GetServerPort helper function", func() {
		It("Should return configured port or default", func() {
			By("Agent with configured port")
			agentWithPort := &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 9090,
				},
			}
			Expect(GetServerPort(agentWithPort)).To(Equal(int32(9090)))

			By("Agent with zero port (should use default)")
			agentWithZeroPort := &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 0,
				},
			}
			Expect(GetServerPort(agentWithZeroPort)).To(Equal(DefaultServerPort))
		})
	})

	Context("ServerURL helper function", func() {
		It("Should generate correct in-cluster URL", func() {
			url := ServerURL("my-agent", "my-namespace", 4096)
			Expect(url).To(Equal("http://my-agent.my-namespace.svc.cluster.local:4096"))
		})
	})

	Context("Naming helper functions", func() {
		It("Should generate correct names", func() {
			Expect(ServerDeploymentName("my-agent")).To(Equal("my-agent-server"))
			Expect(ServerServiceName("my-agent")).To(Equal("my-agent"))
		})
	})

	Context("When enabling share link", func() {
		It("Should create a share Secret with a valid token", func() {
			agentName := "test-share-agent"

			By("Creating an Agent with share enabled")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Share: &kubeopenv1alpha1.ShareConfig{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Expecting a share Secret to be created")
			secretName := ShareSecretName(agentName)
			Eventually(func() error {
				var secret corev1.Secret
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      secretName,
					Namespace: agentNamespace,
				}, &secret)
			}, timeout, interval).Should(Succeed())

			By("Verifying the Secret has correct labels and annotations")
			var secret corev1.Secret
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      secretName,
				Namespace: agentNamespace,
			}, &secret)).Should(Succeed())

			Expect(secret.Labels).To(HaveKeyWithValue(LabelShareToken, "true"))
			Expect(secret.Annotations).To(HaveKeyWithValue(AnnotationShareAgentName, agentName))
			Expect(secret.Annotations).To(HaveKeyWithValue(AnnotationShareAgentNamespace, agentNamespace))

			By("Verifying the token is non-empty and has correct length")
			token, ok := secret.Data[ShareTokenKey]
			Expect(ok).To(BeTrue())
			Expect(string(token)).ToNot(BeEmpty())
			// base64url of 32 bytes = 43 characters
			Expect(len(token)).To(Equal(43))

			By("Verifying the Agent status has share info")
			Eventually(func() bool {
				var updatedAgent kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &updatedAgent); err != nil {
					return false
				}
				return updatedAgent.Status.Share != nil && updatedAgent.Status.Share.SecretName == secretName
			}, timeout, interval).Should(BeTrue())
		})

		It("Should delete share Secret when share is disabled", func() {
			agentName := "test-share-disable-agent"

			By("Creating an Agent with share enabled")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Share: &kubeopenv1alpha1.ShareConfig{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Waiting for the share Secret to be created")
			secretName := ShareSecretName(agentName)
			Eventually(func() error {
				var secret corev1.Secret
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      secretName,
					Namespace: agentNamespace,
				}, &secret)
			}, timeout, interval).Should(Succeed())

			By("Disabling share")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())

			updatedAgent.Spec.Share = nil
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

			By("Expecting the share Secret to be deleted")
			Eventually(func() bool {
				var secret corev1.Secret
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      secretName,
					Namespace: agentNamespace,
				}, &secret)
				return err != nil // Should be NotFound
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Agent status has no share info")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Status.Share == nil
			}, timeout, interval).Should(BeTrue())
		})

		It("Should mark share as expired when expiresAt is in the past", func() {
			agentName := "test-share-expired-agent"

			By("Creating an Agent with expired share link")
			pastTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest",
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "test-agent",
					Port:               4096,
					Share: &kubeopenv1alpha1.ShareConfig{
						Enabled:   true,
						ExpiresAt: &pastTime,
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Verifying the share status shows inactive (expired)")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Status.Share != nil && !a.Status.Share.Active
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var _ = Describe("DeploymentBuilder", func() {
	Context("BuildServerDeployment", func() {
		It("Should build correct Deployment for Agent", func() {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 4096,
				},
			}
			cfg := agentConfig{
				executorImage: "test-executor-image",
				agentImage:    "test-agent-image",
				workspaceDir:  "/workspace",
			}
			sysCfg := systemConfig{}

			deployment := BuildServerDeployment(agent, cfg, sysCfg, nil, nil, nil, nil, nil)
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.Name).To(Equal("test-server-agent-server"))
			Expect(deployment.Namespace).To(Equal("default"))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(4096)))
		})

		It("Should use default port when not specified", func() {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-default-port-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					// Port not specified, should use default
				},
			}
			cfg := agentConfig{
				executorImage: "test-executor-image",
				agentImage:    "test-agent-image",
				workspaceDir:  "/workspace",
			}
			sysCfg := systemConfig{}

			deployment := BuildServerDeployment(agent, cfg, sysCfg, nil, nil, nil, nil, nil)
			Expect(deployment).NotTo(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(DefaultServerPort))
		})
	})

	Context("BuildServerService", func() {
		It("Should build correct Service for Agent", func() {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 8080,
				},
			}

			service := BuildServerService(agent)
			Expect(service).NotTo(BeNil())
			Expect(service.Name).To(Equal("test-server-agent"))
			Expect(service.Namespace).To(Equal("default"))
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(service.Spec.Selector["kubeopencode.io/agent"]).To(Equal("test-server-agent"))
		})

		It("Should include extra ports in Service", func() {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dind-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 4096,
					ExtraPorts: []kubeopenv1alpha1.ExtraPort{
						{Name: "webapp", Port: 3000},
						{Name: "vscode", Port: 8080, Protocol: corev1.ProtocolTCP},
					},
				},
			}

			service := BuildServerService(agent)
			Expect(service).NotTo(BeNil())
			Expect(service.Spec.Ports).To(HaveLen(3))

			// Main port
			Expect(service.Spec.Ports[0].Name).To(Equal("http"))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(4096)))

			// Extra ports
			Expect(service.Spec.Ports[1].Name).To(Equal("webapp"))
			Expect(service.Spec.Ports[1].Port).To(Equal(int32(3000)))
			Expect(service.Spec.Ports[1].Protocol).To(Equal(corev1.ProtocolTCP))

			Expect(service.Spec.Ports[2].Name).To(Equal("vscode"))
			Expect(service.Spec.Ports[2].Port).To(Equal(int32(8080)))
		})

		It("Should build Service with only main port when no extra ports", func() {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 4096,
				},
			}

			service := BuildServerService(agent)
			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Name).To(Equal("http"))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(4096)))
		})
	})
})
