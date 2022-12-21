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

package shoot_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot migration controller tests", func() {
	var (
		sourceSeed *gardencorev1beta1.Seed
		shoot      *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now().Round(time.Second))

		By("Create source seed")
		sourceSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "seed-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
				},
				DNS: gardencorev1beta1.SeedDNS{
					IngressDomain: pointer.String("someotheringress.example.com"),
				},
				Settings: &gardencorev1beta1.SeedSettings{
					OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
						Enabled: true,
					},
				},
			},
		}
		Expect(testClient.Create(ctx, sourceSeed)).To(Succeed())
		log.Info("Created source Seed for migration", "seed", sourceSeed.Name)

		DeferCleanup(func() {
			By("Delete source seed")
			Expect(testClient.Delete(ctx, sourceSeed)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Create Shoot for migration")
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
				Annotations:  map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName:          pointer.String(sourceSeed.Name),
				SecretBindingName: "my-provider-account",
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
					Version: "1.20.1",
				},
				Networking: gardencorev1beta1.Networking{
					Type: "foo-networking",
				},
			},
		}

		Expect(testClient.Create(ctx, shoot)).To(Succeed())
		log.Info("Created Shoot for test", "shoot", client.ObjectKeyFromObject(shoot))

		DeferCleanup(func() {
			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
		})

		By("Patch .status.seedName in Shoot to " + sourceSeed.Name)
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.SeedName = pointer.String(sourceSeed.Name)
		Expect(testClient.Status().Patch(ctx, shoot, patch)).To(Succeed())

		By("Patch .spec.seedName in Shoot to " + seed.Name)
		shoot.Spec.SeedName = pointer.String(seed.Name)
		err := testClient.SubResource("binding").Update(ctx, shoot)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should set migration start time", func() {
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.MigrationStartTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())
	})

	It("should update the Shoot status to force the restoration if forceRestore annotation is present", func() {
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		patch := client.MergeFrom(shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootForceRestore, "true")
		Expect(testClient.Patch(ctx, shoot, patch)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.MigrationStartTime).To(BeNil())
			g.Expect(shoot.Status.SeedName).To(BeNil())
			g.Expect(shoot.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
			g.Expect(shoot.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateAborted))
			g.Expect(shoot.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationShootForceRestore))
		}).Should(Succeed())
	})

	It("should update the Shoot status to force the restoration if grace period is elapsed", func() {
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.MigrationStartTime).To(PointTo(Equal(metav1.Time{Time: fakeClock.Now()})))
		}).Should(Succeed())

		fakeClock.Step(gracePeriod + time.Second)

		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			g.Expect(shoot.Status.MigrationStartTime).To(BeNil())
			g.Expect(shoot.Status.SeedName).To(BeNil())
			g.Expect(shoot.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
			g.Expect(shoot.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateAborted))
		}).Should(Succeed())
	})
})
