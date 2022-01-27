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

package extensionlabels_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	internalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/global/extensionlabels"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"
)

var _ = Describe("ExtensionLabels tests", func() {
	var (
		admissionHandler                  *ExtensionLabels
		gardenInternalCoreInformerFactory internalcoreinformers.SharedInformerFactory
	)

	BeforeEach(func() {
		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })

		gardenInternalCoreInformerFactory = internalcoreinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetInternalCoreInformerFactory(gardenInternalCoreInformerFactory)
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
					Backup: &core.SeedBackup{
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

		It("should add all the correct labels on updation", func() {
			newSeed := seed.DeepCopy()
			newSeed.Spec.Backup = &core.SeedBackup{
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
			extensionType1   = "extension-type-1"
			extensionType2   = "extension-type-2"
			extensionType3   = "extension-type-3"
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: "test-namespace",
				},
				Spec: core.ShootSpec{
					Networking: core.Networking{Type: networkingType},
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
							Type: extensionType1,
						},
						{
							Type:     extensionType2,
							Disabled: pointer.Bool(true),
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
			expectedLabels["extensions.extensions.gardener.cloud/"+extensionType1] = "true"
			for _, crType := range []string{crType1, crType2} {
				expectedLabels["containerruntime.extensions.gardener.cloud/"+crType] = "true"
			}
			for _, dnsProviderType := range []string{dnsProviderType1, dnsProviderType2} {
				expectedLabels["dnsrecord.extensions.gardener.cloud/"+dnsProviderType] = "true"
			}

			Expect(shoot.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on updation", func() {
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
					Disabled: pointer.Bool(false),
				},
				{
					Type: extensionType3,
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
			for _, extensionType := range []string{extensionType2, extensionType3} {
				expectedLabels["extensions.extensions.gardener.cloud/"+extensionType] = "true"
			}

			Expect(newShoot.ObjectMeta.Labels).To(Equal(expectedLabels))
		})
	})

	Context("CloudProfile", func() {
		var (
			cloudProfile  *core.CloudProfile
			providerType  = "provider-type"
			machineImage1 = "machine-image-1"
			machineImage2 = "machine-image-2"
			crType1       = "containerRuntime-types-1"
			crType2       = "containerRuntime-types-2"
		)

		BeforeEach(func() {
			cloudProfile = &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cloudprofile",
				},
				Spec: core.CloudProfileSpec{
					Type: providerType,
					MachineImages: []core.MachineImage{
						{
							Name: machineImage1,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{Version: "version-1"},
									CRI: []core.CRI{
										{
											ContainerRuntimes: []core.ContainerRuntime{{Type: crType1}},
										},
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{Version: "version-2"},
									CRI: []core.CRI{
										{
											ContainerRuntimes: []core.ContainerRuntime{{Type: crType1}},
										},
									},
								},
							},
						},
					},
				},
			}
		})

		It("should add all the correct labels on creation", func() {
			attrs := admission.NewAttributesRecord(cloudProfile, nil, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"operatingsystemconfig.extensions.gardener.cloud/" + machineImage1: "true",
				"provider.extensions.gardener.cloud/" + providerType:               "true",
				"containerruntime.extensions.gardener.cloud/" + crType1:            "true",
			}

			Expect(cloudProfile.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on updation", func() {
			machineImage := core.MachineImage{
				Name: machineImage2,
				Versions: []core.MachineImageVersion{
					{
						CRI: []core.CRI{
							{
								ContainerRuntimes: []core.ContainerRuntime{
									{
										Type: crType2,
									},
								},
							},
						},
					},
				},
			}

			newCloudProfile := cloudProfile.DeepCopy()
			newCloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, machineImage)

			attrs := admission.NewAttributesRecord(newCloudProfile, cloudProfile, core.Kind("CloudProfile").WithVersion("version"), "", cloudProfile.Name, core.Resource("CloudProfile").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := make(map[string]string)

			expectedLabels["provider.extensions.gardener.cloud/"+providerType] = "true"
			for _, machineImage := range []string{machineImage1, machineImage2} {
				expectedLabels["operatingsystemconfig.extensions.gardener.cloud/"+machineImage] = "true"
			}
			for _, crType := range []string{crType1, crType2} {
				expectedLabels["containerruntime.extensions.gardener.cloud/"+crType] = "true"
			}

			Expect(newCloudProfile.ObjectMeta.Labels).To(Equal(expectedLabels))
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
			backupBucket  *core.BackupBucket
			backupEntry   *core.BackupEntry
			providerType1 = "provider-type-1"
			providerType2 = "provider-type-2"
		)

		BeforeEach(func() {
			backupBucket = &core.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backupbucket",
				},
				Spec: core.BackupBucketSpec{
					Provider: core.BackupBucketProvider{
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
			Expect(gardenInternalCoreInformerFactory.Core().InternalVersion().BackupBuckets().Informer().GetStore().Add(backupBucket)).To(Succeed())

			attrs := admission.NewAttributesRecord(backupEntry, nil, core.Kind("BackupEntry").WithVersion("version"), backupEntry.Namespace, backupEntry.Name, core.Resource("BackupEntry").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"provider.extensions.gardener.cloud/" + providerType1: "true",
			}

			Expect(backupEntry.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should add all the correct labels on updation", func() {
			backupBucket2 := &core.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backupbucket-2",
				},
				Spec: core.BackupBucketSpec{
					Provider: core.BackupBucketProvider{
						Type: providerType2,
					},
				}}
			Expect(gardenInternalCoreInformerFactory.Core().InternalVersion().BackupBuckets().Informer().GetStore().Add(backupBucket2)).To(Succeed())

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
			Expect(registered).To(ContainElement(PluginName))
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
			el.SetInternalCoreInformerFactory(internalcoreinformers.NewSharedInformerFactory(nil, 0))
			err := el.ValidateInitialization()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
