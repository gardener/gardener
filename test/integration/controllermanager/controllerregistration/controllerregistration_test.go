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

package controllerregistration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerRegistration controller test", func() {
	var (
		providerType    = "provider"
		dnsProviderType = "dnsProvider"

		seed                   *gardencorev1beta1.Seed
		seedNamespace          *corev1.Namespace
		seedSecret             *corev1.Secret
		controllerRegistration *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   providerType,
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "seed.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: dnsProviderType,
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Settings: &gardencorev1beta1.SeedSettings{
					Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    pointer.String("10.2.0.0/16"),
					ShootDefaults: &gardencorev1beta1.ShootNetworks{
						Pods:     pointer.String("100.128.0.0/11"),
						Services: pointer.String("100.72.0.0/13"),
					},
				},
			},
		}

		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

		seedNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenerutils.ComputeGardenNamespace(seed.Name),
			},
		}

		seedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-secret",
				Namespace: seedNamespace.Name,
				Labels:    map[string]string{"gardener.cloud/role": "global-monitoring"},
			},
		}

		By("Create Seed Namespace")
		Expect(testClient.Create(ctx, seedNamespace)).To(Succeed())
		log.Info("Created Seed Namespace for test", "namespace", client.ObjectKeyFromObject(seedNamespace))

		By("Create Seed Secret")
		Expect(testClient.Create(ctx, seedSecret)).To(Succeed())
		log.Info("Created Seed Secret for test", "secret", client.ObjectKeyFromObject(seedSecret))

		Eventually(func(g Gomega) []string {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			return seed.Finalizers
		}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlreg-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{{Kind: extensionsv1alpha1.DNSRecordResource, Type: dnsProviderType}},
			},
		}

		By("Create ControllerRegistration")
		Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(testClient.Delete(ctx, seed)).To(Or(Succeed(), BeNotFoundError()))

			By("Delete ControllerInstallations")
			Expect(testClient.DeleteAllOf(ctx, &gardencorev1beta1.ControllerInstallation{})).To(Or(Succeed(), BeNotFoundError()))

			By("Delete ControllerRegistrations")
			Expect(testClient.DeleteAllOf(ctx, &gardencorev1beta1.ControllerRegistration{})).To(Or(Succeed(), BeNotFoundError()))

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
			}).Should(BeNotFoundError())

			By("Delete Seed Secret")
			Expect(testClient.Delete(ctx, seedSecret)).To(Or(Succeed(), BeNotFoundError()))

			By("Delete Seed Namespace")
			Expect(testClient.Delete(ctx, seedNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	Context("Seed", func() {
		It("should reconcile the ControllerInstallations", func() {
			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Expect ControllerInstallation be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Delete object")
			Expect(testClient.Delete(ctx, seed)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(0))
			}).Should(Succeed())

			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Expect ControllerRegistration to be deleted")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})
	})

	Context("BackupBucket", func() {
		It("should reconcile the ControllerInstallations", func() {
			obj := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider:  gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					SecretRef: corev1.SecretReference{Name: "some-secret", Namespace: "garden"},
					SeedName:  &seed.Name,
				},
			}

			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType}},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Delete object")
			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(0))
			}).Should(Succeed())

			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Expect ControllerRegistration to be deleted")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})

		It("should keep the ControllerInstallation because it is required", func() {
			obj := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider:  gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					SecretRef: corev1.SecretReference{Name: "some-secret", Namespace: "garden"},
					SeedName:  &seed.Name,
				},
			}

			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType}},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be created")
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				controllerInstallation = &controllerInstallationList.Items[0]
			}).Should(Succeed())

			By("Mark ControllerInstallation as 'required'")
			patch := client.MergeFrom(controllerInstallation.DeepCopy())
			controllerInstallation.Status.Conditions = []gardencorev1beta1.Condition{
				{Type: gardencorev1beta1.ControllerInstallationRequired, Status: gardencorev1beta1.ConditionTrue},
			}
			Expect(testClient.Status().Patch(ctx, controllerInstallation, patch)).To(Succeed())

			By("Delete object")
			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation not be deleted")
			Consistently(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Mark ControllerInstallation as 'not required'")
			patch = client.MergeFrom(controllerInstallation.DeepCopy())
			controllerInstallation.Status.Conditions = nil
			Expect(testClient.Status().Patch(ctx, controllerInstallation, patch)).To(Succeed())

			By("Expect ControllerInstallation to be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(0))
			}).Should(Succeed())

			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Expect ControllerRegistration to be deleted")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})
	})

	Context("BackupEntry", func() {
		It("should reconcile the ControllerInstallations", func() {
			backupBucket := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "bucket-",
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider:  gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					SecretRef: corev1.SecretReference{Name: "some-secret", Namespace: "garden"},
					SeedName:  &seed.Name,
				},
			}

			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

			obj := &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
					SeedName:   &seed.Name,
				},
			}

			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType},
						{Kind: extensionsv1alpha1.BackupEntryResource, Type: providerType},
					},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("Delete object")
			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(0))
			}).Should(Succeed())

			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Expect ControllerRegistration to be deleted")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})
	})

	Context("Shoot", func() {
		It("should reconcile the ControllerInstallations", func() {
			obj := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: "my-provider-account",
					CloudProfileName:  "test-cloudprofile",
					Region:            "foo-region",
					Provider: gardencorev1beta1.Provider{
						Type: providerType,
						Workers: []gardencorev1beta1.Worker{
							{
								Name:    "cpu-worker",
								Minimum: 2,
								Maximum: 2,
								Machine: gardencorev1beta1.Machine{
									Type: "large",
									Image: &gardencorev1beta1.ShootMachineImage{
										Name:    providerType,
										Version: pointer.String("0.0.0"),
									},
								},
								CRI: &gardencorev1beta1.CRI{
									Name: gardencorev1beta1.CRINameContainerD,
									ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{
										Type: providerType,
									}},
								},
							},
						},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.21.1",
					},
					Networking: gardencorev1beta1.Networking{
						Type: providerType,
					},
					Extensions: []gardencorev1beta1.Extension{{
						Type: providerType,
					}},
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{{
							Type: &providerType,
						}},
					},
					SeedName: &seed.Name,
				},
			}

			controllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ControlPlaneResource, Type: providerType},
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: providerType},
						{Kind: extensionsv1alpha1.WorkerResource, Type: providerType},
						{Kind: extensionsv1alpha1.NetworkResource, Type: providerType},
						{Kind: extensionsv1alpha1.ContainerRuntimeResource, Type: providerType},
						{Kind: extensionsv1alpha1.DNSRecordResource, Type: providerType},
						{Kind: extensionsv1alpha1.OperatingSystemConfigResource, Type: providerType},
						{Kind: extensionsv1alpha1.ExtensionResource, Type: providerType},
					},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Delete object")
			Expect(testClient.Delete(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(0))
			}).Should(Succeed())

			By("Delete ControllerRegistration")
			Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

			By("Expect ControllerRegistration to be deleted")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
			}).Should(BeNotFoundError())
		})
	})
})
