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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
		It("Should auto-suspend by setting spec.suspend=true after idle timeout", func() {
			agentName := "test-standby-agent"

			By("Creating an Agent with standby")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: agentNamespace,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
			}, timeout, interval).Should(BeTrue(), "spec.suspend should be set to true by standby controller")

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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
			// Wait longer than idle timeout to confirm no suspension
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

			By("Removing the heartbeat annotation to allow suspension")
			Eventually(func() error {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return err
				}
				delete(a.Annotations, AnnotationLastConnectionActive)
				return k8sClient.Update(ctx, &a)
			}, timeout, interval).Should(Succeed())

			By("Expecting controller to auto-suspend after heartbeat removed")
			Eventually(func() bool {
				var a kubeopenv1alpha1.Agent
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      agentName,
					Namespace: agentNamespace,
				}, &a); err != nil {
					return false
				}
				return a.Spec.Suspend
			}, timeout, interval).Should(BeTrue(), "spec.suspend should be true after heartbeat removed")

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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
					ExecutorImage:      "quay.io/kubeopencode/kubeopencode-agent-devbox:latest",
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
	})
})
