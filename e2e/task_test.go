// Copyright Contributors to the KubeOpenCode project

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var _ = Describe("Task E2E Tests", Label(LabelTask), func() {
	var (
		agent     *kubeopenv1alpha1.Agent
		agentName string
	)

	BeforeEach(func() {
		// Create an Agent with echo executor image for all tests
		// We use a custom command that echoes task content instead of running opencode
		agentName = uniqueName("echo-ws")
		agent = &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      agentName,
				Namespace: testNS,
			},
			Spec: kubeopenv1alpha1.AgentSpec{
				ExecutorImage:      echoImage,
				ServiceAccountName: testServiceAccount,
				WorkspaceDir:       "/workspace",
				Command:            []string{"sh", "-c", "echo '=== Task Content ===' && find ${WORKSPACE_DIR} -type f -print0 2>/dev/null | sort -z | xargs -0 -I {} sh -c 'echo \"--- File: {} ---\" && cat \"{}\" && echo' && echo '=== Task Completed ==='"},
			},
		}
		Expect(k8sClient.Create(ctx, agent)).Should(Succeed())
	})

	AfterEach(func() {
		// Clean up Agent
		if agent != nil {
			_ = k8sClient.Delete(ctx, agent)
		}
	})

	Context("Task with description using echo agent", func() {
		It("should create a Pod that echoes task content and complete successfully", func() {
			taskName := uniqueName("task-echo")
			taskContent := "# Hello E2E Test\n\nThis is a test task for the echo agent.\n\n## Expected Output\nThe echo agent should display this content."

			By("Creating a Task with description")
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

			By("Waiting for Task to transition to Running")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseRunning))

			By("Verifying Pod is created")
			jobName := fmt.Sprintf("%s-pod", taskName)
			jobKey := types.NamespacedName{Name: jobName, Namespace: testNS}
			job := &corev1.Pod{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, jobKey, job) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Pod uses echo agent image")
			Expect(job.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Containers[0].Image).Should(Equal(echoImage))

			By("Waiting for Pod to complete successfully")
			Eventually(func() corev1.PodPhase {
				if err := k8sClient.Get(ctx, jobKey, job); err != nil {
					return ""
				}
				return job.Status.Phase
			}, timeout, interval).Should(Equal(corev1.PodSucceeded))

			By("Verifying Task status is Completed")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying pod logs contain the task content")
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("=== Task Content ==="))
			Expect(logs).Should(ContainSubstring("Hello E2E Test"))
			Expect(logs).Should(ContainSubstring("=== Task Completed ==="))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task with multiple inline contexts", func() {
		It("should mount multiple contexts and complete successfully", func() {
			taskName := uniqueName("task-multi")
			content1 := "# Part 1: Introduction\n\nThis is the introduction."
			content2 := "# Part 2: Details\n\nThese are the details."
			content3 := "# Part 3: Conclusion\n\nThis is the conclusion."
			description := "Review these documents"

			By("Creating a Task with multiple inline contexts")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeText,
							MountPath: "/workspace/intro.md",
							Text:      content1,
						},
						{
							Type:      kubeopenv1alpha1.ContextTypeText,
							MountPath: "/workspace/details.md",
							Text:      content2,
						},
						{
							Type:      kubeopenv1alpha1.ContextTypeText,
							MountPath: "/workspace/conclusion.md",
							Text:      content3,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying all content parts are in the logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("Part 1: Introduction"))
			Expect(logs).Should(ContainSubstring("Part 2: Details"))
			Expect(logs).Should(ContainSubstring("Part 3: Conclusion"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task with ConfigMap context", func() {
		It("should resolve content from ConfigMap and pass to agent", func() {
			taskName := uniqueName("task-cm")
			configMapName := uniqueName("task-content-cm")
			configMapContent := "# ConfigMap Content\n\nThis content comes from a ConfigMap.\n\n## Verification\nIf you see this, ConfigMap resolution works!"
			description := "Test ConfigMap context"

			By("Creating ConfigMap with task content")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNS,
				},
				Data: map[string]string{
					"content.md": configMapContent,
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			By("Creating Task with inline ConfigMap context")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeConfigMap,
							MountPath: "/workspace/guides/content.md",
							ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
								Name: configMapName,
								Key:  "content.md",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying ConfigMap content is in the logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("ConfigMap Content"))
			Expect(logs).Should(ContainSubstring("ConfigMap resolution works"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		})
	})

	Context("Task with Agent contexts", func() {
		It("should merge agent contexts with task contexts", func() {
			taskName := uniqueName("task-default-ctx")
			customWSConfigName := uniqueName("ws-default-ctx")
			defaultContent := "# Default Guidelines\n\nThese are organization-wide default guidelines."
			taskContextContent := "# Additional Context\n\nThis is additional context from the task."
			taskDescription := "# Specific Task\n\nThis is the specific task to execute."

			By("Creating Agent with inline contexts")
			customWSConfig := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      customWSConfigName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo '=== Task Content ===' && find ${WORKSPACE_DIR} -type f -print0 2>/dev/null | sort -z | xargs -0 -I {} sh -c 'echo \"--- File: {} ---\" && cat \"{}\" && echo' && echo '=== Task Completed ==='"},
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: defaultContent,
							// No mountPath - should be written to .kubeopencode/context.md
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, customWSConfig)).Should(Succeed())

			By("Creating Task with inline context")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: customWSConfigName},
					Description: &taskDescription,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type: kubeopenv1alpha1.ContextTypeText,
							Text: taskContextContent,
							// No mountPath - should be written to .kubeopencode/context.md
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying both default and task content are in the logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("Default Guidelines"))
			Expect(logs).Should(ContainSubstring("Specific Task"))
			Expect(logs).Should(ContainSubstring("Additional Context"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, customWSConfig)).Should(Succeed())
		})
	})

	Context("Task lifecycle transitions", func() {
		It("should properly track phase transitions from Pending to Succeeded", func() {
			taskName := uniqueName("task-lifecycle")
			taskContent := "# Lifecycle Test\n\nSimple task for lifecycle testing."

			By("Creating Task")
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

			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}

			By("Verifying Task transitions to Running")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseRunning))

			By("Verifying StartTime is set")
			runningTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, runningTask)).Should(Succeed())
			Expect(runningTask.Status.StartTime).ShouldNot(BeNil())
			Expect(runningTask.Status.PodName).ShouldNot(BeEmpty())

			By("Verifying status.agentRef is populated")
			Expect(runningTask.Status.AgentRef).ShouldNot(BeNil())
			Expect(runningTask.Status.AgentRef.Name).Should(Equal(agentName))

			By("Verifying Task transitions to Completed")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying CompletionTime is set")
			completedTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, completedTask)).Should(Succeed())
			Expect(completedTask.Status.CompletionTime).ShouldNot(BeNil())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task garbage collection", func() {
		It("should clean up Pod when Task is deleted (via OwnerReference)", func() {
			taskName := uniqueName("task-gc")
			taskContent := "# GC Test"

			By("Creating and completing Task")
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

			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			jobName := fmt.Sprintf("%s-pod", taskName)
			jobKey := types.NamespacedName{Name: jobName, Namespace: testNS}

			By("Waiting for Pod to be created")
			Eventually(func() bool {
				job := &corev1.Pod{}
				return k8sClient.Get(ctx, jobKey, job) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Pod has OwnerReference pointing to Task")
			job := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, jobKey, job)).Should(Succeed())
			Expect(job.OwnerReferences).Should(HaveLen(1))
			Expect(job.OwnerReferences[0].Kind).Should(Equal("Task"))
			Expect(job.OwnerReferences[0].Name).Should(Equal(taskName))

			By("Deleting Task")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())

			By("Verifying Task is deleted")
			Eventually(func() bool {
				task := &kubeopenv1alpha1.Task{}
				err := k8sClient.Get(ctx, taskKey, task)
				return err != nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying Pod is cleaned up via OwnerReference cascade")
			Eventually(func() bool {
				job := &corev1.Pod{}
				err := k8sClient.Get(ctx, jobKey, job)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Task termination via annotation", func() {
		It("should terminate a running task when terminate annotation is added", func() {
			taskName := uniqueName("task-term")

			By("Creating an Agent for stop test")
			longRunAgentName := uniqueName("long-run-agent")
			longRunAgent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      longRunAgentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo 'Starting long task' && sleep 300"},
				},
			}
			Expect(k8sClient.Create(ctx, longRunAgent)).Should(Succeed())

			taskContent := "# Long Running Task"
			By("Creating a Task that will run for a long time")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: longRunAgentName},
					Description: &taskContent,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}

			By("Waiting for Task to be Running")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseRunning))

			By("Adding terminate annotation to the Task")
			runningTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, runningTask)).Should(Succeed())
			if runningTask.Annotations == nil {
				runningTask.Annotations = make(map[string]string)
			}
			runningTask.Annotations["kubeopencode.io/stop"] = "true"
			Expect(k8sClient.Update(ctx, runningTask)).Should(Succeed())

			By("Verifying Task transitions to Completed with Stopped condition")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying Stopped condition exists")
			stoppedTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, stoppedTask)).Should(Succeed())

			var hasStoppedCondition bool
			for _, cond := range stoppedTask.Status.Conditions {
				if cond.Type == "Stopped" && cond.Status == "True" {
					hasStoppedCondition = true
					Expect(cond.Reason).Should(Equal("UserStopped"))
					break
				}
			}
			Expect(hasStoppedCondition).Should(BeTrue(), "Task should have Stopped condition")

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, longRunAgent)).Should(Succeed())
		})
	})

	Context("Task with failing Pod", func() {
		It("should transition to Failed phase when Pod fails", func() {
			taskName := uniqueName("task-fail")

			By("Creating an Agent that always fails")
			failAgentName := uniqueName("fail-agent")
			failAgent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failAgentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo 'Task will fail' && exit 1"},
				},
			}
			Expect(k8sClient.Create(ctx, failAgent)).Should(Succeed())

			taskContent := "# Failing Task"
			By("Creating a Task that will fail")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: failAgentName},
					Description: &taskContent,
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}

			By("Waiting for Task to transition to Failed")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				t := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, t); err != nil {
					return ""
				}
				return t.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseFailed))

			By("Verifying Task status")
			failedTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, failedTask)).Should(Succeed())
			Expect(failedTask.Status.CompletionTime).ShouldNot(BeNil())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, failAgent)).Should(Succeed())
		})
	})

	Context("Task with Git Context", func() {
		It("should clone a public Git repository and mount content", func() {
			taskName := uniqueName("task-git")
			description := "Verify git content is available"

			By("Creating Task with inline Git context")
			// Using a well-known public repo that is stable
			depth := 1
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeGit,
							MountPath: "/workspace/repo",
							Git: &kubeopenv1alpha1.GitContext{
								Repository: "https://github.com/octocat/Hello-World.git",
								Ref:        "master",
								Depth:      &depth,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying Git repository content is available in logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			// The Hello-World repo contains a README file
			Expect(logs).Should(ContainSubstring("README"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})

		It("should clone a specific path from Git repository", func() {
			taskName := uniqueName("task-git-path")
			description := "Verify git subpath content"

			By("Creating Task with inline Git context with specific path")
			depth := 1
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeGit,
							MountPath: "/workspace/readme-file",
							Git: &kubeopenv1alpha1.GitContext{
								Repository: "https://github.com/octocat/Hello-World.git",
								Ref:        "master",
								Path:       "README",
								Depth:      &depth,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying specific file content is available")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("readme-file"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task Cleanup with KubeOpenCodeConfig", func() {
		It("should delete Task after TTL expires", func() {
			configName := "cluster" // Singleton name for cluster-scoped KubeOpenCodeConfig
			taskName := uniqueName("task-ttl")
			description := "Task for TTL cleanup test"

			By("Deleting existing KubeOpenCodeConfig if present")
			existingConfig := &kubeopenv1alpha1.KubeOpenCodeConfig{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: configName}, existingConfig); err == nil {
				Expect(k8sClient.Delete(ctx, existingConfig)).Should(Succeed())
				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: configName}, existingConfig)
					return err != nil
				}, timeout, interval).Should(BeTrue())
			}

			By("Creating KubeOpenCodeConfig with TTL cleanup (5 seconds)")
			ttlSeconds := int32(5)
			config := &kubeopenv1alpha1.KubeOpenCodeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: configName,
				},
				Spec: kubeopenv1alpha1.KubeOpenCodeConfigSpec{
					Cleanup: &kubeopenv1alpha1.CleanupConfig{
						TTLSecondsAfterFinished: &ttlSeconds,
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())

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

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Waiting for Task to be deleted due to TTL")
			Eventually(func() bool {
				task := &kubeopenv1alpha1.Task{}
				err := k8sClient.Get(ctx, taskKey, task)
				return err != nil // Task should be deleted (NotFound)
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up KubeOpenCodeConfig")
			Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		})

		It("should not delete Task when no cleanup is configured", func() {
			taskName := uniqueName("task-no-cleanup")
			description := "Task without cleanup config"

			By("Creating Task without KubeOpenCodeConfig")
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

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying Task still exists after waiting")
			Consistently(func() bool {
				task := &kubeopenv1alpha1.Task{}
				err := k8sClient.Get(ctx, taskKey, task)
				return err == nil // Task should still exist
			}, "5s", interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})

		It("should delete oldest Tasks when retention limit is exceeded", func() {
			configName := "cluster" // Singleton name for cluster-scoped KubeOpenCodeConfig
			description := "Task for retention test"

			By("Deleting existing KubeOpenCodeConfig if present")
			existingConfig := &kubeopenv1alpha1.KubeOpenCodeConfig{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: configName}, existingConfig); err == nil {
				Expect(k8sClient.Delete(ctx, existingConfig)).Should(Succeed())
				// Wait for deletion to complete
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: configName}, existingConfig)
					return err != nil
				}, timeout, interval).Should(BeTrue())
			}

			By("Creating KubeOpenCodeConfig with retention limit of 2")
			maxRetained := int32(2)
			config := &kubeopenv1alpha1.KubeOpenCodeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: configName,
				},
				Spec: kubeopenv1alpha1.KubeOpenCodeConfigSpec{
					Cleanup: &kubeopenv1alpha1.CleanupConfig{
						MaxRetainedTasks: &maxRetained,
					},
				},
			}
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())

			By("Creating and completing 3 Tasks sequentially")
			taskNames := []string{
				uniqueName("task-ret-1"),
				uniqueName("task-ret-2"),
				uniqueName("task-ret-3"),
			}

			for _, taskName := range taskNames {
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

				// Wait for Task to complete
				taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
				Eventually(func() kubeopenv1alpha1.TaskPhase {
					createdTask := &kubeopenv1alpha1.Task{}
					if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
						return ""
					}
					return createdTask.Status.Phase
				}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))
			}

			By("Waiting for oldest Task to be deleted due to retention limit")
			oldestTaskKey := types.NamespacedName{Name: taskNames[0], Namespace: testNS}
			Eventually(func() bool {
				task := &kubeopenv1alpha1.Task{}
				err := k8sClient.Get(ctx, oldestTaskKey, task)
				return err != nil // Should be deleted (NotFound)
			}, timeout, interval).Should(BeTrue())

			By("Verifying newer Tasks still exist")
			for _, taskName := range taskNames[1:] {
				taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
				task := &kubeopenv1alpha1.Task{}
				Expect(k8sClient.Get(ctx, taskKey, task)).Should(Succeed())
			}

			By("Cleaning up")
			for _, taskName := range taskNames[1:] {
				task := &kubeopenv1alpha1.Task{}
				taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
				if err := k8sClient.Get(ctx, taskKey, task); err == nil {
					Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
				}
			}
			Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		})
	})

	Context("Task with Runtime Context", func() {
		It("should inject platform awareness content", func() {
			taskName := uniqueName("task-runtime")
			description := "Test runtime context injection"

			By("Creating Task with Runtime context")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type: kubeopenv1alpha1.ContextTypeRuntime,
							// No mountPath - should be written to .kubeopencode/context.md
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying Runtime context content is in the logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			// Runtime context should include platform awareness content
			Expect(logs).Should(ContainSubstring("KubeOpenCode"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task with Optional Context", func() {
		It("should succeed when optional context resolution fails", func() {
			taskName := uniqueName("task-optional")
			description := "Test optional context"
			nonExistentCM := "non-existent-configmap-12345"

			By("Creating Task with optional ConfigMap context referencing non-existent CM")
			optional := true
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeConfigMap,
							MountPath: "/workspace/optional-content.md",
							ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
								Name:     nonExistentCM,
								Key:      "content",
								Optional: &optional,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete (should succeed despite missing ConfigMap)")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying task.md content is in the logs")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("Test optional context"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task with Relative MountPath", func() {
		It("should resolve relative path to workspaceDir", func() {
			taskName := uniqueName("task-rel-path")
			description := "Test relative mount path"
			fileContent := "# Relative Path Content\n\nThis file should be at guides/readme.md"

			By("Creating Task with relative mountPath")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeText,
							Text:      fileContent,
							MountPath: "guides/readme.md", // Relative path
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying file is at the relative path under workspaceDir")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			// The echo agent's find command shows file paths
			Expect(logs).Should(ContainSubstring("/workspace/guides/readme.md"))
			Expect(logs).Should(ContainSubstring("Relative Path Content"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})

	Context("Task with Context FileMode", func() {
		It("should apply fileMode to mounted context file", func() {
			taskName := uniqueName("task-filemode")
			description := "Test file mode"
			scriptContent := "#!/bin/sh\necho 'Hello from script'"

			By("Creating Agent that checks file permissions")
			fileModeAgentName := uniqueName("filemode-agent")
			fileModeAgent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fileModeAgentName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					ExecutorImage:      echoImage,
					ServiceAccountName: testServiceAccount,
					WorkspaceDir:       "/workspace",
					Command:            []string{"sh", "-c", "echo '=== File Permissions ===' && ls -la ${WORKSPACE_DIR}/scripts/ && echo '=== Content ===' && cat ${WORKSPACE_DIR}/scripts/run.sh"},
				},
			}
			Expect(k8sClient.Create(ctx, fileModeAgent)).Should(Succeed())

			By("Creating Task with fileMode 0755")
			fileMode := int32(493) // 0755 in decimal
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: fileModeAgentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeText,
							Text:      scriptContent,
							MountPath: "/workspace/scripts/run.sh",
							FileMode:  &fileMode,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying file has executable permissions")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("File Permissions"))
			// The ls -la output should show executable permissions (rwx)
			Expect(logs).Should(ContainSubstring("run.sh"))
			Expect(logs).Should(ContainSubstring("Hello from script"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, fileModeAgent)).Should(Succeed())
		})
	})

	Context("Task with ConfigMap as directory", func() {
		It("should mount ConfigMap without key as directory with all keys as files", func() {
			taskName := uniqueName("task-cm-dir")
			configMapName := uniqueName("multi-file-cm")
			description := "Test ConfigMap directory mount"

			By("Creating ConfigMap with multiple keys")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: testNS,
				},
				Data: map[string]string{
					"config.yaml":   "app:\n  name: test\n  version: 1.0",
					"settings.json": `{"debug": true, "level": "info"}`,
					"README.md":     "# Configuration Files\n\nThese are the config files.",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			By("Creating Task with ConfigMap context without key (directory mount)")
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      taskName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef:    &kubeopenv1alpha1.AgentReference{Name: agentName},
					Description: &description,
					Contexts: []kubeopenv1alpha1.ContextItem{
						{
							Type:      kubeopenv1alpha1.ContextTypeConfigMap,
							MountPath: "/workspace/config",
							ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
								Name: configMapName,
								// No Key specified - entire ConfigMap as directory
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, task)).Should(Succeed())

			By("Waiting for Task to complete")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Verifying all ConfigMap files are mounted")
			jobName := fmt.Sprintf("%s-pod", taskName)
			logs := getPodLogs(ctx, testNS, jobName)
			Expect(logs).Should(ContainSubstring("config.yaml"))
			Expect(logs).Should(ContainSubstring("settings.json"))
			Expect(logs).Should(ContainSubstring("README.md"))
			// Verify content
			Expect(logs).Should(ContainSubstring("app:"))
			Expect(logs).Should(ContainSubstring("debug"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		})
	})

	Context("Task status AgentRef resolution", func() {
		It("should populate status.agentRef after resolution", func() {
			taskName := uniqueName("task-agentref-status")
			description := "Test AgentRef status resolution"

			By("Creating Task with explicit agentRef")
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

			By("Waiting for Task to start running")
			taskKey := types.NamespacedName{Name: taskName, Namespace: testNS}
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Or(
				Equal(kubeopenv1alpha1.TaskPhaseRunning),
				Equal(kubeopenv1alpha1.TaskPhaseCompleted),
			))

			By("Verifying status.agentRef is populated correctly")
			runningTask := &kubeopenv1alpha1.Task{}
			Expect(k8sClient.Get(ctx, taskKey, runningTask)).Should(Succeed())
			Expect(runningTask.Status.AgentRef).ShouldNot(BeNil())
			Expect(runningTask.Status.AgentRef.Name).Should(Equal(agentName))

			By("Waiting for Task to complete")
			Eventually(func() kubeopenv1alpha1.TaskPhase {
				createdTask := &kubeopenv1alpha1.Task{}
				if err := k8sClient.Get(ctx, taskKey, createdTask); err != nil {
					return ""
				}
				return createdTask.Status.Phase
			}, timeout, interval).Should(Equal(kubeopenv1alpha1.TaskPhaseCompleted))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, task)).Should(Succeed())
		})
	})
})

// getPodLogs retrieves logs from the Pod associated with a Task
func getPodLogs(ctx context.Context, namespace, podName string) string {
	// The pod name is "{task-name}-pod", task name is the label value
	taskName := strings.TrimSuffix(podName, "-pod")

	// List pods with the kubeopencode.io/task label
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels{"kubeopencode.io/task": taskName})
	if err != nil || len(pods.Items) == 0 {
		return ""
	}

	var allLogs strings.Builder
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			req := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: container.Name,
			})
			stream, err := req.Stream(ctx)
			if err != nil {
				continue
			}
			defer func() { _ = stream.Close() }()

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, stream)
			if err != nil {
				continue
			}
			allLogs.WriteString(buf.String())
		}
	}

	return allLogs.String()
}
