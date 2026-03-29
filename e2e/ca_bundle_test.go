// Copyright Contributors to the KubeOpenCode project

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const testCACertPEM = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJALRiMLAh0KIQMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yNDAxMDEwMDAwMDBaFw0yNTAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96HtiVQJKzMRAHxHW/t
LHLnTDHMRVKKxCEpJ0bXaxURmC3OJfSyVnCuRMqPMy8F0fXBFqBVgbMjqVMVd7dV
AgMBAAEwDQYJKoZIhvcNAQELBQADQQBiDDmeGsmF2JJcKz5NLQYHJKGJ3WbaNqSG
0YQMKQ3wPSog44rJFighFqMFrXmnSIQsjiMFikNolNMV2M2NGkPT
-----END CERTIFICATE-----`

var _ = Describe("CA Bundle E2E Tests", Label(LabelAgent), func() {

	Context("Agent with CA bundle from ConfigMap", func() {
		It("should mount CA certificate and expose it via CUSTOM_CA_CERT_PATH", func() {
			agentName := uniqueName("ws-cabundle")
			taskName := uniqueName("task-cabundle")
			configMapName := uniqueName("ca-bundle")
			content := "# CA Bundle Test"

			By("Creating ConfigMap with test CA certificate PEM")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNS,
				},
				Data: map[string]string{
					"ca-bundle.crt": testCACertPEM,
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			By("Creating Agent with caBundle.configMapRef and a command that prints the CA file")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "cat $CUSTOM_CA_CERT_PATH && echo CA_BUNDLE_VERIFIED"},
					CABundle: &kubeopenv1alpha1.CABundleConfig{
						ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
							Name: configMapName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Creating Task using the Agent")
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

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying pod logs contain the CA certificate content and verification marker")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("CA_BUNDLE_VERIFIED"))
			Expect(logs).Should(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(logs).Should(ContainSubstring("END CERTIFICATE"))
			Expect(logs).Should(ContainSubstring("MIIBkTCB"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		})
	})

	Context("Agent with CA bundle env var verification", func() {
		It("should set CUSTOM_CA_CERT_PATH environment variable pointing to the mounted CA file", func() {
			agentName := uniqueName("ws-ca-envvar")
			taskName := uniqueName("task-ca-envvar")
			configMapName := uniqueName("ca-bundle-envvar")
			content := "# CA Bundle Env Var Test"

			By("Creating ConfigMap with test CA certificate PEM")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNS,
				},
				Data: map[string]string{
					"ca-bundle.crt": testCACertPEM,
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			By("Creating Agent with caBundle.configMapRef that prints the env var value")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo CUSTOM_CA_PATH=$CUSTOM_CA_CERT_PATH"},
					CABundle: &kubeopenv1alpha1.CABundleConfig{
						ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
							Name: configMapName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Creating Task using the Agent")
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

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying pod logs contain the expected CUSTOM_CA_CERT_PATH value")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("CUSTOM_CA_PATH=/etc/ssl/certs/custom-ca/tls.crt"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		})
	})

	Context("Agent without CA bundle (baseline)", func() {
		It("should complete tasks normally without CA bundle configuration", func() {
			agentName := uniqueName("ws-no-ca")
			taskName := uniqueName("task-no-ca")
			content := "# No CA Bundle Baseline Test"

			By("Creating Agent without caBundle")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo NO_CA_BUNDLE_CONFIGURED && echo BASELINE_VERIFIED"},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Creating Task using the Agent")
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

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying pod logs confirm normal execution without CA bundle")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("NO_CA_BUNDLE_CONFIGURED"))
			Expect(logs).Should(ContainSubstring("BASELINE_VERIFIED"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})
})
