// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Event controller tests", func() {
	var (
		shootEvent, nonShootEvent *corev1.Event

		// For testing purpose we are setting this variable to be more than TTLNonShootEvents, to have
		// timeUntilDeletion value in controller to be less than 0. This is done to mock the deletion of non-shoot events
		// on reaching TTL.
		ttl = &metav1.Duration{Duration: 45 * time.Minute}
	)

	BeforeEach(func() {
		shootEvent = &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
			},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "core.gardener.cloud/v1beta1", Namespace: testNamespace.Name},
		}

		nonShootEvent = &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
			},
			InvolvedObject: corev1.ObjectReference{Kind: "Secret", APIVersion: "v1", Namespace: testNamespace.Name},
		}
	})

	Context("shoot events", func() {
		JustBeforeEach(func() {
			By("Create Shoot Event")
			Expect(testClient.Create(ctx, shootEvent)).To(Succeed())
			log.Info("Created Shoot Event for test", "shootEvent", client.ObjectKeyFromObject(shootEvent))

			DeferCleanup(func() {
				By("Delete Shoot Event")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootEvent))).To(Succeed())
			})
		})

		Describe("ttl not reached", func() {
			BeforeEach(func() {
				shootEvent.LastTimestamp = metav1.Time{Time: time.Now()}
			})

			It("should not remove the event", func() {
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shootEvent), shootEvent)
				}).Should(Succeed())
			})
		})

		Describe("ttl reached", func() {
			BeforeEach(func() {
				shootEvent.LastTimestamp = metav1.Time{Time: time.Now().Add(-ttl.Duration)}
			})

			It("should not remove the event as this is shoot event", func() {
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shootEvent), shootEvent)
				}).Should(Succeed())
			})
		})

	})

	Context("non-shoot events", func() {
		JustBeforeEach(func() {
			By("Create Non-Shoot Event")
			Expect(testClient.Create(ctx, nonShootEvent)).To(Succeed())
			log.Info("Created non-shoot Event for test", "nonShootEvent", client.ObjectKeyFromObject(nonShootEvent))

			DeferCleanup(func() {
				By("Delete Non-Shoot Event")
				Expect(testClient.Delete(ctx, nonShootEvent)).To(Or(Succeed(), BeNotFoundError()))
			})
		})

		Describe("ttl not reached", func() {
			BeforeEach(func() {
				nonShootEvent.LastTimestamp = metav1.Time{Time: time.Now()}
			})

			It("should not remove the event", func() {
				Consistently(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), nonShootEvent)
				}).Should(Succeed())
			})
		})

		Describe("ttl reached", func() {
			BeforeEach(func() {
				nonShootEvent.LastTimestamp = metav1.Time{Time: time.Now().Add(-ttl.Duration)}
			})

			It("should remove the non shoot event", func() {
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), nonShootEvent)
				}).Should(BeNotFoundError())
			})
		})
	})
})
