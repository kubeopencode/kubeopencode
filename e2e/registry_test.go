// Copyright Contributors to the KubeOpenCode project

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const LabelRegistry = "registry"

var _ = Describe("Registry E2E Tests", Label(LabelRegistry), func() {

	Context("Registry lifecycle", func() {
		It("should reconcile status with ObservedGeneration and Ready condition", func() {
			regName := uniqueName("reg-lifecycle")

			By("Creating Registry with an image")
			reg := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      regName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{
					Images: []kubeopenv1alpha1.RegistryImage{
						{
							Name:  "test-image",
							Image: "ghcr.io/kubeopencode/kubeopencode-agent-devbox:dev",
							Metadata: kubeopenv1alpha1.ImageMetadata{
								Description: "E2E test image",
								Category:    "test",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, reg)).Should(Succeed())

			By("Verifying ObservedGeneration is set")
			regKey := types.NamespacedName{Name: regName, Namespace: testNS}
			Eventually(func() int64 {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, regKey, r); err != nil {
					return 0
				}
				return r.Status.ObservedGeneration
			}, timeout, interval).Should(BeNumerically(">", 0))

			By("Verifying Ready condition is True (image format validation passes)")
			Eventually(func() string {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, regKey, r); err != nil {
					return ""
				}
				for _, c := range r.Status.Conditions {
					if c.Type == "Ready" {
						return string(c.Status)
					}
				}
				return ""
			}, timeout, interval).Should(Equal("True"))

			By("Verifying summary counts")
			r := &kubeopenv1alpha1.Registry{}
			Expect(k8sClient.Get(ctx, regKey, r)).Should(Succeed())
			Expect(r.Status.Summary.Images).To(Equal(1))
			Expect(r.Status.Summary.TotalCount).To(Equal(1))
			Expect(r.Status.Summary.ReadyCount).To(Equal(1))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, reg)).Should(Succeed())
		})
	})

	Context("Registry deletion", func() {
		It("should delete cleanly", func() {
			regName := uniqueName("reg-delete")
			reg := &kubeopenv1alpha1.Registry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      regName,
					Namespace: testNS,
				},
				Spec: kubeopenv1alpha1.RegistrySpec{},
			}
			Expect(k8sClient.Create(ctx, reg)).Should(Succeed())

			// Wait for reconcile
			regKey := types.NamespacedName{Name: regName, Namespace: testNS}
			Eventually(func() int64 {
				r := &kubeopenv1alpha1.Registry{}
				if err := k8sClient.Get(ctx, regKey, r); err != nil {
					return 0
				}
				return r.Status.ObservedGeneration
			}, timeout, interval).Should(BeNumerically(">", 0))

			Expect(k8sClient.Delete(ctx, reg)).Should(Succeed())

			// Verify it's gone
			Eventually(func() bool {
				r := &kubeopenv1alpha1.Registry{}
				err := k8sClient.Get(ctx, regKey, r)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
