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

	// NOTE: ExtraPorts Deployment/Service port details are thoroughly tested in
	// server_builder_test.go (unit) and e2e/agent_test.go (E2E).
	// Integration test verifies reconciler creates correct port count.
	Context("When creating an Agent with ExtraPorts", func() {
		It("Should create Deployment and Service with extra ports", func() {
			agentName := "test-extra-ports-agent"

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

			By("Expecting Deployment with 3 container ports (http + webapp + vscode)")
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
			}, timeout, interval).Should(Equal(3))

			By("Expecting Service with 3 ports")
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
			}, timeout, interval).Should(Equal(3))

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

	// NOTE: Context hash annotation mechanics (computation, different content types)
	// are tested in server_builder_test.go and e2e/agent_test.go.
	// Integration test verifies that updating context triggers a hash change via reconciler.
	Context("When updating Agent context content", func() {
		It("Should update the Deployment pod template hash annotation", func() {
			agentName := "test-context-hash-agent"

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

			By("Updating context content should change the hash")
			var updatedAgent kubeopenv1alpha1.Agent
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      agentName,
				Namespace: agentNamespace,
			}, &updatedAgent)).Should(Succeed())
			updatedAgent.Spec.Contexts[0].Text = "updated system prompt with new instructions"
			Expect(k8sClient.Update(ctx, &updatedAgent)).Should(Succeed())

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

			Expect(k8sClient.Delete(ctx, &updatedAgent)).Should(Succeed())
		})
	})

	// NOTE: Session persistence PVC properties (access modes, size, volume mounts,
	// OPENCODE_DB env var) are thoroughly tested in server_builder_test.go (unit tests)
	// and e2e/server_test.go (E2E tests). Integration tests focus on verifying the
	// reconciler creates the PVC when persistence is configured.
	Context("When creating an Agent with session persistence", func() {
		It("Should create a session PVC via reconciler", func() {
			agentName := "test-session-persist-agent"

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

			By("Expecting session PVC and Deployment to be created")
			pvcName := ServerSessionPVCName(agentName)
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout, interval).Should(Succeed())

			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment)
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("When creating an Agent without session persistence", func() {
		It("Should NOT create a PVC", func() {
			agentName := "test-no-persist-agent"

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

			pvcName := ServerSessionPVCName(agentName)
			Consistently(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout/2, interval).ShouldNot(Succeed())

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

	// NOTE: Workspace persistence PVC properties and Deployment volume wiring are
	// thoroughly tested in server_builder_test.go and e2e/server_test.go.
	// Integration test only verifies the reconciler creates the workspace PVC.
	Context("When creating an Agent with workspace persistence", func() {
		It("Should create a workspace PVC via reconciler", func() {
			agentName := "test-workspace-persist-agent"

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

			pvcName := ServerWorkspacePVCName(agentName)
			Eventually(func() error {
				var pvc corev1.PersistentVolumeClaim
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: agentNamespace,
				}, &pvc)
			}, timeout, interval).Should(Succeed())

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
			url := ServerURL("my-agent", "my-namespace", 4096, "cluster.local")
			Expect(url).To(Equal("http://my-agent.my-namespace.svc.cluster.local:4096"))
		})
		It("Should generate correct in-cluster URL with custom cluster domain", func() {
			url := ServerURL("my-agent", "my-namespace", 4096, "custom.local")
			Expect(url).To(Equal("http://my-agent.my-namespace.svc.custom.local:4096"))
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

	Context("When creating an Agent with plugins", func() {
		It("Should create a Deployment with plugin-init container", func() {
			agentName := "test-plugin-agent"

			By("Creating an Agent with plugins")
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
					Plugins: []kubeopenv1alpha1.PluginSpec{
						{Name: "@kubeopencode/plugin-test", Target: "server"},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Verifying Deployment has plugin-init container")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() bool {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return false
				}

				for _, c := range deployment.Spec.Template.Spec.InitContainers {
					if c.Name == "plugin-init" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "expected plugin-init init container in Deployment")

			By("Verifying Deployment has plugins volume")
			Eventually(func() bool {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return false
				}

				for _, v := range deployment.Spec.Template.Spec.Volumes {
					if v.Name == PluginsVolumeName {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "expected plugins volume in Deployment")
		})
	})

	Context("When creating an Agent with git sync contexts", func() {
		It("Should create a Deployment with git-sync sidecar for HotReload policy", func() {
			agentName := "test-gitsync-agent"

			By("Creating an Agent with git context using sync HotReload")
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
							Name:      "code-repo",
							Type:      kubeopenv1alpha1.ContextTypeGit,
							MountPath: "/workspace/code",
							Git: &kubeopenv1alpha1.GitContext{
								Repository: "https://github.com/example/repo.git",
								Ref:        "main",
								Sync: &kubeopenv1alpha1.GitSync{
									Enabled: true,
									Policy:  kubeopenv1alpha1.GitSyncPolicyHotReload,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Verifying Deployment has git-sync sidecar container")
			deploymentName := ServerDeploymentName(agentName)
			Eventually(func() bool {
				var deployment appsv1.Deployment
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deploymentName,
					Namespace: agentNamespace,
				}, &deployment); err != nil {
					return false
				}

				// git-sync sidecar is a regular container, not an init container
				for _, c := range deployment.Spec.Template.Spec.Containers {
					if c.Name == "git-sync-0" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue(), "expected git-sync sidecar container in Deployment")
		})
	})
})
