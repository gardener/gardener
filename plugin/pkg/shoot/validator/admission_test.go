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
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	"github.com/gardener/gardener/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"
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

			podCIDR     = "100.96.0.0/11"
			serviceCIDR = "100.64.0.0/13"
			nodesCIDR   = "10.250.0.0/16"
			k8sNetworks = garden.K8SNetworks{
				Pods:     &podCIDR,
				Services: &serviceCIDR,
				Nodes:    &nodesCIDR,
			}

			falseVar = false

			seedName      = "seed"
			namespaceName = "garden-my-project"
			projectName   = "my-project"

			unmanagedDNSProvider = garden.DNSUnmanaged
			baseDomain           = "example.com"

			validMachineImageName         = "some-machineimage"
			validMachineImageVersions     = []garden.ExpirableVersion{{Version: "0.0.1"}}
			validShootMachineImageVersion = "0.0.1"
			volumeType                    = "volume-type-1"

			seedPodsCIDR     = "10.241.128.0/17"
			seedServicesCIDR = "10.241.0.0/17"
			seedNodesCIDR    = "10.240.0.0/16"
			seedSecret       = corev1.Secret{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedName,
					Namespace: "garden",
				},
				Data: map[string][]byte{
					kubernetes.KubeConfig: []byte(""),
				},
				Type: corev1.SecretTypeOpaque,
			}

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
				Spec: garden.CloudProfileSpec{
					Type: "unknown",
					Kubernetes: garden.KubernetesSettings{
						Versions: []garden.ExpirableVersion{{Version: "1.6.4"}},
					},
					MachineImages: []garden.CloudProfileMachineImage{
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
					Regions: []garden.Region{
						{
							Name:  "europe",
							Zones: []garden.AvailabilityZone{{Name: "europe-a"}},
						},
						{
							Name:  "asia",
							Zones: []garden.AvailabilityZone{{Name: "asia-a"}},
						},
					},
				},
			}
			seedBase = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Networks: garden.SeedNetworks{
						Pods:     seedPodsCIDR,
						Services: seedServicesCIDR,
						Nodes:    &seedNodesCIDR,
					},
					SecretRef: &corev1.SecretReference{
						Name:      seedSecret.Name,
						Namespace: seedSecret.Namespace,
					},
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: garden.ShootSpec{
					CloudProfileName:  "profile",
					Region:            "europe",
					SecretBindingName: "my-secret",
					SeedName:          &seedName,
					DNS: &garden.DNS{
						Domain: test.MakeStrPointer(fmt.Sprintf("shoot.%s", baseDomain)),
						Providers: []garden.DNSProvider{
							{
								Type: &unmanagedDNSProvider,
							},
						},
					},
					Kubernetes: garden.Kubernetes{
						Version: "1.6.4",
					},
					Networking: garden.Networking{
						Nodes:    k8sNetworks.Nodes,
						Pods:     k8sNetworks.Pods,
						Services: k8sNetworks.Services,
					},
					Provider: garden.Provider{
						Type: "unknown",
						Workers: []garden.Worker{
							{
								Name: "worker-name",
								Machine: garden.Machine{
									Type: "machine-type-1",
								},
								Minimum: 1,
								Maximum: 1,
								Volume: &garden.Volume{
									Size: "40Gi",
									Type: &volumeType,
								},
								Zones: []string{"europe-a"},
							},
						},
					},
				},
			}
		)

		BeforeEach(func() {
			project = projectBase
			cloudProfile = *cloudProfileBase.DeepCopy()
			seed = seedBase
			shoot = *shootBase.DeepCopy()

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
				oldShoot *garden.Shoot
			)

			BeforeEach(func() {
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()

				// set seed name
				shoot.Spec.SeedName = &seedName

				// set old shoot for update
				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil
			})

			It("create should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because the Seed is now protected but the same while shoot has never been in garden namespace", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}
				oldShoot.Spec.SeedName = shoot.Spec.SeedName
				shoot.Spec.Provider.Workers[0].Maximum += 1
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should fail because the new Seed specified in shoot manifest is protected while shoot is not in garden namespace", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("create should pass because shoot is not in garden namespace and seed is not protected", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because shoot is not in garden namespace and seed is not protected", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("create should fail because shoot is not in garden namespace and seed is protected", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("update should fail because shoot is not in garden namespace and seed is protected", func() {
				seed.Spec.Taints = []garden.SeedTaint{{Key: garden.SeedTaintProtected}}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("create should pass because shoot is in garden namespace and seed is not protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("update should pass because shoot is in garden namespace and seed is not protected", func() {
				ns := "garden"
				shoot.Namespace = ns
				project.Spec.Namespace = &ns

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsBadRequest(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("name must not exceed"))
			})

			It("should not test length constraints for operations other than CREATE", func() {
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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))

				attrs = admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)
				err = admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring("name must not exceed"))
			})
		})

		Context("finalizer removal checks", func() {
			var (
				oldShoot *garden.Shoot
			)

			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()

				shoot.Status.TechnicalID = "some-id"
				shoot.Status.LastOperation = &garden.LastOperation{
					Type:     garden.LastOperationTypeReconcile,
					State:    garden.LastOperationStateSucceeded,
					Progress: 100,
				}

				// set old shoot for update and add gardener finalizer to it
				oldShoot = shoot.DeepCopy()
				finalizers := sets.NewString(oldShoot.GetFinalizers()...)
				finalizers.Insert(garden.GardenerName)
				oldShoot.SetFinalizers(finalizers.UnsortedList())
			})

			It("should reject removing the gardener finalizer if the shoot has not yet been deleted successfully", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shoot deletion has not completed successfully yet"))
			})

			It("should admit removing the gardener finalizer if the shoot deletion succeeded ", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				shoot.Status.LastOperation = &garden.LastOperation{
					Type:     garden.LastOperationTypeDelete,
					State:    garden.LastOperationStateSucceeded,
					Progress: 100,
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("hibernation checks", func() {
			var (
				oldShoot *garden.Shoot
			)

			BeforeEach(func() {
				shoot = *shootBase.DeepCopy()
				oldShoot = shoot.DeepCopy()
				oldShoot.Spec.Hibernation = &garden.Hibernation{Enabled: pointer.BoolPtr(false)}

				shoot.Spec.Hibernation = &garden.Hibernation{Enabled: pointer.BoolPtr(true)}
			})

			DescribeTable("should allow/deny hibernating the Shoot according to HibernationPossible constraint",
				func(constraints []garden.Condition, match types.GomegaMatcher) {
					_ = gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
					_ = gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
					_ = gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

					shoot.Status.Constraints = constraints

					attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Admit(context.TODO(), attrs, nil)
					Expect(err).To(match)
				},
				Entry("should allow if set to True", []garden.Condition{
					{
						Type:   garden.ShootHibernationPossible,
						Status: garden.ConditionTrue,
					},
				}, Not(HaveOccurred())),
				Entry("should deny if set to False", []garden.Condition{
					{
						Type:    garden.ShootHibernationPossible,
						Status:  garden.ConditionFalse,
						Message: "foo",
					},
				}, And(HaveOccurred(), MatchError(ContainSubstring("foo")))),
				Entry("should deny if set to Unknown", []garden.Condition{
					{
						Type:    garden.ShootHibernationPossible,
						Status:  garden.ConditionUnknown,
						Message: "foo",
					},
				}, And(HaveOccurred(), MatchError(ContainSubstring("foo")))),
				Entry("should allow if unset", []garden.Condition{}, Not(HaveOccurred())),
			)
		})

		Context("checks for shoots referencing a deleted seed", func() {
			var (
				oldShoot *garden.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()

				seed = *seedBase.DeepCopy()
				now := metav1.Now()
				seed.DeletionTimestamp = &now

				_ = gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				_ = gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				_ = gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			})

			It("should reject creating a shoot on a seed which is marked for deletion", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot create shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name)))
			})

			It("should allow no-op updates", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow modifying the finalizers array", func() {
				oldShoot.Finalizers = []string{garden.GardenerName}
				shoot.Finalizers = []string{}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow adding the deletion confirmation", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations[common.ConfirmationDeletion] = "true"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject modifying the shoot spec when seed is marked for deletion", func() {
				shoot.Spec.Region = "other-region"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name)))
			})

			It("should reject modifying other annotations than the deletion confirmation when seed is marked for deletion", func() {
				shoot.Annotations = make(map[string]string)
				shoot.Annotations["foo"] = "bar"

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name)))
			})
		})

		It("should reject because the referenced cloud profile was not found", func() {
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced seed was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the referenced project was not found", func() {
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the cloud provider in shoot and profile differ", func() {
			cloudProfile.Spec.Type = "gcp"
			shoot.Spec.Provider.Type = "aws"

			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		})

		It("should reject because the provider type was changed", func() {
			shoot2 := shoot.DeepCopy()
			shoot.Spec.Provider.Type = "aws"
			shoot2.Spec.Provider.Type = "gcp"

			gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, shoot2, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(apierrors.NewBadRequest("shoot provider type was changed which is not allowed")))
		})

		Context("tests for infrastructure update", func() {
			var (
				oldShoot *garden.Shoot
			)

			BeforeEach(func() {
				oldShoot = shootBase.DeepCopy()
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			})

			It("should add deploy infrastructure task because spec has changed", func() {
				shoot.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: []byte("infrastructure"),
					},
				}

				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, common.ShootTaskDeployInfrastructure)).To(BeTrue())
			})

			It("should not add deploy infrastructure task because spec has not changed", func() {
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(controllerutils.HasTask(shoot.ObjectMeta.Annotations, common.ShootTaskDeployInfrastructure)).ToNot(BeTrue())
			})
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
					},
				}
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &garden.Volume{
							Size: "10Gi",
							Type: &volumeType,
						},
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
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				awsCloud.Networks = garden.AWSNetworks{K8SNetworks: k8sNetworks}
				awsCloud.Workers = workers
				awsCloud.Zones = zones
				awsCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "aws"
				cloudProfile.Spec.AWS = awsProfile
				shoot.Spec.Cloud.AWS = awsCloud
				shoot.Spec.Provider.Type = "aws"
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.AWS.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeNil())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should not reject due to an usable machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject due to a not usable machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-old",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.AWS.Zones = append(shoot.Spec.Cloud.AWS.Zones, oldShoot.Spec.Cloud.AWS.Zones...)
				shoot.Spec.Cloud.AWS.Zones = append(shoot.Spec.Cloud.AWS.Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.AWS.Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
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
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &garden.Volume{
							Size: "10Gi",
							Type: &volumeType,
						},
					},
				}
				machineImage = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: validShootMachineImageVersion,
				}
				azureCloud = &garden.AzureCloud{}
			)

			BeforeEach(func() {
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				cloudProfile.Spec.Azure = azureProfile
				azureCloud.Networks = garden.AzureNetworks{K8SNetworks: k8sNetworks}
				azureCloud.Workers = workers
				azureCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "azure"
				shoot.Spec.Provider.Type = "azure"
				shoot.Spec.Cloud.Azure = azureCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Azure.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no fault domain count has been specified", func() {
				shoot.Spec.Region = "asia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid region where no update domain count has been specified", func() {
				shoot.Spec.Region = "australia"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
					},
				}
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &garden.Volume{
							Size: "10Gi",
							Type: &volumeType,
						},
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
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				gcpCloud.Networks = garden.GCPNetworks{K8SNetworks: k8sNetworks}
				gcpCloud.Workers = workers
				gcpCloud.Zones = zones
				gcpCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "gcp"
				shoot.Spec.Provider.Type = "gcp"
				cloudProfile.Spec.GCP = gcpProfile
				shoot.Spec.Cloud.GCP = gcpCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.GCP.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.GCP.Zones = append(shoot.Spec.Cloud.GCP.Zones, oldShoot.Spec.Cloud.GCP.Zones...)
				shoot.Spec.Cloud.GCP.Zones = append(shoot.Spec.Cloud.GCP.Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.GCP.Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
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
					},
				}
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &garden.Volume{
							Size: "10Gi",
							Type: &volumeType,
						},
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
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				packetCloud.Networks = garden.PacketNetworks{K8SNetworks: k8sNetworks}
				packetCloud.Workers = workers
				packetCloud.Zones = zones
				packetCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "packet"
				shoot.Spec.Provider.Type = "packet"
				cloudProfile.Spec.Packet = packetProfile
				shoot.Spec.Cloud.Packet = packetCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Packet.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Packet.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.Packet.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.Packet.Zones = append(shoot.Spec.Cloud.Packet.Zones, oldShoot.Spec.Cloud.Packet.Zones...)
				shoot.Spec.Cloud.Packet.Zones = append(shoot.Spec.Cloud.Packet.Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.Packet.Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
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
						LoadBalancerProviders: []garden.OpenStackLoadBalancerProvider{
							{
								Name: "haproxy",
							},
						},
					},
				}
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
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
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				openStackCloud.FloatingPoolName = "pool"
				openStackCloud.LoadBalancerProvider = "haproxy"
				openStackCloud.Networks = garden.OpenStackNetworks{K8SNetworks: k8sNetworks}
				openStackCloud.Workers = workers
				openStackCloud.Zones = zones
				openStackCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "openstack"
				shoot.Spec.Provider.Type = "openstack"
				cloudProfile.Spec.OpenStack = openStackProfile
				shoot.Spec.Cloud.OpenStack = openStackCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should not reject due to an undefined dns domain", func() {
				shoot.Spec.DNS.Domain = nil

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Succeed())
			})

			It("should reject due to an invalid floating pool name", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = "invalid-pool"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid load balancer provider", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = "invalid-provider"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.OpenStack.Zones = append(shoot.Spec.Cloud.OpenStack.Zones, oldShoot.Spec.Cloud.OpenStack.Zones...)
				shoot.Spec.Cloud.OpenStack.Zones = append(shoot.Spec.Cloud.OpenStack.Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.OpenStack.Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("tests for Alicloud", func() {
			var (
				alicloudProfile = &garden.AlicloudProfile{
					Constraints: garden.AlicloudConstraints{
						DNSProviders: []garden.DNSProviderConstraint{
							{
								Name: garden.DNSUnmanaged,
							},
						},
					},
				}
				workers = []garden.Worker{
					{
						Name: "worker-name",
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Minimum: 1,
						Maximum: 1,
						Volume: &garden.Volume{
							Size: "10Gi",
							Type: &volumeType,
						},
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
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				aliCloud.Networks = garden.AlicloudNetworks{K8SNetworks: k8sNetworks}
				aliCloud.Workers = workers
				aliCloud.Zones = zones
				aliCloud.MachineImage = machineImage
				cloudProfile.Spec.Type = "alicloud"
				shoot.Spec.Provider.Type = "alicloud"
				cloudProfile.Spec.Alicloud = alicloudProfile
				shoot.Spec.Cloud.Alicloud = aliCloud
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Cloud.Alicloud.Networks.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Cloud.Alicloud.Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.Alicloud.Zones = append(shoot.Spec.Cloud.Alicloud.Zones, oldShoot.Spec.Cloud.Alicloud.Zones...)
				shoot.Spec.Cloud.Alicloud.Zones = append(shoot.Spec.Cloud.Alicloud.Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Cloud.Alicloud.Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject due to an machine type is not available in shoot zones", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
					},
				}

				cloudProfile.Spec.Regions[0].Zones[0].UnavailableMachineTypes = []string{"machine-type-1"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an volume type is not available in shoot zones", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{
					{
						Volume: &garden.Volume{
							Type: &volumeType,
						},
					},
				}

				cloudProfile.Spec.Regions[0].Zones[0].UnavailableVolumeTypes = []string{"volume-type-1"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})
		})

		Context("tests for unknown provider", func() {
			var workers = []garden.Worker{
				{
					Name: "worker-name",
					Machine: garden.Machine{
						Type: "machine-type-1",
					},
					Minimum: 1,
					Maximum: 1,
					Volume: &garden.Volume{
						Size: "10Gi",
						Type: &volumeType,
					},
					Zones: []string{"europe-a"},
				},
			}

			BeforeEach(func() {
				cloudProfile = *cloudProfileBase.DeepCopy()
				shoot = *shootBase.DeepCopy()
				shoot.Spec.Provider.Workers = workers
			})

			It("should pass because no seed has to be specified (however can be). The scheduler sets the seed instead.", func() {
				shoot.Spec.SeedName = nil
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the shoot node and the seed node networks intersect", func() {
				shoot.Spec.Networking.Nodes = &seedNodesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot pod and the seed pod networks intersect", func() {
				shoot.Spec.Networking.Pods = &seedPodsCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because the shoot service and the seed service networks intersect", func() {
				shoot.Spec.Networking.Services = &seedServicesCIDR

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

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

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(BeNil())
			})

			It("should reject due to an invalid kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.2.3"

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should default a major.minor kubernetes version to latest patch version", func() {
				shoot.Spec.Kubernetes.Version = "1.6"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.6.6"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, highestPatchVersion, garden.ExpirableVersion{Version: "1.7.1"}, garden.ExpirableVersion{Version: "1.7.2"})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
				Expect(shoot.Spec.Kubernetes.Version).To(Equal(highestPatchVersion.Version))
			})

			It("should reject: default only exactly matching minor kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = "1.8"
				highestPatchVersion := garden.ExpirableVersion{Version: "1.81.5"}
				cloudProfile.Spec.Kubernetes.Versions = append(cloudProfile.Spec.Kubernetes.Versions, garden.ExpirableVersion{Version: "1.81.0"}, highestPatchVersion)

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine image", func() {
				shoot.Spec.Provider.Workers[0].Machine.Image = &garden.ShootMachineImage{
					Name:    "not-supported",
					Version: "not-supported",
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to a machine image with expiration date in the past", func() {
				imageVersionExpired := "0.0.1-beta"

				shoot.Spec.Provider.Workers[0].Machine.Image = &garden.ShootMachineImage{
					Name:    validMachineImageName,
					Version: imageVersionExpired,
				}

				timeInThePast := metav1.Now().Add(time.Second * -1000)
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: validMachineImageName,
					Versions: []garden.ExpirableVersion{
						{
							Version:        imageVersionExpired,
							ExpirationDate: &metav1.Time{Time: timeInThePast},
						},
					},
				}, garden.CloudProfileMachineImage{
					Name: "other-image-name",
					Versions: []garden.ExpirableVersion{
						{
							Version: imageVersionExpired,
						},
					},
				})

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should use latest machine image as old shoot does not specify one", func() {
				imageName := "some-image"
				version1 := "1.1.1"
				version2 := "2.2.2"

				cloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{
					{
						Name: imageName,
						Versions: []garden.ExpirableVersion{
							{
								Version: version2,
							},
							{
								Version:        version1,
								ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
							},
						},
					},
				}

				shoot.Spec.Provider.Workers[0].Machine.Image = nil

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&garden.ShootMachineImage{
					Name:    imageName,
					Version: version2,
				}))
			})

			It("should not touch the machine image of the old shoot", func() {
				imageName := "some-image"
				version1 := "1.1.1"
				version2 := "2.2.2"

				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: imageName,
					Versions: []garden.ExpirableVersion{
						{
							Version: version2,
						},
						{
							Version:        version1,
							ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
						},
					},
				})

				shoot.Spec.Provider.Workers[0].Machine.Image = &garden.ShootMachineImage{
					Name:    imageName,
					Version: version1,
				}
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Provider.Workers[0].Machine.Image = nil

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(newShoot, &shoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*newShoot).To(Equal(shoot))
			})

			It("should respect the desired machine image of the new shoot", func() {
				imageName := "some-image"
				version1 := "1.1.1"
				version2 := "2.2.2"

				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, garden.CloudProfileMachineImage{
					Name: imageName,
					Versions: []garden.ExpirableVersion{
						{
							Version: version2,
						},
						{
							Version:        version1,
							ExpirationDate: &metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
						},
					},
				})

				shoot.Spec.Provider.Workers[0].Machine.Image = &garden.ShootMachineImage{
					Name:    imageName,
					Version: version1,
				}
				newShoot := shoot.DeepCopy()
				newShoot.Spec.Provider.Workers[0].Machine.Image = &garden.ShootMachineImage{
					Name:    imageName,
					Version: version2,
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(newShoot, &shoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(newShoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(&garden.ShootMachineImage{
					Name:    imageName,
					Version: version2,
				}))
			})

			It("should not reject due to an usable machine type", func() {
				shoot.Spec.Provider.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject due to a not usable machine type", func() {
				shoot.Spec.Provider.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-old",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid machine type", func() {
				shoot.Spec.Provider.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "not-allowed",
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid volume type", func() {
				notAllowed := "not-allowed"
				shoot.Spec.Provider.Workers = []garden.Worker{
					{
						Machine: garden.Machine{
							Type: "machine-type-1",
						},
						Volume: &garden.Volume{
							Type: &notAllowed,
						},
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone", func() {
				shoot.Spec.Provider.Workers[0].Zones = []string{"invalid-zone"}

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject due to an invalid zone update", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, oldShoot.Spec.Provider.Workers[0].Zones...)
				shoot.Spec.Provider.Workers[0].Zones = append(shoot.Spec.Provider.Workers[0].Zones, "invalid-zone")

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should allow update when zone has removed from CloudProfile", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers[0].Zones = []string{}
				cloudProfile.Spec.Regions = cloudProfile.Spec.Regions[1:]

				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
