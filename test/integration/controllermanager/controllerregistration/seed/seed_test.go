// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerRegistration controller test", func() {
	var (
		shootProviderType           = "shootProvider"
		shoot                       *gardencorev1beta1.Shoot
		shootControllerRegistration *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("my-provider-account"),
				CloudProfileName:  ptr.To("test-cloudprofile"),
				Region:            "foo-region",
				Provider: gardencorev1beta1.Provider{
					Type: shootProviderType,
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    shootProviderType,
									Version: ptr.To("0.0.0"),
								},
							},
							CRI: &gardencorev1beta1.CRI{
								Name: gardencorev1beta1.CRINameContainerD,
								ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{
									Type: shootProviderType,
								}},
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.31.1",
				},
				Networking: &gardencorev1beta1.Networking{
					Type: ptr.To(shootProviderType),
				},
				Extensions: []gardencorev1beta1.Extension{{
					Type: shootProviderType,
				}},
				DNS: &gardencorev1beta1.DNS{
					Providers: []gardencorev1beta1.DNSProvider{{
						Type: &shootProviderType,
					}},
				},
				SeedName: &seed.Name,
			},
		}

		shootControllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "ctrlreg-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: extensionsv1alpha1.ControlPlaneResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.InfrastructureResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.WorkerResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.NetworkResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.ContainerRuntimeResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.DNSRecordResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.OperatingSystemConfigResource, Type: shootProviderType},
					{Kind: extensionsv1alpha1.ExtensionResource, Type: shootProviderType},
				},
			},
		}
	})

	Context("Seed", func() {
		It("should reconcile the ControllerInstallations", func() {
			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seedControllerRegistration), seedControllerRegistration)).To(Succeed())
				return seedControllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Expect ControllerInstallation to be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: seedControllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())
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
					Provider: gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "garden",
						Name:       "some-secret",
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
					Resources: []gardencorev1beta1.ControllerResource{{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType}},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
			log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

				By("Wait until manager has observed controllerregistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation to be created")
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
				g.Expect(controllerInstallationList.Items).To(BeEmpty())
			}).Should(Succeed())
		})

		It("should keep the ControllerInstallation because it is required", func() {
			obj := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider: gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "garden",
						Name:       "some-secret",
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
					Resources: []gardencorev1beta1.ControllerResource{{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType}},
				},
			}

			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
			log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

				By("Wait until manager has observed controllerregistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation to be created")
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
				g.Expect(controllerInstallationList.Items).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("BackupEntry", func() {
		It("should reconcile the ControllerInstallations", func() {
			backupBucket := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "bucket-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider: gardencorev1beta1.BackupBucketProvider{Type: providerType, Region: "region"},
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "garden",
						Name:       "some-secret",
					},
					SeedName: &seed.Name,
				},
			}

			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
			log.Info("Created BackupBucket for test", "controllerRegistration", client.ObjectKeyFromObject(backupBucket))

			By("Wait until manager has observed backupbucket creation")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
			}).Should(Succeed())

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
			log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

				By("Wait until manager has observed controllerregistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, obj)).To(Succeed())

			By("Expect ControllerInstallation to be created")
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
				g.Expect(controllerInstallationList.Items).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("Shoot", func() {
		It("should reconcile the ControllerInstallations", func() {
			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, shootControllerRegistration)).To(Succeed())

			DeferCleanup(func() {
				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, shootControllerRegistration)).To(Succeed())

				By("Wait until manager has observed shootControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootControllerRegistration), shootControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shootControllerRegistration), shootControllerRegistration)).To(Succeed())
				return shootControllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Create object")
			Expect(testClient.Create(ctx, shoot)).To(Succeed())

			By("Expect ControllerInstallation to be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: shootControllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: shootControllerRegistration.Name,
					core.SeedRefName:         seed.Name,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("ControllerRegistration w/o resources but deploy policy", func() {
		Context("'Always' policy", func() {
			policy := gardencorev1beta1.ControllerDeploymentPolicyAlways

			It("should delete ControllerInstallations when ControllerRegistration is deleted", func() {
				controllerRegistration := &gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "ctrlreg-" + testID + "-",
						Labels:       map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
							Policy: &policy,
						},
					},
				}

				By("Create ControllerRegistration")
				Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
				log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

				DeferCleanup(func() {
					By("Delete ControllerRegistration")
					Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

					By("Wait until manager has observed ControllerRegistration deletion")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
					}).Should(BeNotFoundError())
				})

				By("Expect finalizer to be added to ControllerRegistration")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
					return controllerRegistration.Finalizers
				}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

				By("Expect ControllerInstallation to be created")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				}).Should(Succeed())
			})
		})

		Context("'AlwaysExceptNoShoots' policy", func() {
			var (
				policy                 = gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots
				controllerRegistration *gardencorev1beta1.ControllerRegistration
			)

			BeforeEach(func() {
				controllerRegistration = &gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "ctrlreg-" + testID + "-",
						Labels:       map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
							Policy: &policy,
						},
					},
				}

				By("Create ControllerRegistration for Shoot")
				Expect(testClient.Create(ctx, shootControllerRegistration)).To(Succeed())

				DeferCleanup(func() {
					By("Delete ControllerRegistration for Shoot")
					Expect(testClient.Delete(ctx, shootControllerRegistration)).To(Succeed())

					By("Wait until manager has observed ControllerRegistration for Shoot deletion")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(shootControllerRegistration), shootControllerRegistration)
					}).Should(BeNotFoundError())
				})
			})

			It("should delete ControllerInstallations when ControllerRegistration is deleted (even if Shoots still exist)", func() {
				By("Create ControllerRegistration")
				Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
				log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

				DeferCleanup(func() {
					By("Delete ControllerRegistration")
					Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

					By("Wait until manager has observed ControllerRegistration deletion")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
					}).Should(BeNotFoundError())

					By("Delete Shoot")
					Expect(testClient.Delete(ctx, shoot)).To(Succeed())
				})

				By("Expect finalizer to be added to ControllerRegistration")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
					return controllerRegistration.Finalizers
				}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

				By("Expect no ControllerInstallation to be created because no Shoot exists")
				Consistently(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Create object")
				Expect(testClient.Create(ctx, shoot)).To(Succeed())

				By("Expect ControllerInstallation to be created")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				}).Should(Succeed())
			})

			It("should delete ControllerInstallations after all Shoots are gone", func() {
				By("Create ControllerRegistration")
				Expect(testClient.Create(ctx, controllerRegistration)).To(Succeed())
				log.Info("Created ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(controllerRegistration))

				DeferCleanup(func() {
					By("Delete ControllerRegistration")
					Expect(testClient.Delete(ctx, controllerRegistration)).To(Succeed())

					By("Wait until manager has observed ControllerRegistration deletion")
					Eventually(func() error {
						return mgrClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)
					}).Should(BeNotFoundError())
				})

				By("Expect finalizer to be added to ControllerRegistration")
				Eventually(func(g Gomega) []string {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
					return controllerRegistration.Finalizers
				}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

				By("Expect no ControllerInstallation to be created because no Shoot exists")
				Consistently(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Create Shoot")
				Expect(testClient.Create(ctx, shoot)).To(Succeed())

				By("Expect ControllerInstallation to be created")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				}).Should(Succeed())

				By("Delete Shoot")
				Expect(testClient.Delete(ctx, shoot)).To(Succeed())

				By("Expect ControllerInstallation be deleted")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: controllerRegistration.Name,
						core.SeedRefName:         seed.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())
			})
		})
	})
})
