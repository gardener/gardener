// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Quota controller tests", func() {
	var (
		quota         *gardencorev1beta1.Quota
		secretBinding *gardencorev1beta1.SecretBinding
		shoot         *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now().UTC())

		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.QuotaSpec{
				Scope: corev1.ObjectReference{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Project",
				},
				ClusterLifetimeDays: ptr.To[int32](1),
			},
		}

		By("Create Quota")
		Expect(testClient.Create(ctx, quota)).To(Succeed())
		log.Info("Created Quota for test", "quota", client.ObjectKeyFromObject(quota))

		DeferCleanup(func() {
			By("Delete Quota")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, quota))).To(Succeed())
		})

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Provider: &gardencorev1beta1.SecretBindingProvider{
				Type: "foo",
			},
			SecretRef: corev1.SecretReference{Name: "some-secret"},
			Quotas: []corev1.ObjectReference{{
				Name:      quota.Name,
				Namespace: quota.Namespace,
			}},
		}

		By("Create SecretBinding")
		Expect(testClient.Create(ctx, secretBinding)).To(Succeed())
		log.Info("Created SecretBinding for test", "secretBinding", client.ObjectKeyFromObject(secretBinding))

		DeferCleanup(func() {
			By("Delete SecretBinding")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, secretBinding))).To(Succeed())
		})

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To(secretBinding.Name),
				CloudProfileName:  ptr.To("cloudprofile1"),
				Region:            "europe-central-1",
				Provider: gardencorev1beta1.Provider{
					Type: "foo-provider",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 3,
							Maximum: 3,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
							},
						},
					},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("some-domain.example.com"),
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To("foo-networking"),
				},
			},
		}

		By("Create Shoot")
		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})
	})

	JustBeforeEach(func() {
		Eventually(func(g Gomega) map[string]string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Annotations
		}).Should(HaveKey("shoot.gardener.cloud/expiration-timestamp"))
	})

	It("should delete the shoot because the expiration time has passed", func() {
		fakeClock.Step(48 * time.Hour)

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())
	})

	It("should not delete the shoot because the quota has no lifetime setting anymore", func() {
		By("Remove cluster lifetime setting from Quota")
		patch := client.MergeFrom(quota.DeepCopy())
		quota.Spec.ClusterLifetimeDays = nil
		Expect(testClient.Patch(ctx, quota, patch)).To(Succeed())

		By("Ensure manager has observed the updated Quota")
		Eventually(func(g Gomega) *int32 {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
			return quota.Spec.ClusterLifetimeDays
		}).Should(BeNil())

		By("Step the clock")
		fakeClock.Step(48 * time.Hour)

		By("Ensure the shoot still exists")
		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(Succeed())

		By("Ensure the shoot does no longer have the expiration timestamp annotation")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Annotations
		}).ShouldNot(HaveKey("shoot.gardener.cloud/expiration-timestamp"))
	})

	It("should consider shorter (manually set) expiration times and delete the shoot", func() {
		patch := client.MergeFrom(shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/expiration-timestamp", fakeClock.Now().UTC().Add(time.Hour).Format(time.RFC3339))
		Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

		fakeClock.Step(2 * time.Hour)

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())
	})

	It("should consider longer (manually set) expiration times and delete the shoot", func() {
		patch := client.MergeFrom(shoot.DeepCopy())
		newTimestamp := fakeClock.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/expiration-timestamp", newTimestamp)
		Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

		By("Ensure manager has observed the updated Shoot")
		Eventually(func(g Gomega) map[string]string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Annotations
		}).Should(HaveKeyWithValue("shoot.gardener.cloud/expiration-timestamp", newTimestamp))

		By("Verify that shoot is not deleted after original expiration time (1 day) has passed")
		fakeClock.Step(25 * time.Hour)
		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(Succeed())

		By("Verify that shoot is deleted after manually prolonged expiration time has passed")
		fakeClock.Step(25 * time.Hour)
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		}).Should(BeNotFoundError())
	})
})
