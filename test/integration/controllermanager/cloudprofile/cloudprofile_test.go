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

package cloudprofile_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("CloudProfile controller tests", func() {
	var (
		resourceName string

		cloudProfile *gardencorev1beta1.CloudProfile
		shoot        *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(CurrentSpecReport().LeafNodeLocation.String()))[:8]

		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{Name: resourceName},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Type: "some-provider",
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{{Version: "1.2.3"}},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "some-image",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "4.5.6"}},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{{
					Name:   "some-type",
					CPU:    resource.MustParse("1"),
					GPU:    resource.MustParse("0"),
					Memory: resource.MustParse("1Gi"),
				}},
				Regions: []gardencorev1beta1.Region{
					{Name: "some-region"},
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName:  cloudProfile.Name,
				SecretBindingName: "my-provider-account",
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: cloudProfile.Spec.Type,
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{Type: "large"},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.21.1"},
				Networking: gardencorev1beta1.Networking{Type: "foo-networking"},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create CloudProfile")
		Expect(testClient.Create(ctx, cloudProfile)).To(Succeed())
		log.Info("Created CloudProfile for test", "cloudProfile", client.ObjectKeyFromObject(cloudProfile))

		DeferCleanup(func() {
			By("Delete CloudProfile")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, cloudProfile))).To(Succeed())
		})

		if shoot != nil {
			By("Create Shoot")
			Expect(testClient.Create(ctx, shoot)).To(Succeed())
			log.Info("Created shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

			DeferCleanup(func() {
				By("Delete Shoot")
				Expect(client.IgnoreNotFound(gardener.ConfirmDeletion(ctx, testClient, shoot))).To(Succeed())
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
			})
		}
	})

	Context("no shoot referencing the CloudProfile", func() {
		BeforeEach(func() {
			shoot = nil
		})

		It("should add the finalizer and release it on deletion", func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)).To(Succeed())
				g.Expect(cloudProfile.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete CloudProfile")
			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())

			By("Ensure finalizer got removed")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(BeNotFoundError())
		})
	})

	Context("shoots referencing the CloudProfile", func() {
		JustBeforeEach(func() {
			By("Ensure finalizer got added")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)).To(Succeed())
				g.Expect(cloudProfile.Finalizers).To(ConsistOf("gardener"))
			}).Should(Succeed())

			By("Delete CloudProfile")
			Expect(testClient.Delete(ctx, cloudProfile)).To(Succeed())
		})

		It("should add the finalizer and not release it on deletion since there still is a referencing shoot", func() {
			By("Ensure finalizer got removed")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(Succeed())
		})

		It("should add the finalizer and release it on deletion after the shoot got deleted", func() {
			By("Delete Shoot")
			Expect(gardener.ConfirmDeletion(ctx, testClient, shoot)).To(Succeed())
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Ensure finalizer got removed")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(cloudProfile), cloudProfile)
			}).Should(BeNotFoundError())
		})
	})
})
