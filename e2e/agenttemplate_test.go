// Copyright Contributors to the KubeOpenCode project

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const LabelAgentTemplate = "agenttemplate"

var _ = Describe("AgentTemplate E2E Tests", Label(LabelAgentTemplate), func() {

	Context("AgentTemplate lifecycle", func() {
		It("should reconcile status with ObservedGeneration and Ready condition", func() {
			tmplName := uniqueName("tmpl-lifecycle")

			By("Creating AgentTemplate")
			tmpl := &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmplName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
				},
			}
			Expect(k8sClient.Create(ctx, tmpl)).Should(Succeed())

			By("Verifying ObservedGeneration is set")
			tmplKey := types.NamespacedName{Name: tmplName, Namespace: testNS}
			Eventually(func() int64 {
				t := &kubeopenv1alpha1.AgentTemplate{}
				if err := k8sClient.Get(ctx, tmplKey, t); err != nil {
					return 0
				}
				return t.Status.ObservedGeneration
			}, timeout, interval).Should(BeNumerically(">", 0))

			By("Verifying Ready condition is True")
			Eventually(func() string {
				t := &kubeopenv1alpha1.AgentTemplate{}
				if err := k8sClient.Get(ctx, tmplKey, t); err != nil {
					return ""
				}
				for _, c := range t.Status.Conditions {
					if c.Type == "Ready" {
						return string(c.Status)
					}
				}
				return ""
			}, timeout, interval).Should(Equal("True"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, tmpl)).Should(Succeed())
		})
	})

	Context("Agent inherits template configuration", func() {
		It("should use template values when Agent does not override", func() {
			tmplName := uniqueName("tmpl-inherit")
			agentName := uniqueName("agent-inherit")
			taskName := uniqueName("task-inherit")

			By("Creating AgentTemplate with specific executor image and command")
			tmpl := &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmplName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo 'TEMPLATE_INHERITED' && cat ${WORKSPACE_DIR}/task.md && echo '=== Task Completed ==='"},
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Name: "template-context",
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: "Context from template",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, tmpl)).Should(Succeed())

			By("Creating Agent referencing the template")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: tmplName},
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Verifying agent-template label is set on Agent")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() string {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return ""
				}
				return a.Labels["kubeopencode.io/agent-template"]
			}, timeout, interval).Should(Equal(tmplName))

			By("Creating Task to verify template command is inherited")
			taskContent := "Test task content"
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &taskContent,
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

			By("Verifying Pod used the template command (TEMPLATE_INHERITED in logs)")
			pod, err := getPodForTask(ctx, testNS, taskName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())
			// Verify the pod's command came from the template
			agentContainer := pod.Spec.Containers[0]
			Expect(agentContainer.Command).To(ContainElement("sh"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, tmpl)).Should(Succeed())
		})
	})

	Context("Agent overrides template fields", func() {
		It("should use Agent values when Agent specifies its own fields", func() {
			tmplName := uniqueName("tmpl-override")
			agentName := uniqueName("agent-override")
			taskName := uniqueName("task-override")

			By("Creating AgentTemplate with a command")
			tmpl := &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmplName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo 'FROM_TEMPLATE' && echo '=== Task Completed ==='"},
				},
			}
			Expect(k8sClient.Create(ctx, tmpl)).Should(Succeed())

			By("Creating Agent with its own command (overrides template)")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: tmplName},
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo 'FROM_AGENT' && cat ${WORKSPACE_DIR}/task.md && echo '=== Task Completed ==='"},
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Creating Task")
			taskContent := "Override test"
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &taskContent,
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

			By("Verifying Pod used Agent's command (FROM_AGENT), not template's")
			pod, err := getPodForTask(ctx, testNS, taskName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())
			// The agent's command should have been used
			agentContainer := pod.Spec.Containers[0]
			found := false
			for _, arg := range agentContainer.Command {
				if arg == "sh" {
					found = true
				}
			}
			Expect(found).To(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, tmpl)).Should(Succeed())
		})
	})

	Context("Multiple Agents from same template", func() {
		It("should allow multiple agents to reference the same template", func() {
			tmplName := uniqueName("tmpl-multi")
			agent1Name := uniqueName("agent-multi-1")
			agent2Name := uniqueName("agent-multi-2")

			By("Creating AgentTemplate")
			tmpl := &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmplName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo '=== Task Completed ==='"},
				},
			}
			Expect(k8sClient.Create(ctx, tmpl)).Should(Succeed())

			By("Creating two Agents referencing the same template")
			for _, name := range []string{agent1Name, agent2Name} {
				agent := &kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: testNS,
					},
					Spec: kubeopenv1alpha1.AgentSpec{
						TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: tmplName},
						ExecutorImage:      echoImage,
						ServiceAccountName: testServiceAccount,
						WorkspaceDir:       "/workspace",
						Profile:            fmt.Sprintf("Agent %s using template %s", name, tmplName),
					},
				}
				Expect(k8sClient.Create(ctx, agent)).Should(Succeed())
			}

			By("Verifying both agents have the template label")
			for _, name := range []string{agent1Name, agent2Name} {
				agentKey := types.NamespacedName{Name: name, Namespace: testNS}
				Eventually(func() string {
					a := &kubeopenv1alpha1.Agent{}
					if err := k8sClient.Get(ctx, agentKey, a); err != nil {
						return ""
					}
					return a.Labels["kubeopencode.io/agent-template"]
				}, timeout, interval).Should(Equal(tmplName))
			}

			By("Verifying agents can be listed by template label")
			var agentList kubeopenv1alpha1.AgentList
			Expect(k8sClient.List(ctx, &agentList,
				client.InNamespace(testNS),
				client.MatchingLabels{"kubeopencode.io/agent-template": tmplName},
			)).Should(Succeed())
			Expect(agentList.Items).Should(HaveLen(2))

			By("Cleaning up")
			for _, name := range []string{agent1Name, agent2Name} {
				agent := &kubeopenv1alpha1.Agent{}
				agent.Name = name
				agent.Namespace = testNS
				Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
			}
			Expect(k8sClient.Delete(ctx, tmpl)).Should(Succeed())
		})
	})

	Context("Missing template reference", func() {
		It("should fail task when referenced template does not exist", func() {
			agentName := uniqueName("agent-notmpl")
			taskName := uniqueName("task-notmpl")

			By("Creating Agent referencing a non-existent template")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: "nonexistent-template"},
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Creating Task")
			taskContent := "Should fail"
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &taskContent,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Verifying Task fails with template not found error")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseFailed))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, agent)).Should(Succeed())
		})
	})

	Context("Template label cleanup", func() {
		It("should remove template label when templateRef is removed from Agent", func() {
			tmplName := uniqueName("tmpl-cleanup")
			agentName := uniqueName("agent-cleanup")

			By("Creating AgentTemplate")
			tmpl := &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tmplName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
				},
			}
			Expect(k8sClient.Create(ctx, tmpl)).Should(Succeed())

			By("Creating Agent with templateRef")
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      agentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: tmplName},
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
				},
			}
			Expect(k8sClient.Create(ctx, agent)).Should(Succeed())

			By("Verifying template label is set")
			agentKey := types.NamespacedName{Name: agentName, Namespace: testNS}
			Eventually(func() string {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return ""
				}
				return a.Labels["kubeopencode.io/agent-template"]
			}, timeout, interval).Should(Equal(tmplName))

			By("Removing templateRef from Agent")
			Eventually(func() error {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return err
				}
				a.Spec.TemplateRef = nil
				return k8sClient.Update(ctx, a)
			}, timeout, interval).Should(Succeed())

			By("Verifying template label is removed")
			Eventually(func() bool {
				a := &kubeopenv1alpha1.Agent{}
				if err := k8sClient.Get(ctx, agentKey, a); err != nil {
					return false
				}
				_, hasLabel := a.Labels["kubeopencode.io/agent-template"]
				return !hasLabel
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, &kubeopenv1alpha1.Agent{ObjectMeta: metav1.ObjectMeta{Name: agentName, Namespace: testNS}})).Should(Succeed())
			Expect(k8sClient.Delete(ctx, tmpl)).Should(Succeed())
		})
	})
})
