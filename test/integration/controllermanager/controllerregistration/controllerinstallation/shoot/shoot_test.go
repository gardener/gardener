// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation-Shoot controller test", func() {
	Context("Shoot", func() {
		It("should reconcile the ControllerInstallations", func() {
			By("Expect finalizer to be added to ControllerRegistration")
			Eventually(func(g Gomega) []string {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistration), controllerRegistration)).To(Succeed())
				return controllerRegistration.Finalizers
			}).Should(ConsistOf("core.gardener.cloud/controllerregistration"))

			By("Expect ControllerInstallation to be created")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(HaveLen(1))
		})

		It("should do nothing for hosted shoots", func() {
			hostedShoot := shoot.DeepCopy()
			hostedShoot.Spec.Provider.Workers[0].ControlPlane = nil
			hostedShoot.Name = ""
			hostedShoot.ResourceVersion = ""

			By("Delete Shoot")
			Expect(testClient.Delete(ctx, shoot)).To(Succeed())
			DeferCleanup(func() {
				shoot.ResourceVersion = ""
				Expect(testClient.Create(ctx, shoot)).To(Succeed())
			})

			By("Wait for ControllerInstallation to disappear")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(BeEmpty())

			By("Create hosted Shoot")
			Expect(testClient.Create(ctx, hostedShoot)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, hostedShoot)).To(Succeed())
			})

			By("Ensure no ControllerInstallations get created")
			Consistently(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(BeEmpty())
		})
	})

	Context("BackupBucket", func() {
		var backupBucket *gardencorev1beta1.BackupBucket

		BeforeEach(func() {
			backupBucket = &gardencorev1beta1.BackupBucket{
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
					ShootRef: &corev1.ObjectReference{
						APIVersion: "core.gardener.cloud/v1beta1",
						Kind:       "Shoot",
						Name:       shoot.Name,
						Namespace:  shoot.Namespace,
					},
				},
			}
		})

		It("should reconcile the ControllerInstallations", func() {
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

			By("Create BackupBucket")
			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

			By("Expect ControllerInstallation to be created")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(HaveLen(1))

			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(BeEmpty())
		})

		It("should keep the ControllerInstallation because it is required", func() {
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

			By("Create BackupBucket")
			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())

			By("Expect ControllerInstallation to be created")
			controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
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

			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("Expect ControllerInstallation not be deleted")
			Consistently(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(HaveLen(1))

			By("Mark ControllerInstallation as 'not required'")
			patch = client.MergeFrom(controllerInstallation.DeepCopy())
			controllerInstallation.Status.Conditions = nil
			Expect(testClient.Status().Patch(ctx, controllerInstallation, patch)).To(Succeed())

			By("Expect ControllerInstallation to be deleted")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(BeEmpty())
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
					ShootRef: &corev1.ObjectReference{
						APIVersion: "core.gardener.cloud/v1beta1",
						Kind:       "Shoot",
						Name:       shoot.Name,
						Namespace:  shoot.Namespace,
					},
				},
			}

			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
			log.Info("Created BackupBucket for test", "controllerRegistration", client.ObjectKeyFromObject(backupBucket))

			By("Wait until manager has observed BackupBucket creation")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket)
			}).Should(Succeed())

			backupEntry := &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testID + "-",
					Namespace:    testNamespace.Name,
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					BucketName: backupBucket.Name,
					ShootRef: &corev1.ObjectReference{
						APIVersion: "core.gardener.cloud/v1beta1",
						Kind:       "Shoot",
						Name:       shoot.Name,
						Namespace:  shoot.Namespace,
					},
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

			By("Create BackupEntry")
			Expect(testClient.Create(ctx, backupEntry)).To(Succeed())

			By("Expect ControllerInstallation to be created")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(HaveLen(1))

			By("Delete BackupEntry")
			Expect(testClient.Delete(ctx, backupEntry)).To(Succeed())

			By("Delete BackupBucket")
			Expect(testClient.Delete(ctx, backupBucket)).To(Succeed())

			By("Expect ControllerInstallation be deleted")
			Eventually(func(g Gomega) []gardencorev1beta1.ControllerInstallation {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: controllerRegistration.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				return controllerInstallationList.Items
			}).Should(BeEmpty())
		})
	})
})
