// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionlabels_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/security"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/global/extensionlabels"
)

var _ = Describe("ExtensionLabels tests", func() {
	var (
		admissionHandler          *ExtensionLabels
		gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory
	)

	BeforeEach(func() {
		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })

		gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)
	})

	Context("Seed", func() {
		var (
			seed *core.Seed

			providerType1   = "provider-type-1"
			providerType2   = "provider-type-2"
			dnsProviderType = "dns-provider"
		)

		BeforeEach(func() {
			seed = &core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-seed",
				},
				Spec: core.SeedSpec{
					Provider: core.SeedProvider{
						Type: providerType1,
					},
					Ingress: &core.Ingress{},
					DNS: core.SeedDNS{
						Provider: &core.SeedDNSProvider{
							Type: dnsProviderType,
						},
					},
					Backup: &core.Backup{
						Provider: providerType1,
					},
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("Seed").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1:    "true",
				"dnsrecord.extensions.gardener.cloud/" + dnsProviderType: "true",
			}

			Expect(seed.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on update", func() {
			newSeed := seed.DeepCopy()
			newSeed.Spec.Backup = &core.Backup{
				Provider: providerType2,
			}

			attrs := admission.NewAttributesRecord(newSeed, seed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("Seed").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := make(map[string]string)
			expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
			for _, providerType := range []string{providerType1, providerType2} {
				expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
			}

			Expect(newSeed.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("SecretBinding", func() {
		const (
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
			providerType3 = "provider-type-3"
		)

		DescribeTable("should add all the correct labels on creation",
			func(secretBinding *core.SecretBinding, expectedLabels map[string]string) {
				attrs := admission.NewAttributesRecord(secretBinding, nil, core.Kind("SecretBinding").WithVersion("version"), "", secretBinding.Name, core.Resource("SecretBinding").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(secretBinding.ObjectMeta.Labels).To(Equal(expectedLabels))
			},
			Entry("when provider is nil", &core.SecretBinding{ObjectMeta: metav1.ObjectMeta{Name: "test-secretbinding"}, Provider: nil}, nil),
			Entry("when provider is set", &core.SecretBinding{ObjectMeta: metav1.ObjectMeta{Name: "test-secretbinding"}, Provider: &core.SecretBindingProvider{Type: providerType1}}, map[string]string{"provider.extensions.gardener.cloud/" + providerType1: "true"}),
		)

		It("should add all the correct labels on update", func() {
			secretBinding := &core.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-secretbinding",
				},
			}
			newSecretBinding := secretBinding.DeepCopy()
			newSecretBinding.Provider = &core.SecretBindingProvider{
				Type: fmt.Sprintf("%s,%s", providerType2, providerType3),
			}

			attrs := admission.NewAttributesRecord(newSecretBinding, secretBinding, core.Kind("SecretBinding").WithVersion("version"), "", secretBinding.Name, core.Resource("SecretBinding").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType2: "true",
				"provider.extensions.gardener.cloud/" + providerType3: "true",
			}

			Expect(newSecretBinding.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("CredentialsBinding", func() {
		const (
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
		)

		It("should add the correct label on creation", func() {
			credentialsBinding := &security.CredentialsBinding{ObjectMeta: metav1.ObjectMeta{Name: "test-cb"}, Provider: security.CredentialsBindingProvider{Type: providerType1}}
			attrs := admission.NewAttributesRecord(credentialsBinding, nil, security.Kind("CredentialsBinding").WithVersion("version"), "", credentialsBinding.Name, security.Resource("CredentialsBinding").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(credentialsBinding.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1: "true",
			}))
		})

		It("should add the correct label on update", func() {
			credentialsBinding := &security.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cb",
				},
			}
			newCredentialsBinding := credentialsBinding.DeepCopy()
			newCredentialsBinding.Provider = security.CredentialsBindingProvider{Type: providerType2}

			attrs := admission.NewAttributesRecord(newCredentialsBinding, credentialsBinding, security.Kind("CredentialsBinding").WithVersion("version"), "", credentialsBinding.Name, security.Resource("CredentialsBinding").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(newCredentialsBinding.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.extensions.gardener.cloud/" + providerType2: "true",
			}))
		})
	})

	Context("WorkloadIdentity", func() {
		const (
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
		)

		It("should add the correct label on creation", func() {
			workloadIdentity := &security.WorkloadIdentity{ObjectMeta: metav1.ObjectMeta{Name: "test-wi"}, Spec: security.WorkloadIdentitySpec{
				TargetSystem: security.TargetSystem{
					Type: providerType1,
				},
			}}
			attrs := admission.NewAttributesRecord(workloadIdentity, nil, security.Kind("WorkloadIdentity").WithVersion("version"), "", workloadIdentity.Name, security.Resource("WorkloadIdentity").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(workloadIdentity.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1: "true",
			}))
		})

		It("should add the correct label on update", func() {
			workloadIdentity := &security.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-wi",
				},
			}
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.Type = providerType2

			attrs := admission.NewAttributesRecord(newWorkloadIdentity, workloadIdentity, security.Kind("WorkloadIdentity").WithVersion("version"), "", workloadIdentity.Name, security.Resource("WorkloadIdentity").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(newWorkloadIdentity.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.extensions.gardener.cloud/" + providerType2: "true",
			}))
		})
	})

	Context("Shoot", func() {
		var (
			shoot *core.Shoot

			providerType     = "provider-type"
			networkingType   = "networking-type"
			machineImage1    = "machine-image-1"
			machineImage2    = "machine-image-2"
			crType1          = "containerRuntime-type-1"
			crType2          = "containerRuntime-type-2"
			crType3          = "containerRuntime-type-3"
			crType4          = "containerRuntime-type-4"
			dnsProviderType1 = "dns-external-1"
			dnsProviderType2 = "dns-external-2"
			extensionType1   = "extension-type-1" // globally enabled
			extensionType2   = "extension-type-2" // globally enabled + disabled for shoot
			extensionType3   = "extension-type-3" // enabled for shoot
			extensionType4   = "extension-type-4" // not enabled
			extensionType5   = "extension-type-5" // globally enabled +  workerlessSupported
		)

		BeforeEach(func() {
			controllerRegistrations := []*gardencorev1beta1.ControllerRegistration{{
				ObjectMeta: metav1.ObjectMeta{Name: "registration1"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:            extensionsv1alpha1.ExtensionResource,
							Type:            extensionType1,
							GloballyEnabled: ptr.To(true),
						},
						{
							Kind:            extensionsv1alpha1.ExtensionResource,
							Type:            extensionType2,
							GloballyEnabled: ptr.To(true),
						},
						{
							Kind:                extensionsv1alpha1.ExtensionResource,
							Type:                extensionType5,
							GloballyEnabled:     ptr.To(true),
							WorkerlessSupported: ptr.To(true),
						},
					},
					Deployment: nil,
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{Name: "registration2"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{{
						Kind: extensionsv1alpha1.ExtensionResource,
						Type: extensionType3,
					}, {
						Kind: extensionsv1alpha1.ExtensionResource,
						Type: extensionType4,
					}},
					Deployment: nil,
				},
			}}

			for _, reg := range controllerRegistrations {
				Expect(gardenCoreInformerFactory.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(reg)).To(Succeed())
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: "test-namespace",
				},
				Spec: core.ShootSpec{
					Networking: &core.Networking{Type: ptr.To(networkingType)},
					DNS: &core.DNS{
						Providers: []core.DNSProvider{
							{Type: &dnsProviderType1},
							{Type: &dnsProviderType2},
						},
					},
					Provider: core.Provider{
						Type: providerType,
						Workers: []core.Worker{
							{
								Machine: core.Machine{
									Type: "type-1",
									Image: &core.ShootMachineImage{
										Name: machineImage1,
									},
								},
								CRI: &core.CRI{
									ContainerRuntimes: []core.ContainerRuntime{
										{Type: crType1},
										{Type: crType2},
										{Type: crType2},
									},
								},
							},
							{
								Machine: core.Machine{
									Type: "type-2",
									Image: &core.ShootMachineImage{
										Name: machineImage1,
									},
								},
								CRI: &core.CRI{
									ContainerRuntimes: []core.ContainerRuntime{
										{Type: crType1},
										{Type: crType1},
										{Type: crType2},
									},
								},
							},
						},
					},
					Extensions: []core.Extension{
						{
							Type:     extensionType2,
							Disabled: ptr.To(true),
						},
						{
							Type: extensionType3,
						},
					},
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("Shoot").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := make(map[string]string)

			expectedLabels["networking.extensions.gardener.cloud/"+networkingType] = "true"
			expectedLabels["operatingsystemconfig.extensions.gardener.cloud/"+machineImage1] = "true"
			expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
			for _, extensionType := range []string{extensionType1, extensionType3, extensionType5} {
				expectedLabels["extensions.extensions.gardener.cloud/"+extensionType] = "true"
			}
			for _, crType := range []string{crType1, crType2} {
				expectedLabels["containerruntime.extensions.gardener.cloud/"+crType] = "true"
			}
			for _, dnsProviderType := range []string{dnsProviderType1, dnsProviderType2} {
				expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
			}

			Expect(shoot.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on update", func() {
			worker := core.Worker{
				Machine: core.Machine{
					Image: &core.ShootMachineImage{
						Name: machineImage2,
					},
				},
				CRI: &core.CRI{
					ContainerRuntimes: []core.ContainerRuntime{
						{
							Type: crType3,
						},
						{
							Type: crType4,
						},
					},
				},
			}
			extension := []core.Extension{
				{
					Type:     extensionType2,
					Disabled: ptr.To(false),
				},
				{
					Type: extensionType4,
				}}

			newShoot := shoot.DeepCopy()
			newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)
			newShoot.Spec.Extensions = extension

			attrs := admission.NewAttributesRecord(newShoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("Shoot").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := make(map[string]string)

			expectedLabels["networking.extensions.gardener.cloud/"+networkingType] = "true"
			expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
			for _, machineImage := range []string{machineImage1, machineImage2} {
				expectedLabels["operatingsystemconfig.extensions.gardener.cloud/"+machineImage] = "true"
			}
			for _, crType := range []string{crType1, crType2, crType3, crType4} {
				expectedLabels["containerruntime.extensions.gardener.cloud/"+crType] = "true"
			}
			for _, dnsProviderType := range []string{dnsProviderType1, dnsProviderType2} {
				expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
			}
			for _, extensionType := range []string{extensionType1, extensionType2, extensionType4, extensionType5} {
				expectedLabels["extensions.extensions.gardener.cloud/"+extensionType] = "true"
			}

			Expect(newShoot.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot = &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-shoot",
						Namespace: "test-namespace",
					},
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{
								{Type: &dnsProviderType1},
							},
						},
						Provider: core.Provider{
							Type: providerType,
						},
						Extensions: []core.Extension{
							{
								Type:     extensionType2,
								Disabled: ptr.To(true),
							},
							{
								Type: extensionType3,
							},
						},
					},
				}
			})

			It("should add all the correct labels on creation", func() {
				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("Shoot").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())

				expectedLabels := make(map[string]string)

				expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
				for _, extensionType := range []string{extensionType3, extensionType5} {
					expectedLabels["extensions.extensions.gardener.cloud/"+extensionType] = "true"
				}
				for _, dnsProviderType := range []string{dnsProviderType1} {
					expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
				}

				Expect(shoot.ObjectMeta.Labels).To(Equal(expectedLabels))
			})

			It("should add all the correct labels on update", func() {
				newShoot := shoot.DeepCopy()
				newShoot.Spec.DNS.Providers = append(newShoot.Spec.DNS.Providers, core.DNSProvider{Type: &dnsProviderType2})
				newShoot.Spec.Extensions = append(newShoot.Spec.Extensions, core.Extension{Type: extensionType4})

				attrs := admission.NewAttributesRecord(newShoot, shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("Shoot").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())

				expectedLabels := make(map[string]string)

				expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
				for _, extensionType := range []string{extensionType3, extensionType4, extensionType5} {
					expectedLabels["extensions.extensions.gardener.cloud/"+extensionType] = "true"
				}
				for _, dnsProviderType := range []string{dnsProviderType1, dnsProviderType2} {
					expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
				}

				Expect(newShoot.ObjectMeta.Labels).To(Equal(expectedLabels))
			})
		})
	})

	Context("CloudProfile", func() {
		var (
			cloudProfile  *core.CloudProfile
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
		)

		BeforeEach(func() {
			cloudProfile = &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cloudprofile",
				},
				Spec: core.CloudProfileSpec{
					Type: providerType1,
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			attrs := admission.NewAttributesRecord(cloudProfile, nil, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1: "true",
			}

			Expect(cloudProfile.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on update", func() {
			newCloudProfile := cloudProfile.DeepCopy()
			newCloudProfile.Spec.Type = providerType2

			attrs := admission.NewAttributesRecord(newCloudProfile, cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType2: "true",
			}

			Expect(newCloudProfile.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("NamespacedCloudProfile", func() {
		var (
			namespacedCloudProfile *core.NamespacedCloudProfile
			parentCloudProfile     *gardencorev1beta1.CloudProfile
			providerType           string
		)

		BeforeEach(func() {
			providerType = "provider-type"
			namespacedCloudProfile = &core.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-namespacedcloudprofile",
					Namespace: "project-namespace",
				},
				Spec: core.NamespacedCloudProfileSpec{
					Parent: core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "test-cloudprofile",
					},
				},
			}
			parentCloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cloudprofile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{Type: providerType},
			}
		})

		It("should fail for an invalid parent kind", func() {
			namespacedCloudProfile.Spec.Parent.Kind = "Invalid-kind"
			attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("NamespacedCloudProfile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.Background(), attrs, nil)

			Expect(err).To(MatchError(ContainSubstring("invalid parent kind")))
		})

		It("should fail if parent CloudProfile is not found", func() {
			attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("NamespacedCloudProfile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.Background(), attrs, nil)

			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("should successfully add all the correct labels on creation", func() {
			Expect(gardenCoreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

			attrs := admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("NamespacedCloudProfile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.Background(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType: "true",
			}

			Expect(namespacedCloudProfile.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("Backup Bucket", func() {
		var (
			backupBucket *core.BackupBucket
			providerType = "provider-type"
		)

		BeforeEach(func() {
			backupBucket = &core.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backupbucket",
				},
				Spec: core.BackupBucketSpec{
					Provider: core.BackupBucketProvider{
						Type: providerType,
					},
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			attrs := admission.NewAttributesRecord(backupBucket, nil, core.Kind("BackupBucket").WithVersion("version"), "", backupBucket.Name, core.Resource("BackupBucket").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType: "true",
			}

			Expect(backupBucket.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("Backup Entry", func() {
		var (
			backupBucket  *gardencorev1beta1.BackupBucket
			backupEntry   *core.BackupEntry
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
		)

		BeforeEach(func() {
			backupBucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backupbucket",
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider: gardencorev1beta1.BackupBucketProvider{
						Type: providerType1,
					},
				},
			}
			backupEntry = &core.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backupentry",
					Namespace: "test-namespace",
				},
				Spec: core.BackupEntrySpec{
					BucketName: backupBucket.Name,
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			Expect(gardenCoreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("BackupEntry").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1: "true",
			}

			Expect(backupEntry.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on update", func() {
			backupBucket2 := &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backupbucket-2",
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					Provider: gardencorev1beta1.BackupBucketProvider{
						Type: providerType2,
					},
				}}
			Expect(gardenCoreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(backupBucket2)).To(Succeed())

			newBackupEntry := backupEntry.DeepCopy()
			newBackupEntry.Spec.BucketName = backupBucket2.Name

			attrs := admission.NewAttributesRecord(newBackupEntry, backupEntry, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("BackupEntry").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType2: "true",
			}

			Expect(newBackupEntry.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ExtensionLabels"))
		})
	})

	Describe("#NewFactory", func() {
		It("should create a new PluginFactory", func() {
			f, err := NewFactory(nil)

			Expect(f).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE + UPDATE operations", func() {
			el, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(el.Handles(admission.Create)).To(BeTrue())
			Expect(el.Handles(admission.Update)).To(BeTrue())
			Expect(el.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(el.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no BackupBucketLister is set", func() {
			el, _ := New()
			err := el.ValidateInitialization()
			Expect(err).To(HaveOccurred())
		})

		It("should not return error if BackupBucketLister and core client are set", func() {
			el, _ := New()
			el.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			err := el.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
