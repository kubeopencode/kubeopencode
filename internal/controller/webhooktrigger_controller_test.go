// Copyright Contributors to the KubeTask project

//go:build integration

// See suite_test.go for explanation of the "integration" build tag pattern.

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubetaskv1alpha1 "github.com/kubetask/kubetask/api/v1alpha1"
)

var _ = Describe("WebhookTriggerController", func() {
	const (
		namespace = "default"
	)

	Context("When creating a WebhookTrigger with legacy TaskTemplate", func() {
		It("Should update status with webhookURL and Ready condition", func() {
			triggerName := "test-wht-legacy"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					TaskTemplate: &kubetaskv1alpha1.WebhookTaskTemplate{
						AgentRef:    "default",
						Description: "Test task",
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Checking WebhookTrigger status is updated with webhookURL")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return ""
				}
				return t.Status.WebhookURL
			}, timeout, interval).Should(Equal("/webhooks/default/" + triggerName))

			By("Checking Ready condition is set")
			Eventually(func() bool {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return false
				}
				for _, c := range t.Status.Conditions {
					if c.Type == "Ready" && c.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When creating a WebhookTrigger with ResourceTemplate", func() {
		It("Should update status with webhookURL", func() {
			triggerName := "test-wht-resourcetpl"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					ResourceTemplate: &kubetaskv1alpha1.WebhookResourceTemplate{
						Task: &kubetaskv1alpha1.WebhookTaskSpec{
							AgentRef:    "default",
							Description: "Test task from resourceTemplate",
						},
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Checking WebhookTrigger status is updated with webhookURL")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return ""
				}
				return t.Status.WebhookURL
			}, timeout, interval).Should(Equal("/webhooks/default/" + triggerName))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When creating a WebhookTrigger with Rules", func() {
		It("Should update status with webhookURL and initialize RuleStatuses", func() {
			triggerName := "test-wht-rules"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					MatchPolicy: kubetaskv1alpha1.MatchPolicyFirst,
					Rules: []kubetaskv1alpha1.WebhookRule{
						{
							Name:   "rule-a",
							Filter: `body.action == "opened"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									AgentRef:    "default",
									Description: "Rule A task",
								},
							},
						},
						{
							Name:   "rule-b",
							Filter: `body.action == "closed"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									AgentRef:    "default",
									Description: "Rule B task",
								},
							},
						},
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Checking WebhookTrigger status is updated with webhookURL")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return ""
				}
				return t.Status.WebhookURL
			}, timeout, interval).Should(Equal("/webhooks/default/" + triggerName))

			By("Checking RuleStatuses is initialized")
			Eventually(func() int {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return 0
				}
				return len(t.Status.RuleStatuses)
			}, timeout, interval).Should(Equal(2))

			By("Verifying RuleStatuses contains both rules")
			t := &kubetaskv1alpha1.WebhookTrigger{}
			Expect(k8sClient.Get(ctx, triggerKey, t)).Should(Succeed())

			ruleNames := make(map[string]bool)
			for _, rs := range t.Status.RuleStatuses {
				ruleNames[rs.Name] = true
			}
			Expect(ruleNames).Should(HaveKey("rule-a"))
			Expect(ruleNames).Should(HaveKey("rule-b"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When WebhookTrigger has active Tasks", func() {
		It("Should track active Tasks in status", func() {
			triggerName := "test-wht-active"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					TaskTemplate: &kubetaskv1alpha1.WebhookTaskTemplate{
						AgentRef:    "default",
						Description: "Test task",
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Creating a Task with webhook-trigger label")
			taskDescription := "Task created by webhook"
			task := &kubetaskv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "webhook-created-task",
					Namespace: namespace,
					Labels: map[string]string{
						WebhookTriggerLabelKey: triggerName,
					},
				},
				Spec: kubetaskv1alpha1.TaskSpec{
					Description: &taskDescription,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Checking WebhookTrigger tracks the active Task")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() []string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return nil
				}
				return t.Status.ActiveResources
			}, timeout, interval).Should(ContainElement("webhook-created-task"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When WebhookTrigger has rules with active Tasks", func() {
		It("Should track active Tasks per rule in RuleStatuses", func() {
			triggerName := "test-wht-rules-active"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					MatchPolicy: kubetaskv1alpha1.MatchPolicyAll,
					Rules: []kubetaskv1alpha1.WebhookRule{
						{
							Name:   "pr-review",
							Filter: `body.action == "opened"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									AgentRef:    "default",
									Description: "PR Review task",
								},
							},
						},
						{
							Name:   "issue-triage",
							Filter: `body.action == "labeled"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									AgentRef:    "default",
									Description: "Issue triage task",
								},
							},
						},
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Creating a Task with webhook-trigger and webhook-rule labels")
			taskDescription := "Task created by pr-review rule"
			task := &kubetaskv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pr-review-task",
					Namespace: namespace,
					Labels: map[string]string{
						WebhookTriggerLabelKey: triggerName,
						WebhookRuleLabelKey:    "pr-review",
					},
				},
				Spec: kubetaskv1alpha1.TaskSpec{
					Description: &taskDescription,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Checking RuleStatuses tracks the active Task for pr-review rule")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() []string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return nil
				}
				for _, rs := range t.Status.RuleStatuses {
					if rs.Name == "pr-review" {
						return rs.ActiveResources
					}
				}
				return nil
			}, timeout, interval).Should(ContainElement("pr-review-task"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When WebhookTrigger uses WorkflowRef in ResourceTemplate", func() {
		It("Should update status correctly", func() {
			triggerName := "test-wht-workflowref"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					ResourceTemplate: &kubetaskv1alpha1.WebhookResourceTemplate{
						WorkflowRef: "my-workflow",
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Checking WebhookTrigger status is updated with webhookURL")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() string {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return ""
				}
				return t.Status.WebhookURL
			}, timeout, interval).Should(Equal("/webhooks/default/" + triggerName))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})

	Context("When WebhookTrigger has matchPolicy All", func() {
		It("Should initialize RuleStatuses for all rules", func() {
			triggerName := "test-wht-matchpolicy-all"

			trigger := &kubetaskv1alpha1.WebhookTrigger{
				ObjectMeta: metav1.ObjectMeta{
					Name:      triggerName,
					Namespace: namespace,
				},
				Spec: kubetaskv1alpha1.WebhookTriggerSpec{
					MatchPolicy: kubetaskv1alpha1.MatchPolicyAll,
					Rules: []kubetaskv1alpha1.WebhookRule{
						{
							Name:   "high-priority",
							Filter: `body.priority == "high"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									Description: "High priority handler",
								},
							},
						},
						{
							Name:   "bug-category",
							Filter: `body.category == "bug"`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									Description: "Bug handler",
								},
							},
						},
						{
							Name:   "security-label",
							Filter: `body.labels.exists(l, l == "security")`,
							ResourceTemplate: kubetaskv1alpha1.WebhookResourceTemplate{
								Task: &kubetaskv1alpha1.WebhookTaskSpec{
									Description: "Security handler",
								},
							},
						},
					},
				},
			}

			By("Creating the WebhookTrigger")
			Expect(k8sClient.Create(ctx, trigger)).Should(Succeed())

			By("Checking RuleStatuses contains all 3 rules")
			triggerKey := types.NamespacedName{Name: triggerName, Namespace: namespace}
			Eventually(func() int {
				t := &kubetaskv1alpha1.WebhookTrigger{}
				if err := k8sClient.Get(ctx, triggerKey, t); err != nil {
					return 0
				}
				return len(t.Status.RuleStatuses)
			}, timeout, interval).Should(Equal(3))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, trigger)).Should(Succeed())
		})
	})
})
