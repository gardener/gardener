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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
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
		// extensions exclusively needed by the seed role are created with .spec.shootRef (not .spec.seedRef) so
		// that the shoot gardenlet can manage them. They are marked with the SeedRefName label so that the shoot
		// reconciler does not accidentally manage or delete them.
		It("should create ControllerInstallations with .spec.shootRef for seed-role extensions, not interfering with shoot-role ControllerInstallations", func() {
			selfHostedSeedName := "self-hosted-" + seedName
			selfHostedProviderType := "self-hosted-provider"

			By("Create garden namespace")
			gardenNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1beta1constants.GardenNamespace,
				},
			}
			Expect(testClient.Create(ctx, gardenNamespace)).To(Or(Succeed(), BeAlreadyExistsError()))

			DeferCleanup(func() {
				By("Delete garden namespace")
				Expect(testClient.Delete(ctx, gardenNamespace)).To(Or(Succeed(), BeNotFoundError()))
			})

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

			// selfHostedShootControllerRegistration covers the extension type used by the self-hosted Shoot itself.
			// The shoot reconciler will create ControllerInstallations for these (without SeedRefName label).
			// Using Extension resources (with WorkerlessSupported) avoids the need for a full worker pool spec.
			selfHostedShootControllerRegistration := &gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "ctrlreg-self-hosted-shoot-",
					Labels:       map[string]string{testID: testRunID},
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: extensionsv1alpha1.ExtensionResource, Type: selfHostedProviderType, WorkerlessSupported: ptr.To(true)},
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
			selfHostedShoot := &gardencorev1beta1.Shoot{
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
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.31.1",
					},
					// Use an explicit Extension instead of workers to trigger the shoot reconciler without
					// needing a full worker pool spec (which requires machine image, networking type, etc.).
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
			})

			By("Create self-hosted Seed")
			selfHostedSeed := seed.DeepCopy()
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

			By("Expect seed-role ControllerInstallations to be created with .spec.shootRef and SeedRefName label (seed reconciler)")
			Eventually(func(g Gomega) {
				controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
				g.Expect(testClient.List(ctx, controllerInstallationList, client.MatchingFields{
					core.RegistrationRefName: seedControllerRegistration.Name,
					core.ShootRefName:        selfHostedSeedName,
					core.ShootRefNamespace:   v1beta1constants.GardenNamespace,
				})).To(Succeed())
				g.Expect(controllerInstallationList.Items).To(HaveLen(1))
				for _, item := range controllerInstallationList.Items {
					g.Expect(item.Spec.RegistrationRef.Name).To(Equal(seedControllerRegistration.Name), "seed-role ControllerInstallation %q must reference seedControllerRegistration", item.Name)
					g.Expect(item.Spec.ShootRef).To(PointTo(MatchFields(IgnoreExtras, Fields{
						"Name":      Equal(selfHostedSeedName),
						"Namespace": Equal(v1beta1constants.GardenNamespace),
					})), "seed-role ControllerInstallation %q must reference the self-hosted shoot", item.Name)
					g.Expect(item.Spec.SeedRef).To(BeNil(), "seed-role ControllerInstallation %q must not have .spec.seedRef", item.Name)
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
