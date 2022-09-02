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

package statuslabel_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot Quota controller tests", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: "secretbinding",
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
				DNS: &gardencorev1beta1.DNS{
					Domain: pointer.String("some-domain.example.com"),
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.20.1",
				},
				Networking: gardencorev1beta1.Networking{
					Type: "foo-networking",
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

		Eventually(func(g Gomega) string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			return shoot.Labels["shoot.gardener.cloud/status"]
		}).Should(Equal("healthy"))
	})

	It("should set the status to healthy because lastOperation is nil", func() {})

	Context("creation (unfinished)", func() {
		BeforeEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status = gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateProcessing,
				},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should set the status to healthy because there are no last errors", func() {
			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("healthy"))
		})

		It("should set the status to unhealthy because there are last errors", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastErrors = []gardencorev1beta1.LastError{{}}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unhealthy"))
		})
	})

	Context("deletion", func() {
		BeforeEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status = gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeDelete,
				},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should set the status to healthy because there are no last errors", func() {
			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("healthy"))
		})

		It("should set the status to unhealthy because there are last errors", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.LastErrors = []gardencorev1beta1.LastError{{}}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unhealthy"))
		})
	})

	Context("no active reconciliation", func() {
		BeforeEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status = gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should set the status to healthy because the conditions are true", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionTrue},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("healthy"))
		})

		It("should set the status to progressing because a conditions is progressing", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionProgressing},
				{Status: gardencorev1beta1.ConditionTrue},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("progressing"))
		})

		It("should set the status to unhealthy because a conditions is false", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionFalse},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unhealthy"))
		})

		It("should set the status to unhealthy because a conditions is unknown", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionUnknown},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unknown"))
		})
	})

	Context("active reconciliation", func() {
		BeforeEach(func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status = gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateProcessing,
				},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())
		})

		It("should set the status to unhealthy because the conditions are true but there are last errors", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionTrue},
			}
			shoot.Status.LastErrors = []gardencorev1beta1.LastError{{}}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unhealthy"))
		})

		It("should set the status to unhealthy because the conditions are false even though there are no last errors", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionFalse},
				{Status: gardencorev1beta1.ConditionFalse},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("unhealthy"))
		})

		It("should set the status to healthy because the conditions are true and there are no last errors", func() {
			patch := client.MergeFrom(shoot.DeepCopy())
			shoot.Status.Conditions = []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionTrue},
			}
			Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

			Eventually(func(g Gomega) string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				return shoot.Labels["shoot.gardener.cloud/status"]
			}).Should(Equal("healthy"))
		})
	})
})
