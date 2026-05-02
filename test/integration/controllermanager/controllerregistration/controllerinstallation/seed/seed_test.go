// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ControllerInstallation-Seed controller test", func() {
	var (
		shootProviderType           = "shoot-provider"
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
						Type:           &shootProviderType,
						CredentialsRef: &autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"},
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

			By("Create Shoot")
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

	Context("Self-hosted shoot seed", func() {
		// When the seed is a self-hosted shoot cluster, the seed reconciler must subtract kind/type combinations
		// already managed by the shoot reconciler for the corresponding Shoot. ControllerInstallations for
		// extensions exclusively needed by the seed role are created with both .spec.seedRef (for seed gardenlet
		// cache visibility) and .spec.shootRef (for shoot gardenlet deployment). They are marked with the
		// SeedRefName label so that the shoot reconciler does not accidentally manage or delete them.
		// For shoot-owned CIs also needed by the seed, the seed reconciler patches .spec.seedRef onto them.
		// When the seed no longer needs a CI, seed-owned ones are deleted directly and shoot-owned ones have
		// .spec.seedRef cleared.

		var (
			selfHostedSeedName                    string
			selfHostedProviderType                string
			selfHostedShoot                       *gardencorev1beta1.Shoot
			selfHostedSeed                        *gardencorev1beta1.Seed
			selfHostedShootControllerRegistration *gardencorev1beta1.ControllerRegistration
		)

		BeforeEach(func() {
			selfHostedSeedName = "sh-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]
			selfHostedProviderType = "self-hosted-provider"

			By("Create garden namespace")
			gardenNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1beta1constants.GardenNamespace,
				},
			}
			Expect(testClient.Create(ctx, gardenNamespace)).To(Or(Succeed(), BeAlreadyExistsError()))

			selfHostedSeedNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenerutils.ComputeGardenNamespace(selfHostedSeedName),
				},
			}

			By("Create self-hosted Seed Namespace")
			Expect(testClient.Create(ctx, selfHostedSeedNamespace)).To(Succeed())

			DeferCleanup(func() {
				By("Delete self-hosted Seed Namespace")
				Expect(testClient.Delete(ctx, selfHostedSeedNamespace)).To(Or(Succeed(), BeNotFoundError()))
			})

			selfHostedSeedInternalDomainSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      internalDomainSecret.Name,
					Namespace: selfHostedSeedNamespace.Name,
				},
			}

			By("Create self-hosted Seed Internal Domain Secret")
			Expect(testClient.Create(ctx, selfHostedSeedInternalDomainSecret)).To(Succeed())

			DeferCleanup(func() {
				By("Delete self-hosted Seed Internal Domain Secret")
				Expect(testClient.Delete(ctx, selfHostedSeedInternalDomainSecret)).To(Or(Succeed(), BeNotFoundError()))
			})

			// selfHostedShootControllerRegistration covers all extension types used by the self-hosted Shoot itself.
			// The shoot reconciler will create ControllerInstallations for these (without SeedRefName label).
			selfHostedShootControllerRegistration = &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-self-hosted-shoot-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ControlPlaneResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.InfrastructureResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.WorkerResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.NetworkResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.ContainerRuntimeResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.OperatingSystemConfigResource, Type: selfHostedProviderType},
						{Kind: extensionsv1alpha1.ExtensionResource, Type: selfHostedProviderType},
					},
				},
			}

			By("Create self-hosted Shoot ControllerRegistration")
			Expect(testClient.Create(ctx, selfHostedShootControllerRegistration)).To(Succeed())
			log.Info("Created self-hosted Shoot ControllerRegistration for test", "controllerRegistration", client.ObjectKeyFromObject(selfHostedShootControllerRegistration))

			DeferCleanup(func() {
				By("Delete self-hosted Shoot ControllerRegistration")
				Expect(testClient.Delete(ctx, selfHostedShootControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed self-hosted Shoot ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(selfHostedShootControllerRegistration), selfHostedShootControllerRegistration)
				}).Should(BeNotFoundError())
			})

			// Create the Shoot before the Seed so that the seed reconciler sees the self-hosted Shoot from its
			// very first reconciliation.
			By("Create self-hosted Shoot in garden namespace")
			selfHostedShoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      selfHostedSeedName,
					Namespace: v1beta1constants.GardenNamespace,
					Labels:    map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ShootSpec{
					CloudProfile: &gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "test-cloudprofile",
					},
					Region: "foo-region",
					Provider: gardencorev1beta1.Provider{
						// Use a different provider type so that the shoot's required extensions do not overlap
						// with the seed's providerType extensions, leaving those for the seed reconciler to handle.
						Type: selfHostedProviderType,
						Workers: []gardencorev1beta1.Worker{{
							Name:         "control-plane",
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							Minimum:      1,
							Maximum:      1,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    selfHostedProviderType,
									Version: ptr.To("0.0.0"),
								},
							},
							CRI: &gardencorev1beta1.CRI{
								Name: gardencorev1beta1.CRINameContainerD,
								ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{
									Type: selfHostedProviderType,
								}},
							},
						}},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To(selfHostedProviderType),
					},
					Extensions: []gardencorev1beta1.Extension{
						{Type: selfHostedProviderType},
					},
				},
			}
			Expect(testClient.Create(ctx, selfHostedShoot)).To(Succeed())
			log.Info("Created self-hosted Shoot for test", "shoot", client.ObjectKeyFromObject(selfHostedShoot))

			DeferCleanup(func() {
				By("Delete self-hosted Shoot")
				Expect(testClient.Delete(ctx, selfHostedShoot)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for shoot-role ControllerInstallations to be cleaned up")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: selfHostedShootControllerRegistration.Name,
						core.ShootRefName:        selfHostedSeedName,
						core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())
			})

			By("Create self-hosted Seed")
			selfHostedSeed = seed.DeepCopy()
			selfHostedSeed.Name = selfHostedSeedName
			selfHostedSeed.ResourceVersion = ""
			selfHostedSeed.Labels = map[string]string{
				testID: testRunID,
				v1beta1constants.LabelSelfHostedShootCluster: "true",
			}
			selfHostedSeed.Spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConfig{
				Type:   providerType,
				Domain: "internal.example.com",
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       selfHostedSeedInternalDomainSecret.Name,
					Namespace:  selfHostedSeedNamespace.Name,
				},
			}

			Expect(testClient.Create(ctx, selfHostedSeed)).To(Succeed())
			log.Info("Created self-hosted Seed for test", "seed", client.ObjectKeyFromObject(selfHostedSeed))

			DeferCleanup(func() {
				By("Delete self-hosted Seed")
				Expect(testClient.Delete(ctx, selfHostedSeed)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed self-hosted Seed deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(selfHostedSeed), selfHostedSeed)
				}).Should(BeNotFoundError())
			})
		})

		It("should create ControllerInstallations with both refs for seed-role extensions, not interfering with shoot-role ControllerInstallations", func() {
			By("Expect seed-role ControllerInstallations to be created with both .spec.seedRef and .spec.shootRef and SeedRefName label (seed reconciler)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: seedControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				for _, item := range controllerInstallationList.Items {
					g.Expect(item.Spec.RegistrationRef.Name).To(Equal(seedControllerRegistration.Name), "seed-role ControllerInstallation %q must reference seedControllerRegistration", item.Name)
					g.Expect(item.Spec.SeedRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
						"Name": Equal(selfHostedSeedName),
					})), "seed-role ControllerInstallation %q must have .spec.seedRef", item.Name)
					g.Expect(item.Spec.ShootRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
						"Name":      Equal(selfHostedSeedName),
						"Namespace": Equal(v1beta1constants.GardenNamespace),
					})), "seed-role ControllerInstallation %q must reference the self-hosted shoot", item.Name)
					g.Expect(item.Labels).To(HaveKeyWithValue(controllerinstallation.SeedRefName, selfHostedSeedName), "seed-role ControllerInstallation %q must carry SeedRefName label", item.Name)
				}
			}).Should(Succeed())

			By("Expect shoot-role ControllerInstallations to be created with .spec.shootRef and no SeedRefName label (shoot reconciler)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: selfHostedShootControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				for _, item := range controllerInstallationList.Items {
					g.Expect(item.Spec.RegistrationRef.Name).To(Equal(selfHostedShootControllerRegistration.Name), "shoot-role ControllerInstallation %q must reference selfHostedShootControllerRegistration", item.Name)
					g.Expect(item.Spec.ShootRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
						"Name":      Equal(selfHostedSeedName),
						"Namespace": Equal(v1beta1constants.GardenNamespace),
					})), "shoot-role ControllerInstallation %q must reference the self-hosted shoot", item.Name)
					g.Expect(item.Spec.SeedRef).To(BeNil(), "shoot-role ControllerInstallation %q must not have .spec.seedRef", item.Name)
					g.Expect(item.Labels).NotTo(HaveKey(controllerinstallation.SeedRefName), "shoot-role ControllerInstallation %q must not carry SeedRefName label", item.Name)
				}
			}).Should(Succeed())
		})

		It("should hand over a seed-exclusive ControllerInstallation to the shoot reconciler by stripping the seed-ref-name label", func() {
			handoffType := "handoff-ext"

			By("Enable handoff extension on the self-hosted seed")
			patch := client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = []gardencorev1beta1.Extension{{Type: handoffType}}
			Expect(testClient.Patch(ctx, selfHostedSeed, patch)).To(Succeed())

			By("Create ControllerRegistration for the handoff extension")
			handoffControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-handoff-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:                extensionsv1alpha1.ExtensionResource,
							Type:                handoffType,
							WorkerlessSupported: ptr.To(true),
						},
					},
				},
			}
			Expect(testClient.Create(ctx, handoffControllerRegistration)).To(Succeed())
			log.Info("Created handoff ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(handoffControllerRegistration))

			DeferCleanup(func() {
				By("Revert self-hosted Seed extensions first to prevent the seed reconciler from re-creating the ControllerInstallation")
				seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
				selfHostedSeed.Spec.Extensions = nil
				Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Revert self-hosted Shoot extensions so the shoot reconciler deletes the handoff ControllerInstallation")
				shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
				selfHostedShoot.Spec.Extensions = []gardencorev1beta1.Extension{{Type: selfHostedProviderType}}
				Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for shoot reconciler to delete the handoff ControllerInstallation")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(mgrClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: handoffControllerRegistration.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Delete handoff ControllerRegistration")
				Expect(testClient.Delete(ctx, handoffControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed handoff ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(handoffControllerRegistration), handoffControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect seed-exclusive ControllerInstallation with seed-ref-name label")
			var handoffControllerInstallationName string
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: handoffControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Labels).To(HaveKeyWithValue(controllerinstallation.SeedRefName, selfHostedSeedName))
				handoffControllerInstallationName = controllerInstallationList.Items[0].Name
			}).Should(Succeed())

			By("Update self-hosted Shoot to also require the handoff extension type")
			shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
			selfHostedShoot.Spec.Extensions = append(selfHostedShoot.Spec.Extensions, gardencorev1beta1.Extension{Type: handoffType})
			Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Succeed())

			By("Ensure no additional ControllerInstallation is created for the handoff extension")
			Consistently(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: handoffControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Name).To(Equal(handoffControllerInstallationName))
			}).Should(Succeed())

			By("Expect seed-ref-name label to be removed from the same ControllerInstallation (ownership handed to shoot reconciler)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: handoffControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Name).To(Equal(handoffControllerInstallationName))
				g.Expect(controllerInstallationList.Items[0].Labels).NotTo(HaveKey(controllerinstallation.SeedRefName))
			}).Should(Succeed())

			By("Expect seed reconciler to re-patch .spec.seedRef onto the now shoot-owned ControllerInstallation (seed still needs it)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: handoffControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Name).To(Equal(handoffControllerInstallationName))
				g.Expect(controllerInstallationList.Items[0].Spec.SeedRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal(selfHostedSeedName),
				})))
			}).Should(Succeed())
		})

		It("should not create a duplicate ControllerInstallation when the shoot reconciler already installed the same ControllerRegistration", func() {
			adoptionShootExtType := "adopt-shoot-ext"
			adoptionSeedExtType := "adopt-seed-ext"

			By("Create ControllerRegistration that provides both a shoot-needed and a seed-needed extension")
			adoptionControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-adoption-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ExtensionResource, Type: adoptionShootExtType},
						{Kind: extensionsv1alpha1.ExtensionResource, Type: adoptionSeedExtType, WorkerlessSupported: ptr.To(true)},
					},
				},
			}
			Expect(testClient.Create(ctx, adoptionControllerRegistration)).To(Succeed())
			log.Info("Created adoption ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(adoptionControllerRegistration))

			DeferCleanup(func() {
				By("Revert self-hosted Seed extensions")
				seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
				selfHostedSeed.Spec.Extensions = nil
				Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Revert self-hosted Shoot extensions so the shoot reconciler deletes the ControllerInstallation")
				shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
				selfHostedShoot.Spec.Extensions = []gardencorev1beta1.Extension{{Type: selfHostedProviderType}}
				Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for the adoption ControllerInstallation to be cleaned up")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(mgrClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: adoptionControllerRegistration.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Delete adoption ControllerRegistration")
				Expect(testClient.Delete(ctx, adoptionControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed adoption ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(adoptionControllerRegistration), adoptionControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Update self-hosted Shoot to require the adoption shoot extension")
			shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
			selfHostedShoot.Spec.Extensions = append(selfHostedShoot.Spec.Extensions, gardencorev1beta1.Extension{Type: adoptionShootExtType})
			Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Succeed())

			By("Expect shoot reconciler to create a ControllerInstallation without SeedRefName label")
			var adoptionControllerInstallationName string
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: adoptionControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Labels).NotTo(HaveKey(controllerinstallation.SeedRefName))
				adoptionControllerInstallationName = controllerInstallationList.Items[0].Name
			}).Should(Succeed())

			By("Enable adoption seed extension on the self-hosted Seed so the seed reconciler also needs this registration")
			seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = []gardencorev1beta1.Extension{{Type: adoptionSeedExtType}}
			Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Succeed())

			By("Expect no duplicate ControllerInstallation to be created and .spec.seedRef to be patched onto the shoot-owned installation")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: adoptionControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1), "no duplicate ControllerInstallation should be created")
				g.Expect(controllerInstallationList.Items[0].Name).To(Equal(adoptionControllerInstallationName), "must be the same ControllerInstallation object")
				g.Expect(controllerInstallationList.Items[0].Labels).NotTo(HaveKey(controllerinstallation.SeedRefName), "seed reconciler must not add SeedRefName label to shoot-owned installation")
				g.Expect(controllerInstallationList.Items[0].Spec.SeedRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal(selfHostedSeedName),
				})), "seed reconciler must patch .spec.seedRef onto shoot-owned installation")
			}).Should(Succeed())
		})

		It("should delete seed-owned ControllerInstallations directly when the seed no longer needs them", func() {
			deleteType := "delete-ext"

			By("Enable delete extension on the self-hosted seed")
			patch := client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = []gardencorev1beta1.Extension{{Type: deleteType}}
			Expect(testClient.Patch(ctx, selfHostedSeed, patch)).To(Succeed())

			By("Create ControllerRegistration for the delete extension")
			deleteControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-delete-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:                extensionsv1alpha1.ExtensionResource,
							Type:                deleteType,
							WorkerlessSupported: ptr.To(true),
						},
					},
				},
			}
			Expect(testClient.Create(ctx, deleteControllerRegistration)).To(Succeed())
			log.Info("Created delete ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(deleteControllerRegistration))

			DeferCleanup(func() {
				By("Revert self-hosted Seed extensions")
				seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
				selfHostedSeed.Spec.Extensions = nil
				Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Delete delete ControllerRegistration")
				Expect(testClient.Delete(ctx, deleteControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed delete ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(deleteControllerRegistration), deleteControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect seed-owned ControllerInstallation to be created with SeedRefName label")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: deleteControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Labels).To(HaveKeyWithValue(controllerinstallation.SeedRefName, selfHostedSeedName))
			}).Should(Succeed())

			By("Remove the extension from the seed so it is no longer needed")
			patch = client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = nil
			Expect(testClient.Patch(ctx, selfHostedSeed, patch)).To(Succeed())

			By("Expect seed-owned ControllerInstallation to be deleted directly")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: deleteControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(BeEmpty())
			}).Should(Succeed())
		})

		It("should set .spec.seedRef on shoot-owned ControllerInstallations when the kind/type is shared between the self-hosted shoot and a hosted shoot", func() {
			sharedExtType := "shared-ext"

			By("Create ControllerRegistration for the shared extension type")
			sharedControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-shared-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ExtensionResource, Type: sharedExtType, WorkerlessSupported: ptr.To(true)},
					},
				},
			}
			Expect(testClient.Create(ctx, sharedControllerRegistration)).To(Succeed())
			log.Info("Created shared ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(sharedControllerRegistration))

			DeferCleanup(func() {
				By("Revert self-hosted Shoot extensions")
				shootRevert := client.MergeFrom(selfHostedShoot.DeepCopy())
				selfHostedShoot.Spec.Extensions = []gardencorev1beta1.Extension{{Type: selfHostedProviderType}}
				Expect(testClient.Patch(ctx, selfHostedShoot, shootRevert)).To(Or(Succeed(), BeNotFoundError()))

				By("Delete hosted shoot")
				hostedShoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "hosted-" + selfHostedSeedName, Namespace: testNamespace.Name}}
				Expect(testClient.Delete(ctx, hostedShoot)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for the shared ControllerInstallation to be cleaned up")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(mgrClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: sharedControllerRegistration.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Wait for hosted shoot to be fully deleted")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(hostedShoot), hostedShoot)
				}).Should(BeNotFoundError())

				By("Delete shared ControllerRegistration")
				Expect(testClient.Delete(ctx, sharedControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed shared ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(sharedControllerRegistration), sharedControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Update self-hosted Shoot to use the shared extension type")
			shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
			selfHostedShoot.Spec.Extensions = append(selfHostedShoot.Spec.Extensions, gardencorev1beta1.Extension{Type: sharedExtType})
			Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Succeed())

			By("Create a hosted Shoot on the self-hosted seed that also uses the shared extension type")
			hostedShoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hosted-" + selfHostedSeedName,
					Namespace: testNamespace.Name,
					Labels:    map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: ptr.To("my-provider-account"),
					CloudProfile: &gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "test-cloudprofile",
					},
					Region: "foo-region",
					Provider: gardencorev1beta1.Provider{
						Type: selfHostedProviderType,
						Workers: []gardencorev1beta1.Worker{{
							Name:    "worker",
							Minimum: 1,
							Maximum: 1,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    selfHostedProviderType,
									Version: ptr.To("0.0.0"),
								},
							},
							CRI: &gardencorev1beta1.CRI{
								Name: gardencorev1beta1.CRINameContainerD,
								ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{
									Type: selfHostedProviderType,
								}},
							},
						}},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
					},
					Networking: &gardencorev1beta1.Networking{
						Type: ptr.To(selfHostedProviderType),
					},
					Extensions: []gardencorev1beta1.Extension{
						{Type: sharedExtType},
					},
					SeedName: &selfHostedSeedName,
				},
			}
			Expect(testClient.Create(ctx, hostedShoot)).To(Succeed())
			log.Info("Created hosted Shoot on self-hosted seed", "shoot", client.ObjectKeyFromObject(hostedShoot))

			By("Expect shoot reconciler to create a ControllerInstallation for the shared registration (for the self-hosted shoot)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: sharedControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Expect seed reconciler to patch .spec.seedRef because the hosted shoot also needs this kind/type on the seed")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: sharedControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Spec.SeedRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": Equal(selfHostedSeedName),
				})), "seed reconciler must set .spec.seedRef when hosted shoots also need this kind/type")
				g.Expect(controllerInstallationList.Items[0].Spec.ShootRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Name":      Equal(selfHostedSeedName),
					"Namespace": Equal(v1beta1constants.GardenNamespace),
				})), "spec.shootRef must remain for the shoot gardenlet")
			}).Should(Succeed())
		})

		It("should clear .spec.seedRef from shoot-owned ControllerInstallations when the seed no longer needs them", func() {
			clearType := "clear-ext"

			By("Update self-hosted Shoot to require the clear extension")
			shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
			selfHostedShoot.Spec.Extensions = append(selfHostedShoot.Spec.Extensions, gardencorev1beta1.Extension{Type: clearType})
			Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Succeed())

			By("Create ControllerRegistration for the clear extension")
			clearControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-clear-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:                extensionsv1alpha1.ExtensionResource,
							Type:                clearType,
							WorkerlessSupported: ptr.To(true),
						},
					},
				},
			}
			Expect(testClient.Create(ctx, clearControllerRegistration)).To(Succeed())
			log.Info("Created clear ControllerRegistration", "controllerRegistration", client.ObjectKeyFromObject(clearControllerRegistration))

			DeferCleanup(func() {
				By("Revert self-hosted Seed extensions")
				seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
				selfHostedSeed.Spec.Extensions = nil
				Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Or(Succeed(), BeNotFoundError()))

				By("Revert self-hosted Shoot extensions")
				shootRevert := client.MergeFrom(selfHostedShoot.DeepCopy())
				selfHostedShoot.Spec.Extensions = []gardencorev1beta1.Extension{{Type: selfHostedProviderType}}
				Expect(testClient.Patch(ctx, selfHostedShoot, shootRevert)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for the clear ControllerInstallation to be cleaned up")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(mgrClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: clearControllerRegistration.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Delete clear ControllerRegistration")
				Expect(testClient.Delete(ctx, clearControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed clear ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(clearControllerRegistration), clearControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect shoot-owned ControllerInstallation to be created by shoot reconciler")
			var clearControllerInstallationName string
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: clearControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Labels).NotTo(HaveKey(controllerinstallation.SeedRefName))
				clearControllerInstallationName = controllerInstallationList.Items[0].Name
			}).Should(Succeed())

			By("Enable the clear extension on the seed so it also needs this registration")
			seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = []gardencorev1beta1.Extension{{Type: clearType}}
			Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Succeed())

			By("Expect seed reconciler to patch .spec.seedRef onto the shoot-owned ControllerInstallation")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: clearControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				g.Expect(controllerInstallationList.Items[0].Name).To(Equal(clearControllerInstallationName))
			}).Should(Succeed())

			By("Remove the extension from the seed so it is no longer needed by the seed")
			seedPatch = client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = nil
			Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Succeed())

			By("Expect .spec.seedRef to be cleared (ControllerInstallation remains for the shoot)")
			Eventually(func(g Gomega) {
				ci := &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: clearControllerInstallationName}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ci), ci)).To(Succeed())
				g.Expect(ci.Spec.SeedRef).To(BeNil(), "spec.seedRef must be cleared when seed no longer needs the CI")
				g.Expect(ci.Spec.ShootRef).NotTo(BeNil(), "spec.shootRef must remain for the shoot gardenlet")
			}).Should(Succeed())
		})

		It("should release the cluster finalizer on the seed when .spec.seedRef is cleared from ControllerInstallations", func() {
			clearFinalizerType := "clear-finalizer-ext"

			By("Update self-hosted Shoot to require the extension")
			shootPatch := client.MergeFrom(selfHostedShoot.DeepCopy())
			selfHostedShoot.Spec.Extensions = append(selfHostedShoot.Spec.Extensions, gardencorev1beta1.Extension{Type: clearFinalizerType})
			Expect(testClient.Patch(ctx, selfHostedShoot, shootPatch)).To(Succeed())

			By("Create ControllerRegistration for the extension")
			finalizerControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-clear-finalizer-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:                extensionsv1alpha1.ExtensionResource,
							Type:                clearFinalizerType,
							WorkerlessSupported: ptr.To(true),
						},
					},
				},
			}
			Expect(testClient.Create(ctx, finalizerControllerRegistration)).To(Succeed())

			DeferCleanup(func() {
				By("Revert self-hosted Shoot extensions")
				shootRevert := client.MergeFrom(selfHostedShoot.DeepCopy())
				selfHostedShoot.Spec.Extensions = []gardencorev1beta1.Extension{{Type: selfHostedProviderType}}
				Expect(testClient.Patch(ctx, selfHostedShoot, shootRevert)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait for the ControllerInstallation to be cleaned up")
				Eventually(func(g Gomega) {
					controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
					g.Expect(mgrClient.List(ctx, controllerInstallationList, client.MatchingFields{
						core.RegistrationRefName: finalizerControllerRegistration.Name,
					})).To(Succeed())
					g.Expect(controllerInstallationList.Items).To(BeEmpty())
				}).Should(Succeed())

				By("Delete ControllerRegistration")
				Expect(testClient.Delete(ctx, finalizerControllerRegistration)).To(Or(Succeed(), BeNotFoundError()))

				By("Wait until manager has observed ControllerRegistration deletion")
				Eventually(func() error {
					return mgrClient.Get(ctx, client.ObjectKeyFromObject(finalizerControllerRegistration), finalizerControllerRegistration)
				}).Should(BeNotFoundError())
			})

			By("Expect shoot-owned ControllerInstallation to be created")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: finalizerControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Enable the extension on the seed")
			seedPatch := client.MergeFrom(selfHostedSeed.DeepCopy())
			selfHostedSeed.Spec.Extensions = []gardencorev1beta1.Extension{{Type: clearFinalizerType}}
			Expect(testClient.Patch(ctx, selfHostedSeed, seedPatch)).To(Succeed())

			By("Wait for seed reconciler to patch .spec.seedRef")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: finalizerControllerRegistration.Name,
					core.SeedRefName:         selfHostedSeedName,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
			}).Should(Succeed())

			By("Verify cluster finalizer is on the seed")
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(selfHostedSeed), selfHostedSeed)).To(Succeed())
			Expect(selfHostedSeed.Finalizers).To(ContainElement("core.gardener.cloud/controllerregistration"))

			By("Delete the self-hosted seed (triggers seed reconciler to clear .spec.seedRef on CIs)")
			Expect(testClient.Delete(ctx, selfHostedSeed)).To(Succeed())

			By("Expect the cluster finalizer to be released even though the ControllerInstallation still exists (with only .spec.shootRef)")
			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(selfHostedSeed), selfHostedSeed)
				g.Expect(err).To(BeNotFoundError())
			}).Should(Succeed())

			By("Re-create the self-hosted seed for other tests")
			selfHostedSeed = seed.DeepCopy()
			selfHostedSeed.Name = selfHostedSeedName
			selfHostedSeed.ResourceVersion = ""
			selfHostedSeed.Labels = map[string]string{
				testID: testRunID,
				v1beta1constants.LabelSelfHostedShootCluster: "true",
			}
			selfHostedSeed.Spec.DNS.Internal = &gardencorev1beta1.SeedDNSProviderConfig{
				Type:   providerType,
				Domain: "internal.example.com",
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       internalDomainSecret.Name,
					Namespace:  gardenerutils.ComputeGardenNamespace(selfHostedSeedName),
				},
			}
			Expect(testClient.Create(ctx, selfHostedSeed)).To(Succeed())
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
