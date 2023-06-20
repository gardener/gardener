// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hibernation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot Hibernation controller tests", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: pointer.String("my-provider-account"),
				CloudProfileName:  "cloudprofile1",
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
					Version: "1.25.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: pointer.String("foo-networking"),
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

	It("should successfully hibernate then wake up the shoot based on schedule", func() {
		By("Set clock time to be 1 second before hibernation trigger time")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		nextHour := roundToNextHour(shoot.CreationTimestamp.Time)
		fakeClock.SetTime(nextHour.Add(59 * time.Second))

		By("Patch shoot with hibernation schedules")
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Schedules: []gardencorev1beta1.HibernationSchedule{
				{
					Start: pointer.String("1 * * * *"),
					End:   pointer.String("2 * * * *"),
				},
			},
		}
		Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

		By("Step clock by 1 minute and check that shoot gets hibernated")
		fakeClock.Step(time.Minute)
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Spec.Hibernation.Enabled).To(PointTo(Equal(true)))
			g.Expect(shoot.Status.LastHibernationTriggerTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())

		By("Step clock by 1 minute and check that shoot gets woken up")
		fakeClock.Step(time.Minute)
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Spec.Hibernation.Enabled).To(PointTo(Equal(false)))
			g.Expect(shoot.Status.LastHibernationTriggerTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())
	})
})

func roundToNextHour(t time.Time) time.Time {
	tmpTime := t.Round(time.Hour)
	if tmpTime.Before(t) {
		return tmpTime.Add(time.Hour)
	}
	return tmpTime
}
