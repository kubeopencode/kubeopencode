// Copyright Contributors to the KubeOpenCode project

//go:build integration

package controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

var _ = Describe("RegistryController", func() {
	const (
		registryNamespace = "default"
		registryTimeout   = time.Second * 30
		registryInterval  = time.Millisecond * 250
	)

	Context("When creating an empty Registry", func() {
		It("Should set Ready condition with Empty reason", func() {
			registryName := "test-registry-empty"

			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{},
			}

			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			// Wait for controller to reconcile and set status
			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() bool {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return false
				}
				return r.Status.ObservedGeneration == r.Generation
			}, registryTimeout, registryInterval).Should(BeTrue())

			// Verify status
			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.TotalCount).To(Equal(0))
			Expect(r.Status.Summary.ReadyCount).To(Equal(0))

			// Verify Ready condition
			Expect(r.Status.Conditions).NotTo(BeEmpty())
			var readyCondition *metav1.Condition
			for i := range r.Status.Conditions {
				if r.Status.Conditions[i].Type == "Ready" {
					readyCondition = &r.Status.Conditions[i]
					break
				}
			}
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal("Empty"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})

	Context("When creating a Registry with images", func() {
		It("Should set image statuses and update summary", func() {
			registryName := "test-registry-images"

			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Images: []kubeopenv1alpha1.RegistryImage{
						{
							Name:  "go-dev",
							Image: "ghcr.io/kubeopencode/test-image:latest",
							Metadata: kubeopenv1alpha1.ImageMetadata{
								Description: "Go development image",
								Category:    "backend",
								Tags:        []string{"golang"},
								Tools:       []string{"go", "gopls"},
							},
						},
						{
							Name:  "invalid",
							Image: "",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			// Wait for controller to reconcile
			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() int {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return -1
				}
				return r.Status.Summary.TotalCount
			}, registryTimeout, registryInterval).Should(Equal(2))

			// Verify statuses
			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.Images).To(Equal(2))
			Expect(r.Status.Images).To(HaveLen(2))

			// The valid image should be Ready (basic format check passes)
			var goDevStatus *kubeopenv1alpha1.ImageStatus
			var invalidStatus *kubeopenv1alpha1.ImageStatus
			for i := range r.Status.Images {
				switch r.Status.Images[i].Name {
				case "go-dev":
					goDevStatus = &r.Status.Images[i]
				case "invalid":
					invalidStatus = &r.Status.Images[i]
				}
			}
			Expect(goDevStatus).NotTo(BeNil())
			Expect(goDevStatus.Phase).To(Equal(kubeopenv1alpha1.AssetPhaseReady))
			Expect(invalidStatus).NotTo(BeNil())
			Expect(invalidStatus.Phase).To(Equal(kubeopenv1alpha1.AssetPhaseUnavailable))

			// Cleanup
			Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})

	Context("When creating a Registry with skills", func() {
		It("Should validate skill Git URLs", func() {
			registryName := "test-registry-skills"

			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Skills: []kubeopenv1alpha1.RegistrySkill{
						{
							Name: "valid-skill",
							Git: &kubeopenv1alpha1.GitSkillSource{
								Repository: "https://github.com/company/skills.git",
							},
							Metadata: kubeopenv1alpha1.AssetMetadata{
								Description: "Test skill",
								Tags:        []string{"test"},
							},
						},
						{
							Name: "no-git-skill",
							// No Git source specified
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			// Wait for controller to reconcile
			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() int {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return -1
				}
				return r.Status.Summary.TotalCount
			}, registryTimeout, registryInterval).Should(Equal(2))

			// Verify
			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.Skills).To(Equal(2))
			Expect(r.Status.Skills).To(HaveLen(2))

			// Valid skill should be Ready (URL format is valid)
			var validSkill *kubeopenv1alpha1.SkillStatus
			var invalidSkill *kubeopenv1alpha1.SkillStatus
			for i := range r.Status.Skills {
				switch r.Status.Skills[i].Name {
				case "valid-skill":
					validSkill = &r.Status.Skills[i]
				case "no-git-skill":
					invalidSkill = &r.Status.Skills[i]
				}
			}
			Expect(validSkill).NotTo(BeNil())
			Expect(validSkill.Phase).To(Equal(kubeopenv1alpha1.AssetPhaseReady))
			Expect(invalidSkill).NotTo(BeNil())
			Expect(invalidSkill.Phase).To(Equal(kubeopenv1alpha1.AssetPhaseUnavailable))

		// Cleanup
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})

	Context("When creating a Registry with plugins", func() {
		It("Should set plugin statuses", func() {
			registryName := "test-registry-plugins"
			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Plugins: []kubeopenv1alpha1.RegistryPlugin{
						{
							Name:   "valid-plugin",
							Plugin: kubeopenv1alpha1.PluginSpec{Name: "lodash@4.17.21"},
						},
						{
							Name:   "empty-plugin",
							Plugin: kubeopenv1alpha1.PluginSpec{Name: ""},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() int {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return -1
				}
				return r.Status.Summary.TotalCount
			}, registryTimeout, registryInterval).Should(Equal(2))

			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.Plugins).To(Equal(2))
			Expect(r.Status.Plugins).To(HaveLen(2))

			// Empty name plugin should be Unavailable
			var emptyPlugin *kubeopenv1alpha1.PluginStatus
			for i := range r.Status.Plugins {
				if r.Status.Plugins[i].Name == "empty-plugin" {
					emptyPlugin = &r.Status.Plugins[i]
				}
			}
			Expect(emptyPlugin).NotTo(BeNil())
			Expect(emptyPlugin.Phase).To(Equal(kubeopenv1alpha1.AssetPhaseUnavailable))
			Expect(emptyPlugin.Message).To(ContainSubstring("empty"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})

	Context("When creating a Registry with mixed assets", func() {
		It("Should set correct summary counts and Ready=False", func() {
			registryName := "test-registry-mixed"
			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Images: []kubeopenv1alpha1.RegistryImage{
						{
							Name:  "valid-image",
							Image: "ghcr.io/kubeopencode/test:latest",
						},
						{
							Name:  "invalid-image",
							Image: "",
						},
					},
					Skills: []kubeopenv1alpha1.RegistrySkill{
						{
							Name: "valid-skill",
							Git: &kubeopenv1alpha1.GitSkillSource{
								Repository: "https://github.com/company/skills.git",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() int {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return -1
				}
				return r.Status.Summary.TotalCount
			}, registryTimeout, registryInterval).Should(Equal(3))

			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.TotalCount).To(Equal(3))
			Expect(r.Status.Summary.ReadyCount).To(Equal(2))
			Expect(r.Status.Summary.Images).To(Equal(2))
			Expect(r.Status.Summary.Skills).To(Equal(1))

			// Ready condition should be False with reason AssetsUnavailable
			var readyCondition *metav1.Condition
			for i := range r.Status.Conditions {
				if r.Status.Conditions[i].Type == "Ready" {
					readyCondition = &r.Status.Conditions[i]
					break
				}
			}
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal("AssetsUnavailable"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})

	Context("When creating a Registry with all valid assets", func() {
		It("Should set Ready=True with reason AllAssetsReady", func() {
			registryName := "test-registry-all-valid"
			registry := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      registryName,
					Namespace: registryNamespace,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Images: []kubeopenv1alpha1.RegistryImage{
						{
							Name:  "valid-image",
							Image: "ghcr.io/kubeopencode/test:latest",
						},
					},
					Skills: []kubeopenv1alpha1.RegistrySkill{
						{
							Name: "valid-skill",
							Git: &kubeopenv1alpha1.GitSkillSource{
								Repository: "https://github.com/company/skills.git",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())

			registryKey := types.NamespacedName{Name: registryName, Namespace: registryNamespace}
			Eventually(func() int {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, registryKey, r); err != nil {
					return -1
				}
				return r.Status.Summary.TotalCount
			}, registryTimeout, registryInterval).Should(Equal(2))

			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, registryKey, r)).Should(Succeed())
			Expect(r.Status.Summary.TotalCount).To(Equal(2))
			Expect(r.Status.Summary.ReadyCount).To(Equal(2))

			// Ready condition should be True with reason AllAssetsReady
			var readyCondition *metav1.Condition
			for i := range r.Status.Conditions {
				if r.Status.Conditions[i].Type == "Ready" {
					readyCondition = &r.Status.Conditions[i]
					break
				}
			}
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
			Expect(readyCondition.Reason).To(Equal("AllAssetsReady"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		})
	})
})
