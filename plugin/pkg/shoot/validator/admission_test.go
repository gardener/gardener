// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator_test

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *ValidateShoot
			gardenInformerFactory gardeninformers.SharedInformerFactory
			cloudProfile          garden.CloudProfile
			seed                  garden.Seed
			project               garden.Project
			shoot                 garden.Shoot

			podCIDR     = garden.CIDR("100.96.0.0/11")
			serviceCIDR = garden.CIDR("100.64.0.0/13")
			nodesCIDR   = garden.CIDR("10.250.0.0/16")
			k8sNetworks = garden.K8SNetworks{
				Pods:     &podCIDR,
				Services: &serviceCIDR,
				Nodes:    &nodesCIDR,
			}

			seedName      = "seed"
			namespaceName = "garden-my-project"
			projectName   = "my-project"

			seedPodsCIDR     = garden.CIDR("10.241.128.0/17")
			seedServicesCIDR = garden.CIDR("10.241.0.0/17")
			seedNodesCIDR    = garden.CIDR("10.240.0.0/16")

			projectBase = garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: garden.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
			cloudProfileBase = garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: garden.CloudProfileSpec{},
			}
			seedBase = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Networks: garden.SeedNetworks{
						Pods:     seedPodsCIDR,
						Services: seedServicesCIDR,
						Nodes:    seedNodesCIDR,
					},
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: "profile",
						Region:  "europe",
						Seed:    &seedName,
						SecretBindingRef: corev1.LocalObjectReference{
							Name: "my-secret",
						},
					},
					DNS: garden.DNS{
						Provider: garden.DNSUnmanaged,
						Domain:   makeStrPointer("shoot.example.com"),
					},
					Kubernetes: garden.Kubernetes{
						Version: "1.6.4",
					},
				},
			}
		)

		BeforeEach(func() {
			project = projectBase
			cloudProfile = cloudProfileBase
			seed = seedBase
			shoot = shootBase

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)
		})

		AfterEach(func() {
			cloudProfile.Spec.AWS = nil
			cloudProfile.Spec.Azure = nil
			cloudProfile.Spec.GCP = nil
			cloudProfile.Spec.OpenStack = nil

			shoot.Spec.Cloud.AWS = nil
			shoot.Spec.Cloud.Azure = nil
			shoot.Spec.Cloud.GCP = nil
			shoot.Spec.Cloud.OpenStack = nil
		})

		Context("name/project length checks", func() {
			It("should reject Shoot resources with two consecutive hyphens in project name", func() {
				twoConsecutiveHyphensName := "n--o"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: twoConsecutiveHyphensName,
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsBadRequest(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("consecutive hyphens"))
			})

			It("should reject create operations on Shoot resources in projects which shall be deleted", func() {
				deletionTimestamp := metav1.NewTime(time.Now())
				project.ObjectMeta.DeletionTimestamp = &deletionTimestamp

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("already marked for deletion"))
			})

			It("should reject Shoot resources with not fulfilling the length constraints", func() {
				tooLongName := "too-long-namespace"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: tooLongName,
				}
				shoot.ObjectMeta = metav1.ObjectMeta{
					Name:      "too-long-name",
					Namespace: namespaceName,
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsBadRequest(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("name must not exceed"))
			})

			It("should not testing length constraints for operations other than CREATE", func() {
				shortName := "short"
				projectName := "too-long-long-long-label"
				project.ObjectMeta = metav1.ObjectMeta{
					Name: projectName,
				}
				shoot.ObjectMeta = metav1.ObjectMeta{
					Name:      shortName,
					Namespace: shortName,
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)
				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))

				attrs = admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)
				err = admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))
			})
		})

		It("should reject because the referenced cloud profile was not found", func() {
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced seed was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced project was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the cloud provider in shoot and profile differ", func() {
			cloudProfile.Spec.GCP = &garden.GCPProfile{}
			shoot.Spec.Cloud.AWS = &garden.AWSCloud{}

			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		Context("tests for AWS cloud", func() {
			var (
				awsProfile = &garden.AWSProfile{
					Constraints: garden.AWSConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{"1.6.4"},
						},
						MachineImages: []garden.AWSMachineImageMapping{
							{
								Name: garden.MachineImageCoreOS,
								Regions: []garden.AWSRegionalMachineImage{
									{
										Name: "europe",
										AMI:  "ami-12345678",
									},
								},
							},
						},
						MachineTypes: []garden.MachineType{
							{
								Name:   "machine-type-1",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
							},
						},
						VolumeTypes: []garden.VolumeType{
							{
								Name:  "volume-type-1",
								Class: "super-premium",
							},
						},
						Zones: []garden.Zone{
							{
								Region: "europe",
								Names:  []string{"europe-a"},
							},
							{
								Region: "asia",
								Names:  []string{"asia-a"},
							},
						},
					},
				}
				workers = []garden.AWSWorker{
					{
						Worker: garden.Worker{
							Name:          "worker-name",
							MachineType:   "machine-type-1",
							AutoScalerMin: 1,
							AutoScalerMax: 1,
						},
						VolumeSize: "10Gi",
						VolumeType: "volume-type-1",
					},
				}
				zones    = []string{"europe-a"}
				awsCloud = &garden.AWSCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				awsCloud.Networks = garden.AWSNetworks{K8SNetworks: k8sNetworks}
				awsCloud.Workers = workers
				awsCloud.Zones = zones
				cloudProfile.Spec.AWS = awsProfile
				shoot.Spec.Cloud.AWS = awsCloud
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the specified domain is already used by another shoot", func() {
				anotherShoot := shoot.DeepCopy()
				anotherShoot.Name = "another-shoot"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.AWS.MachineImage = &garden.AWSMachineImage{
					Name: garden.MachineImageName("not-supported"),
					AMI:  "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker: garden.Worker{
							MachineType: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
						VolumeType: "not-allowed",
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no machine image has been specified", func() {
				shoot.Spec.Cloud.Region = "asia"
				shoot.Spec.Cloud.AWS.Zones = []string{"asia-a"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for Azure cloud", func() {
			var (
				azureProfile = &garden.AzureProfile{
					Constraints: garden.AzureConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{"1.6.4"},
						},
						MachineImages: []garden.AzureMachineImage{
							{
								Name:      garden.MachineImageCoreOS,
								Publisher: "CoreOS",
								Offer:     "CoreOS",
								SKU:       "Beta",
								Version:   "1.2.3",
							},
						},
						MachineTypes: []garden.MachineType{
							{
								Name:   "machine-type-1",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
							},
						},
						VolumeTypes: []garden.VolumeType{
							{
								Name:  "volume-type-1",
								Class: "super-premium",
							},
						},
					},
					CountFaultDomains: []garden.AzureDomainCount{
						{
							Region: "europe",
							Count:  1,
						},
						{
							Region: "australia",
							Count:  1,
						},
					},
					CountUpdateDomains: []garden.AzureDomainCount{
						{
							Region: "europe",
							Count:  1,
						},
						{
							Region: "asia",
							Count:  1,
						},
					},
				}
				workers = []garden.AzureWorker{
					{
						Worker: garden.Worker{
							Name:          "worker-name",
							MachineType:   "machine-type-1",
							AutoScalerMin: 1,
							AutoScalerMax: 1,
						},
						VolumeSize: "10Gi",
						VolumeType: "volume-type-1",
					},
				}
				azureCloud = &garden.AzureCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				cloudProfile.Spec.Azure = azureProfile
				azureCloud.Networks = garden.AzureNetworks{K8SNetworks: k8sNetworks}
				azureCloud.Workers = workers
				shoot.Spec.Cloud.Azure = azureCloud
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.Azure.MachineImage = &garden.AzureMachineImage{
					Name:      garden.MachineImageName("not-supported"),
					Publisher: "not-supported",
					Offer:     "not-supported",
					SKU:       "not-supported",
					Version:   "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker: garden.Worker{
							MachineType: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
						VolumeType: "not-allowed",
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no fault domain count has been specified", func() {
				shoot.Spec.Cloud.Region = "asia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no update domain count has been specified", func() {
				shoot.Spec.Cloud.Region = "australia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for GCP cloud", func() {
			var (
				gcpProfile = &garden.GCPProfile{
					Constraints: garden.GCPConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{"1.6.4"},
						},
						MachineImages: []garden.GCPMachineImage{
							{
								Name:  garden.MachineImageCoreOS,
								Image: "core-1.2.3",
							},
						},
						MachineTypes: []garden.MachineType{
							{
								Name:   "machine-type-1",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
							},
						},
						VolumeTypes: []garden.VolumeType{
							{
								Name:  "volume-type-1",
								Class: "super-premium",
							},
						},
						Zones: []garden.Zone{
							{
								Region: "europe",
								Names:  []string{"europe-a"},
							},
							{
								Region: "asia",
								Names:  []string{"asia-a"},
							},
						},
					},
				}
				workers = []garden.GCPWorker{
					{
						Worker: garden.Worker{
							Name:          "worker-name",
							MachineType:   "machine-type-1",
							AutoScalerMin: 1,
							AutoScalerMax: 1,
						},
						VolumeSize: "10Gi",
						VolumeType: "volume-type-1",
					},
				}
				zones    = []string{"europe-a"}
				gcpCloud = &garden.GCPCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				gcpCloud.Networks = garden.GCPNetworks{K8SNetworks: k8sNetworks}
				gcpCloud.Workers = workers
				gcpCloud.Zones = zones
				cloudProfile.Spec.GCP = gcpProfile
				shoot.Spec.Cloud.GCP = gcpCloud
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.GCP.MachineImage = &garden.GCPMachineImage{
					Name:  garden.MachineImageName("not-supported"),
					Image: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker: garden.Worker{
							MachineType: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
						VolumeType: "not-allowed",
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for OpenStack cloud", func() {
			var (
				openStackProfile = &garden.OpenStackProfile{
					Constraints: garden.OpenStackConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						FloatingPools: []garden.OpenStackFloatingPool{
							{
								Name: "pool",
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{"1.6.4"},
						},
						LoadBalancerProviders: []garden.OpenStackLoadBalancerProvider{
							{
								Name: "haproxy",
							},
						},
						MachineImages: []garden.OpenStackMachineImage{
							{
								Name:  garden.MachineImageCoreOS,
								Image: "core-1.2.3",
							},
						},
						MachineTypes: []garden.OpenStackMachineType{
							{
								MachineType: garden.MachineType{
									Name:   "machine-type-1",
									CPU:    resource.MustParse("2"),
									GPU:    resource.MustParse("0"),
									Memory: resource.MustParse("100Gi"),
								},
								VolumeType: "default",
								VolumeSize: resource.MustParse("20Gi"),
							},
						},
						Zones: []garden.Zone{
							{
								Region: "europe",
								Names:  []string{"europe-a"},
							},
							{
								Region: "asia",
								Names:  []string{"asia-a"},
							},
						},
					},
				}
				workers = []garden.OpenStackWorker{
					{
						Worker: garden.Worker{
							Name:          "worker-name",
							MachineType:   "machine-type-1",
							AutoScalerMin: 1,
							AutoScalerMax: 1,
						},
					},
				}
				zones          = []string{"europe-a"}
				openStackCloud = &garden.OpenStackCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				openStackCloud.FloatingPoolName = "pool"
				openStackCloud.LoadBalancerProvider = "haproxy"
				openStackCloud.Networks = garden.OpenStackNetworks{K8SNetworks: k8sNetworks}
				openStackCloud.Workers = workers
				openStackCloud.Zones = zones
				cloudProfile.Spec.OpenStack = openStackProfile
				shoot.Spec.Cloud.OpenStack = openStackCloud
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an undefined dns domain", func() {
				shoot.Spec.DNS.Domain = nil

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid floating pool name", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = "invalid-pool"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid load balancer provider", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = "invalid-provider"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.OpenStack.MachineImage = &garden.OpenStackMachineImage{
					Name:  garden.MachineImageName("not-supported"),
					Image: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: garden.Worker{
							MachineType: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for Ali cloud", func() {
			var (
				alicloudProfile = &garden.AlicloudProfile{
					Constraints: garden.AlicloudConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{"1.6.4"},
						},
						MachineImages: []garden.AlicloudMachineImage{
							{
								Name: garden.MachineImageCoreOS,
								ID:   "coreos_1745_7_0_64_30G_alibase_20180705.vhd",
							},
						},
						MachineTypes: []garden.AlicloudMachineType{
							{
								MachineType: garden.MachineType{
									Name:   "machine-type-1",
									CPU:    resource.MustParse("2"),
									GPU:    resource.MustParse("0"),
									Memory: resource.MustParse("100Gi"),
								},
								Zones: []string{
									"europe-a",
								},
							},
						},
						VolumeTypes: []garden.AlicloudVolumeType{
							{
								VolumeType: garden.VolumeType{
									Name:  "volume-type-1",
									Class: "standard",
								},
								Zones: []string{
									"europe-a",
								},
							},
						},
						Zones: []garden.Zone{
							{
								Region: "europe",
								Names:  []string{"europe-a"},
							},
							{
								Region: "asia",
								Names:  []string{"asia-a"},
							},
						},
					},
				}
				workers = []garden.AlicloudWorker{
					{
						Worker: garden.Worker{
							Name:          "worker-name",
							MachineType:   "machine-type-1",
							AutoScalerMin: 1,
							AutoScalerMax: 1,
						},
						VolumeSize: "10Gi",
						VolumeType: "volume-type-1",
					},
				}
				zones    = []string{"europe-a"}
				aliCloud = &garden.Alicloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				aliCloud.Networks = garden.AlicloudNetworks{K8SNetworks: k8sNetworks}
				aliCloud.Workers = workers
				aliCloud.Zones = zones
				cloudProfile.Spec.Alicloud = alicloudProfile
				shoot.Spec.Cloud.Alicloud = aliCloud
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.Alicloud.MachineImage = &garden.AlicloudMachineImage{
					Name: garden.MachineImageName("not-supported"),
					ID:   "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker: garden.Worker{
							MachineType: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
						VolumeType: "not-allowed",
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.Alicloud.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an machine type is not available in shoot zones", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
					},
				}

				shoot.Spec.Cloud.Alicloud.Zones = []string{"cn-beijing-a"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an volume type is not available in shoot zones", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						VolumeType: "volume-type-1",
					},
				}

				shoot.Spec.Cloud.Alicloud.Zones = []string{"cn-beijing-a"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})
	})
})

func makeStrPointer(in string) *string {
	out := in
	return &out
}
