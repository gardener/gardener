// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot retry controller tests", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("my-provider-account"),
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
		log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
		})
	})

	It("should successfully retry a failed Shoot with rate limits exceeded error", func() {
		By("Mark the Shoot as failed with rate limits exceeded error code")
		shootCopy := shoot.DeepCopy()
		shoot.Status = gardencorev1beta1.ShootStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateFailed,
				LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Minute * -1)},
			},
			LastErrors: []gardencorev1beta1.LastError{
				{
					Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraRateLimitsExceeded},
				},
			},
			ObservedGeneration: 1,
		}
		Expect(testClient.Status().Patch(ctx, shoot, client.MergeFrom(shootCopy))).To(Succeed())

		By("Verify shoot is retried")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Generation).To(Equal(int64(2)))
		}).Should(Succeed())
	})
})
