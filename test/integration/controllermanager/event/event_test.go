// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package event_test

import (
	"time"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Event controller tests", func() {
	var (
		shootEvent, nonShootEvent *corev1.Event

		// For testing purpose we are setting this variable to be same as TTLNonShootEvents, to have
		// timeUntilDeletion value in controller to be 0 or less that 0. This is done to mock the deletion of non-shoot events
		// on reaching TTL.
		ttl = &metav1.Duration{Duration: 30 * time.Minute}
	)

	BeforeEach(func() {
		shootEvent = &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			LastTimestamp:  metav1.Time{Time: time.Now().Add(-ttl.Duration)},
			InvolvedObject: corev1.ObjectReference{Kind: "Shoot", APIVersion: "core.gardener.cloud/v1beta1", Namespace: testNamespace.Name},
		}

		nonShootEvent = &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			LastTimestamp:  metav1.Time{Time: time.Now().Add(-ttl.Duration)},
			InvolvedObject: corev1.ObjectReference{Kind: "Project", APIVersion: "core.gardener.cloud/v1beta1", Namespace: testNamespace.Name},
		}
	})

	JustBeforeEach(func() {
		By("Create shoot event")
		Expect(testClient.Create(ctx, shootEvent)).To(Succeed())
		log.Info("Created shoot Event for test", "shootEvent", client.ObjectKeyFromObject(shootEvent))

		DeferCleanup(func() {
			By("Delete shoot Event")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, shootEvent))).To(Succeed())
		})

		By("Create non-shoot event")
		Expect(testClient.Create(ctx, nonShootEvent)).To(Succeed())
		log.Info("Created non-shoot Event for test", "nonShootEvent", client.ObjectKeyFromObject(nonShootEvent))

		DeferCleanup(func() {
			By("Delete non-shoot Event")
			Expect(testClient.Delete(ctx, nonShootEvent)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	Context("shoot events", func() {
		It("should not remove the shoot event", func() {
			Eventually(func(g Gomega) error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(shootEvent), shootEvent)
			}).Should(Succeed())
		})
	})

	Context("non-Shoot events", func() {
		Describe("ttl not reached", func() {
			BeforeEach(func() {
				nonShootEvent.LastTimestamp = metav1.Time{Time: time.Now()}
			})

			It("should not remove the event", func() {
				Eventually(func(g Gomega) error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), nonShootEvent)
				}).Should(Succeed())
			})
		})

		Describe("ttl reached", func() {
			It("should remove the non shoot event", func() {
				Eventually(func(g Gomega) error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(nonShootEvent), nonShootEvent)
				}).Should(BeNotFoundError())
			})
		})
	})
})
