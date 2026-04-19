// Copyright Contributors to the KubeOpenCode project

//go:build integration

// See suite_test.go for explanation of the "integration" build tag pattern.

package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var _ = Describe("CronTaskController", func() {
	const (
		cronTaskNamespace = "default"

		// farFutureSchedule is a schedule that won't fire during tests (Jan 1 at midnight)
		farFutureSchedule = "0 0 1 1 *"

		// everyMinuteSchedule fires every minute for time-based tests
		everyMinuteSchedule = "* * * * *"
	)

	// cronTaskTimeout is longer than the default timeout because cron scheduling
	// requires waiting for at least one minute boundary.
	cronTaskTimeout := 90 * time.Second

	// newCronTask creates a CronTask with the given name, schedule, and options.
	newCronTask := func(name, schedule string) *kubeopenv1alpha1.CronTask {
		description := fmt.Sprintf("Task from CronTask %s", name)
		return &kubeopenv1alpha1.CronTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: cronTaskNamespace,
			},
			Spec: kubeopenv1alpha1.CronTaskSpec{
				Schedule: schedule,
				TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef:    &kubeopenv1alpha1.AgentReference{Name: testAgentName},
						Description: &description,
					},
				},
			},
		}
	}

	Context("Basic scheduling", func() {
		It("Should set nextScheduleTime in status", func() {
			cronTaskName := fmt.Sprintf("ct-basic-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, farFutureSchedule)

			By("Creating the CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Verifying nextScheduleTime is set")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			Eventually(func() *metav1.Time {
				if err := k8sClient.Get(ctx, lookup, created); err != nil {
					return nil
				}
				return created.Status.NextScheduleTime
			}, timeout, interval).ShouldNot(BeNil())

			By("Verifying nextScheduleTime is in the future")
			Expect(created.Status.NextScheduleTime.Time.After(time.Now())).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
		})
	})

	Context("Task creation on schedule", func() {
		It("Should create a child Task with correct labels and ownerReferences", func() {
			cronTaskName := fmt.Sprintf("ct-sched-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)

			By("Creating the CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for a child Task to be created")
			taskList := &kubeopenv1alpha1.TaskList{}
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 1))

			By("Verifying child Task has correct label")
			childTask := taskList.Items[0]
			Expect(childTask.Labels).Should(HaveKeyWithValue(kubeopenv1alpha1.CronTaskLabelKey, cronTaskName))

			By("Verifying child Task has ownerReference pointing to CronTask")
			Expect(childTask.OwnerReferences).Should(HaveLen(1))
			Expect(childTask.OwnerReferences[0].Kind).Should(Equal("CronTask"))
			Expect(childTask.OwnerReferences[0].Name).Should(Equal(cronTaskName))

			By("Verifying CronTask status is updated")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			updatedCronTask := &kubeopenv1alpha1.CronTask{}
			Eventually(func() *metav1.Time {
				if err := k8sClient.Get(ctx, lookup, updatedCronTask); err != nil {
					return nil
				}
				return updatedCronTask.Status.LastScheduleTime
			}, cronTaskTimeout, interval).ShouldNot(BeNil())
			Expect(updatedCronTask.Status.TotalExecutions).Should(BeNumerically(">=", int64(1)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
		})
	})

	Context("Suspend", func() {
		It("Should not create Tasks when suspended and should clear nextScheduleTime", func() {
			cronTaskName := fmt.Sprintf("ct-suspend-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)
			suspend := true
			cronTask.Spec.Suspend = &suspend

			By("Creating the suspended CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Verifying nextScheduleTime is nil (suspended)")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			// Wait for reconciliation to happen
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lookup, created); err != nil {
					return false
				}
				// Check that the Ready condition is set with reason Suspended
				for _, c := range created.Status.Conditions {
					if c.Type == "Ready" && c.Reason == "Suspended" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Expect(created.Status.NextScheduleTime).Should(BeNil())

			By("Verifying no child Tasks exist")
			taskList := &kubeopenv1alpha1.TaskList{}
			Consistently(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return -1
				}
				return len(taskList.Items)
			}, 3*time.Second, interval).Should(Equal(0))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
		})
	})

	Context("MaxRetainedTasks blocking", func() {
		It("Should not create new Tasks when maxRetainedTasks is reached", func() {
			cronTaskName := fmt.Sprintf("ct-maxretain-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)
			maxRetained := int32(2)
			cronTask.Spec.MaxRetainedTasks = &maxRetained

			By("Creating the CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Manually creating child Tasks up to the limit")
			for i := 0; i < int(maxRetained); i++ {
				description := fmt.Sprintf("Manual child task %d", i)
				childTask := &kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-manual-%d", cronTaskName, i),
						Namespace: cronTaskNamespace,
						Labels: map[string]string{
							kubeopenv1alpha1.CronTaskLabelKey: cronTaskName,
						},
					},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef:    &kubeopenv1alpha1.AgentReference{Name: testAgentName},
						Description: &description,
					},
				}
				Expect(k8sClient.Create(ctx, childTask)).Should(Succeed())
			}

			By("Verifying controller detects maxRetainedTasks is reached")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			updatedCronTask := &kubeopenv1alpha1.CronTask{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lookup, updatedCronTask); err != nil {
					return false
				}
				for _, c := range updatedCronTask.Status.Conditions {
					if c.Type == "Ready" && c.Reason == "MaxRetainedTasksReached" {
						return true
					}
				}
				return false
			}, cronTaskTimeout, interval).Should(BeTrue())

			By("Verifying no additional Tasks beyond the limit are created")
			taskList := &kubeopenv1alpha1.TaskList{}
			Consistently(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return -1
				}
				return len(taskList.Items)
			}, 5*time.Second, interval).Should(Equal(int(maxRetained)))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
			for i := 0; i < int(maxRetained); i++ {
				childTask := &kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-manual-%d", cronTaskName, i),
						Namespace: cronTaskNamespace,
					},
				}
				_ = k8sClient.Delete(ctx, childTask)
			}
		})
	})

	Context("ConcurrencyPolicy Forbid", func() {
		It("Should skip creating new Task when an active Task exists", func() {
			cronTaskName := fmt.Sprintf("ct-forbid-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)
			cronTask.Spec.ConcurrencyPolicy = kubeopenv1alpha1.ForbidConcurrent

			By("Creating the CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for the first child Task to be created")
			taskList := &kubeopenv1alpha1.TaskList{}
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 1))

			By("Verifying the first Task is still active (Running or Pending)")
			// In envtest, Tasks stay in Running phase because there's no real Pod
			// to complete the task. This simulates an active Task.
			firstTaskCount := len(taskList.Items)

			By("Verifying CronTask status shows ActiveTaskExists condition")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			updatedCronTask := &kubeopenv1alpha1.CronTask{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lookup, updatedCronTask); err != nil {
					return false
				}
				for _, c := range updatedCronTask.Status.Conditions {
					if c.Type == "Ready" && c.Reason == "ActiveTaskExists" {
						return true
					}
				}
				return false
			}, cronTaskTimeout, interval).Should(BeTrue())

			By("Verifying no additional Tasks are created while first is active")
			Consistently(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return -1
				}
				return len(taskList.Items)
			}, 5*time.Second, interval).Should(Equal(firstTaskCount))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
		})
	})

	Context("Trigger annotation", func() {
		It("Should create a Task immediately when trigger annotation is set and remove the annotation", func() {
			cronTaskName := fmt.Sprintf("ct-trigger-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, farFutureSchedule)

			By("Creating the CronTask with far future schedule")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for initial reconciliation")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			Eventually(func() *metav1.Time {
				if err := k8sClient.Get(ctx, lookup, created); err != nil {
					return nil
				}
				return created.Status.NextScheduleTime
			}, timeout, interval).ShouldNot(BeNil())

			By("Verifying no Tasks exist yet (far future schedule)")
			taskList := &kubeopenv1alpha1.TaskList{}
			Consistently(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return -1
				}
				return len(taskList.Items)
			}, 2*time.Second, interval).Should(Equal(0))

			By("Adding trigger annotation")
			// Re-fetch to get latest version
			Expect(k8sClient.Get(ctx, lookup, created)).Should(Succeed())
			if created.Annotations == nil {
				created.Annotations = make(map[string]string)
			}
			created.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation] = "true"
			Expect(k8sClient.Update(ctx, created)).Should(Succeed())

			By("Verifying a Task is created immediately")
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, timeout, interval).Should(Equal(1))

			By("Verifying the trigger annotation is removed")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lookup, created); err != nil {
					return false
				}
				_, exists := created.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation]
				return !exists
			}, timeout, interval).Should(BeTrue())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, cronTask)).Should(Succeed())
		})
	})

	Context("ConcurrencyPolicy Replace", func() {
		It("Should stop active Tasks when a new scheduled Task is created", func() {
			cronTaskName := fmt.Sprintf("ct-replace-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)
			cronTask.Spec.ConcurrencyPolicy = kubeopenv1alpha1.ReplaceConcurrent

			By("Creating the CronTask with everyMinuteSchedule")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for the first child Task to be created via schedule")
			taskList := &kubeopenv1alpha1.TaskList{}
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 1))

			firstTaskName := taskList.Items[0].Name

			By("Waiting for the second child Task via next schedule (Replace should stop the first)")
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 2))

			By("Verifying the first Task has stop annotation from Replace policy")
			firstTask := &kubeopenv1alpha1.Task{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: firstTaskName, Namespace: cronTaskNamespace}, firstTask); err != nil {
					return false
				}
				return firstTask.Annotations != nil && firstTask.Annotations["kubeopencode.io/stop"] == "true"
			}, cronTaskTimeout, interval).Should(BeTrue())

			By("Cleaning up")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			Expect(k8sClient.Get(ctx, lookup, created)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, created)).Should(Succeed())
		})
	})

	Context("ConcurrencyPolicy Allow", func() {
		It("Should allow multiple Tasks to run concurrently", func() {
			cronTaskName := fmt.Sprintf("ct-allow-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, everyMinuteSchedule)
			cronTask.Spec.ConcurrencyPolicy = kubeopenv1alpha1.AllowConcurrent

			By("Creating the CronTask with everyMinuteSchedule")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for the first child Task via schedule")
			taskList := &kubeopenv1alpha1.TaskList{}
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 1))

			By("Waiting for the second child Task via next schedule (Allow should keep the first running)")
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, cronTaskTimeout, interval).Should(BeNumerically(">=", 2))

			By("Verifying no Tasks have stop annotation (Allow policy)")
			for _, task := range taskList.Items {
				Expect(task.Annotations).ShouldNot(HaveKey("kubeopencode.io/stop"))
			}

			By("Cleaning up")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			Expect(k8sClient.Get(ctx, lookup, created)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, created)).Should(Succeed())
		})
	})

	Context("OwnerReference setup", func() {
		It("Should set correct ownerReference on child Tasks", func() {
			cronTaskName := fmt.Sprintf("ct-gc-%d", time.Now().UnixNano())
			cronTask := newCronTask(cronTaskName, farFutureSchedule)

			By("Creating the CronTask")
			Expect(k8sClient.Create(ctx, cronTask)).Should(Succeed())

			By("Waiting for initial reconciliation")
			lookup := types.NamespacedName{Name: cronTaskName, Namespace: cronTaskNamespace}
			created := &kubeopenv1alpha1.CronTask{}
			Eventually(func() *metav1.Time {
				if err := k8sClient.Get(ctx, lookup, created); err != nil {
					return nil
				}
				return created.Status.NextScheduleTime
			}, timeout, interval).ShouldNot(BeNil())

			By("Triggering Task creation via annotation")
			Expect(k8sClient.Get(ctx, lookup, created)).Should(Succeed())
			if created.Annotations == nil {
				created.Annotations = make(map[string]string)
			}
			created.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation] = "true"
			Expect(k8sClient.Update(ctx, created)).Should(Succeed())

			By("Waiting for child Task to be created")
			taskList := &kubeopenv1alpha1.TaskList{}
			Eventually(func() int {
				if err := k8sClient.List(ctx, taskList,
					client.InNamespace(cronTaskNamespace),
					client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTaskName},
				); err != nil {
					return 0
				}
				return len(taskList.Items)
			}, timeout, interval).Should(BeNumerically(">=", 1))

			By("Verifying child Task has ownerReference with controller=true")
			childTask := taskList.Items[0]
			Expect(childTask.OwnerReferences).Should(HaveLen(1))
			Expect(childTask.OwnerReferences[0].Kind).Should(Equal("CronTask"))
			Expect(childTask.OwnerReferences[0].Name).Should(Equal(cronTaskName))
			Expect(childTask.OwnerReferences[0].Controller).ShouldNot(BeNil())
			Expect(*childTask.OwnerReferences[0].Controller).Should(BeTrue())

			// Note: Actual garbage collection (cascade delete) is tested in E2E tests.
			// envtest does not run the Kubernetes GC controller.

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, created)).Should(Succeed())
		})
	})
})
