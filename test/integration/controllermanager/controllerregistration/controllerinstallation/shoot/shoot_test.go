// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
					Namespace:    shoot.Namespace,
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

	Context("ControllerDeployment with resource references", func() {
		var (
			referencedSecret     *corev1.Secret
			referencedConfigMap  *corev1.ConfigMap
			controllerDeployment *gardencorev1.ControllerDeployment
			ctrlReg              *gardencorev1beta1.ControllerRegistration
			backupBucket         *gardencorev1beta1.BackupBucket
		)

		BeforeEach(func() {
			referencedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ref-secret-",
					Namespace:    v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						testID:                      testRunID,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleResourceReference,
					},
				},
				Data: map[string][]byte{"token": []byte("s3cret")},
			}
			By("Create referenced Secret")
			Expect(testClient.Create(ctx, referencedSecret)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, referencedSecret))).To(Succeed())
			})

			referencedConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ref-configmap-",
					Namespace:    v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						testID:                      testRunID,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleResourceReference,
					},
				},
				Data: map[string]string{"endpoint": "https://example.com"},
			}
			By("Create referenced ConfigMap")
			Expect(testClient.Create(ctx, referencedConfigMap)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, referencedConfigMap))).To(Succeed())
			})

			controllerDeployment = &gardencorev1.ControllerDeployment{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrldeploy-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Helm: &gardencorev1.HelmControllerDeployment{
					RawChart: []byte("not-a-real-chart"),
				},
				Resources: []gardencorev1.NamedResourceReference{
					{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: referencedSecret.Name}},
					{Name: "cfg", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: referencedConfigMap.Name}},
				},
			}
			By("Create ControllerDeployment")
			Expect(testClient.Create(ctx, controllerDeployment)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerDeployment))).To(Succeed())
			})

			ctrlReg = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-" + testID + "-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.BackupBucketResource, Type: providerType},
					},
					Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
						DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: controllerDeployment.Name}},
					},
				},
			}
			By("Create ControllerRegistration")
			Expect(testClient.Create(ctx, ctrlReg)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, ctrlReg))).To(Succeed())

				By("Wait until manager has observed ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(ctrlReg), ctrlReg)
				}).Should(BeNotFoundError())
			})

			backupBucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "bucket-" + testID + "-",
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
			By("Create BackupBucket")
			Expect(testClient.Create(ctx, backupBucket)).To(Succeed())
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, backupBucket))).To(Succeed())
			})
		})

		It("should write ResourceRefs to the ControllerInstallation based on the ControllerDeployment Resources", func() {
			By("Expect ControllerInstallation to be created with ResourceRefs")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: ctrlReg.Name,
					core.ShootRefName:        shoot.Name,
					core.ShootRefNamespace:   shoot.Namespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))

				ci := controllerInstallationList.Items[0]
				g.Expect(ci.Spec.DeploymentRef).NotTo(BeNil())
				g.Expect(ci.Spec.DeploymentRef.Name).To(Equal(controllerDeployment.Name))
				g.Expect(ci.Spec.ResourceRefs).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Kind":      Equal("Secret"),
						"Name":      Equal(referencedSecret.Name),
						"Namespace": Equal(v1beta1constants.GardenNamespace),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Kind":      Equal("ConfigMap"),
						"Name":      Equal(referencedConfigMap.Name),
						"Namespace": Equal(v1beta1constants.GardenNamespace),
					}),
				))

				var secretRefVersion, configMapRefVersion string
				for _, ref := range ci.Spec.ResourceRefs {
					switch ref.Kind {
					case "Secret":
						secretRefVersion = ref.ResourceVersion
					case "ConfigMap":
						configMapRefVersion = ref.ResourceVersion
					}
				}

				updatedSecret := &corev1.Secret{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(referencedSecret), updatedSecret)).To(Succeed())
				g.Expect(secretRefVersion).To(Equal(updatedSecret.ResourceVersion))

				updatedConfigMap := &corev1.ConfigMap{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(referencedConfigMap), updatedConfigMap)).To(Succeed())
				g.Expect(configMapRefVersion).To(Equal(updatedConfigMap.ResourceVersion))
			}).Should(Succeed())
		})
	})
})
