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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	"github.com/gardener/gardener/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
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

			unmanagedDNSProvider = garden.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName         = "some-machineimage"
			validMachineImageVersions     = []garden.MachineImageVersion{{Version: "0.0.1"}}
			validShootMachineImageVersion = "0.0.1"

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
						Provider: &unmanagedDNSProvider,
						Domain:   test.MakeStrPointer(fmt.Sprintf("shoot.%s", baseDomain)),
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
			cloudProfile.Spec.Packet = nil
			cloudProfile.Spec.OpenStack = nil

			shoot.Spec.Cloud.AWS = nil
			shoot.Spec.Cloud.Azure = nil
			shoot.Spec.Cloud.GCP = nil
			shoot.Spec.Cloud.Packet = nil
			shoot.Spec.Cloud.OpenStack = nil
			shoot.Spec.Kubernetes = garden.Kubernetes{
				KubeControllerManager: nil,
			}
		})

		// The verification of protection is independent of the Cloud Provider (being checked before). We use AWS.
		Context("VALIDATION: Shoot references a Seed already -  validate user provided seed regarding protection", func() {
			var (
				falseVar   = false
				oldShoot   *garden.Shoot
				awsProfile = &garden.AWSProfile{
					Constraints: garden.AWSConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
							},
						},
						MachineTypes: []garden.MachineType{
							{
								Name:   "machine-type-1",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
							},
							{
								Name:   "machine-type-old",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
								Usable: &falseVar,
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				awsCloud = &garden.AWSCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				awsCloud.Networks = garden.AWSNetworks{K8SNetworks: k8sNetworks}
				awsCloud.Workers = workers
				awsCloud.Zones = zones
				awsCloud.MachineImage = machineImage
				cloudProfile.Spec.AWS = awsProfile
				shoot.Spec.Cloud.AWS = awsCloud

				// set seed name
				shoot.Spec.Cloud.Seed = &seedName

				// set old shoot for update
				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.Cloud.Seed = nil
			})

			It("create should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("create should pass because shoot is not in garden namespace and seed is not protected", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because shoot is not in garden namespace and seed is not protected", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("create should fail because shoot is not in garden namespace and seed is protected", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("update should fail because shoot is not in garden namespace and seed is protected", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("create should pass because shoot is in garden namespace and seed is protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because shoot is in garden namespace and seed is protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("create should pass because shoot is in garden namespace and seed is not protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because shoot is in garden namespace and seed is not protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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
				err := admissionHandler.Admit(attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))

				attrs = admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)
				err = admissionHandler.Admit(attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))
			})
		})

		It("should reject because the referenced cloud profile was not found", func() {
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced seed was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced project was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs, nil)

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

			err := admissionHandler.Admit(attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		Context("tests for AWS cloud", func() {
			var (
				falseVar   = false
				awsProfile = &garden.AWSProfile{
					Constraints: garden.AWSConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
							},
						},
						MachineTypes: []garden.MachineType{
							{
								Name:   "machine-type-1",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
							},
							{
								Name:   "machine-type-old",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("100Gi"),
								Usable: &falseVar,
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				awsCloud = &garden.AWSCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				awsCloud.Networks = garden.AWSNetworks{K8SNetworks: k8sNetworks}
				awsCloud.Workers = workers
				awsCloud.Zones = zones
				awsCloud.MachineImage = machineImage
				cloudProfile.Spec.AWS = awsProfile
				shoot.Spec.Cloud.AWS = awsCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the specified domain is a subdomain of a domain already used by another shoot", func() {
				anotherShoot := shoot.DeepCopy()
				anotherShoot.Name = "another-shoot"

				subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
				shoot.Spec.DNS.Domain = &subdomain

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case one)", func() {
				anotherShoot := shoot.DeepCopy()
				anotherShoot.Name = "another-shoot"

				subdomain := fmt.Sprintf("subdomain.%s", *anotherShoot.Spec.DNS.Domain)
				shoot.Spec.DNS.Domain = &subdomain

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the specified domain is a subdomain of a domain already used by another shoot (case two)", func() {
				anotherShoot := shoot.DeepCopy()
				anotherShoot.Name = "another-shoot"

				shoot.Spec.DNS.Domain = &baseDomain

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow because the specified domain is not a subdomain of a domain already used by another shoot", func() {
				anotherShoot := shoot.DeepCopy()
				anotherShoot.Name = "another-shoot"

				anotherDomain := fmt.Sprintf("someprefix%s", *anotherShoot.Spec.DNS.Domain)
				shoot.Spec.DNS.Domain = &anotherDomain

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(anotherShoot)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(BeNil())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.AWS.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.AWS.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.AWS.Constraints.MachineImages = append(cloudProfile.Spec.AWS.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should not reject due to an usable machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-1",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject due to a not usable machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker: garden.Worker{
							MachineType: "machine-type-old",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
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
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				azureCloud = &garden.AzureCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				cloudProfile.Spec.Azure = azureProfile
				azureCloud.Networks = garden.AzureNetworks{K8SNetworks: k8sNetworks}
				azureCloud.Workers = workers
				azureCloud.MachineImage = machineImage
				shoot.Spec.Cloud.Azure = azureCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It(" ", func() {
				shoot.Spec.Cloud.Azure.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.Azure.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.Azure.Constraints.MachineImages = append(cloudProfile.Spec.Azure.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no fault domain count has been specified", func() {
				shoot.Spec.Cloud.Region = "asia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no update domain count has been specified", func() {
				shoot.Spec.Cloud.Region = "australia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				gcpCloud = &garden.GCPCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				gcpCloud.Networks = garden.GCPNetworks{K8SNetworks: k8sNetworks}
				gcpCloud.Workers = workers
				gcpCloud.Zones = zones
				gcpCloud.MachineImage = machineImage
				cloudProfile.Spec.GCP = gcpProfile
				shoot.Spec.Cloud.GCP = gcpCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.GCP.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.GCP.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.GCP.Constraints.MachineImages = append(cloudProfile.Spec.GCP.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for Packet cloud", func() {
			var (
				packetProfile = &garden.PacketProfile{
					Constraints: garden.PacketConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
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
				workers = []garden.PacketWorker{
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				packetCloud = &garden.PacketCloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				packetCloud.Networks = garden.PacketNetworks{K8SNetworks: k8sNetworks}
				packetCloud.Workers = workers
				packetCloud.Zones = zones
				packetCloud.MachineImage = machineImage
				cloudProfile.Spec.Packet = packetProfile
				shoot.Spec.Cloud.Packet = packetCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Packet.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Packet.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.Packet.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.Packet.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.Packet.Constraints.MachineImages = append(cloudProfile.Spec.Packet.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.Packet.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						LoadBalancerProviders: []garden.OpenStackLoadBalancerProvider{
							{
								Name: "haproxy",
							},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
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
				openStackCloud.MachineImage = machineImage
				cloudProfile.Spec.OpenStack = openStackProfile
				shoot.Spec.Cloud.OpenStack = openStackCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should not reject due to an undefined dns domain", func() {
				shoot.Spec.DNS.Domain = nil

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Succeed())
			})

			It("should reject due to an invalid floating pool name", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = "invalid-pool"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid load balancer provider", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = "invalid-provider"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.OpenStack.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.OpenStack.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.OpenStack.Constraints.MachineImages = append(cloudProfile.Spec.OpenStack.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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
							OfferedVersions: []garden.KubernetesVersion{{Version: "1.6.4"}},
						},
						MachineImages: []garden.MachineImage{
							{
								Name:     validMachineImageName,
								Versions: validMachineImageVersions,
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
				zones        = []string{"europe-a"}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				aliCloud = &garden.Alicloud{}
			)

			BeforeEach(func() {
				cloudProfile = cloudProfileBase
				shoot = shootBase
				aliCloud.Networks = garden.AlicloudNetworks{K8SNetworks: k8sNetworks}
				aliCloud.Workers = workers
				aliCloud.Zones = zones
				aliCloud.MachineImage = machineImage
				cloudProfile.Spec.Alicloud = alicloudProfile
				shoot.Spec.Cloud.Alicloud = aliCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.Cloud.Seed = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid dns provider", func() {
				provider := "some-provider"
				shoot.Spec.DNS.Provider = &provider

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.6.6"}
				cloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions, highestPatchVersion, garden.KubernetesVersion{Version: "1.7.1"}, garden.KubernetesVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.KubernetesVersion{Version: "1.81.5"}
				cloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = append(cloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Cloud.Alicloud.MachineImage = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Cloud.Alicloud.MachineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.Alicloud.Constraints.MachineImages = append(cloudProfile.Spec.Alicloud.Constraints.MachineImages, garden.MachineImage{
					Name: validMachineImageName,
					Versions: []garden.MachineImageVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.MachineImage{
					Name: "other-image-name",
					Versions: []garden.MachineImageVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.Alicloud.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

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

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})
	})
})
