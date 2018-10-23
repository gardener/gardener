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

package quotavalidator_test

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

var _ = Describe("quotavalidator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *QuotaValidator
			gardenInformerFactory gardeninformers.SharedInformerFactory
			shoot                 garden.Shoot
			oldShoot              garden.Shoot
			secretBinding         garden.SecretBinding
			quotaProject          garden.Quota
			quotaSecret           garden.Quota
			cloudProfile          garden.CloudProfile
			namespace             string = "test"
			trialNamespace        string = "trial"
			machineTypeName       string = "n1-standard-2"
			volumeTypeName        string = "pd-standard"

			cloudProfileGCEBase = garden.GCPProfile{
				Constraints: garden.GCPConstraints{
					MachineTypes: []garden.MachineType{
						{
							Name:   machineTypeName,
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("5Gi"),
						},
					},
					VolumeTypes: []garden.VolumeType{
						{
							Name:  volumeTypeName,
							Class: "standard",
						},
					},
					Kubernetes: garden.KubernetesConstraints{
						Versions: []string{
							"1.0.1",
							"1.1.1",
						},
					},
				},
			}

			cloudProfileBase = garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: garden.CloudProfileSpec{
					GCP: &cloudProfileGCEBase,
				},
			}

			quotaProjectLifetime = 1
			quotaProjectBase     = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "project-quota",
				},
				Spec: garden.QuotaSpec{
					ClusterLifetimeDays: &quotaProjectLifetime,
					Scope:               garden.QuotaScopeProject,
					Metrics: corev1.ResourceList{
						garden.QuotaMetricCPU:             resource.MustParse("2"),
						garden.QuotaMetricGPU:             resource.MustParse("0"),
						garden.QuotaMetricMemory:          resource.MustParse("5Gi"),
						garden.QuotaMetricStorageStandard: resource.MustParse("30Gi"),
						garden.QuotaMetricStoragePremium:  resource.MustParse("0Gi"),
						garden.QuotaMetricLoadbalancer:    resource.MustParse("2"),
					},
				},
			}

			quotaSecretLifetime = 7
			quotaSecretBase     = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "secret-quota",
				},
				Spec: garden.QuotaSpec{
					ClusterLifetimeDays: &quotaSecretLifetime,
					Scope:               garden.QuotaScopeSecret,
					Metrics: corev1.ResourceList{
						garden.QuotaMetricCPU:             resource.MustParse("4"),
						garden.QuotaMetricGPU:             resource.MustParse("0"),
						garden.QuotaMetricMemory:          resource.MustParse("10Gi"),
						garden.QuotaMetricStorageStandard: resource.MustParse("60Gi"),
						garden.QuotaMetricStoragePremium:  resource.MustParse("0Gi"),
						garden.QuotaMetricLoadbalancer:    resource.MustParse("4"),
					},
				},
			}

			secretBindingBase = garden.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-binding",
				},
				Quotas: []corev1.ObjectReference{
					{
						Namespace: trialNamespace,
						Name:      "project-quota",
					},
					{
						Namespace: trialNamespace,
						Name:      "secret-quota",
					},
				},
			}

			shootSpecCloudGCEBase = garden.GCPCloud{
				Workers: []garden.GCPWorker{
					{
						Worker: garden.Worker{
							Name:          "test-worker-1",
							MachineType:   machineTypeName,
							AutoScalerMax: 1,
							AutoScalerMin: 1,
						},
						VolumeType: volumeTypeName,
						VolumeSize: "30Gi",
					},
				},
			}

			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-shoot",
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: "profile",
						SecretBindingRef: corev1.LocalObjectReference{
							Name: "test-binding",
						},
						GCP: &shootSpecCloudGCEBase,
					},
					Kubernetes: garden.Kubernetes{
						Version: "1.0.1",
					},
					Addons: &garden.Addons{
						NginxIngress: &garden.NginxIngress{
							Addon: garden.Addon{
								Enabled: true,
							},
						},
					},
				},
			}
		)

		BeforeSuite(func() {
			logger.Logger = logger.NewLogger("")
		})

		BeforeEach(func() {
			shoot = *shootBase.DeepCopy()
			cloudProfile = *cloudProfileBase.DeepCopy()
			secretBinding = *secretBindingBase.DeepCopy()
			quotaProject = *quotaProjectBase.DeepCopy()
			quotaSecret = *quotaSecretBase.DeepCopy()

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaSecret)
			gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBindingBase)
		})

		Context("tests for Shoots, which have at least one Quota referenced", func() {
			It("should pass because all quotas limits are sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail because the limits of at least one quota are exceeded", func() {
				shoot.Spec.Cloud.GCP.Workers[0].AutoScalerMax = 2
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
			})

			It("should fail because other shoots exhaust quota limits", func() {
				shoot2 := *shoot.DeepCopy()
				shoot2.Name = "test-shoot-2"
				gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore().Add(&shoot2)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
			})

			It("should pass because can update non worker property although quota is exceeded", func() {
				oldShoot = *shoot.DeepCopy()
				quotaProject.Spec.Metrics[garden.QuotaMetricCPU] = resource.MustParse("1")
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)

				shoot.Spec.Kubernetes.Version = "1.1.1"
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for Quota validation corner cases", func() {
			It("should pass because shoot is intended to get deleted", func() {
				var now metav1.Time
				now.Time = time.Now()
				shoot.DeletionTimestamp = &now

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because shoots secret binding has no quotas referenced", func() {
				secretBinding.Quotas = make([]corev1.ObjectReference, 0)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass shoots secret binding having quota with no metrics", func() {
				emptyQuotaName := "empty-quota"
				emptyQuota := garden.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
					Spec: garden.QuotaSpec{
						ClusterLifetimeDays: &quotaProjectLifetime,
						Scope:               garden.QuotaScopeProject,
						Metrics:             corev1.ResourceList{},
					},
				}
				secretBinding.Quotas = []corev1.ObjectReference{
					{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&emptyQuota)
				gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for OpenStack Shoots, which have special logic for collecting volume and machine types", func() {
			var (
				machineTypeNameOpenStack = "mx-test-1"
				cloudProfileOpenStack    = garden.OpenStackProfile{
					Constraints: garden.OpenStackConstraints{
						MachineTypes: []garden.OpenStackMachineType{
							{
								MachineType: garden.MachineType{
									Name:   machineTypeNameOpenStack,
									CPU:    resource.MustParse("2"),
									GPU:    resource.MustParse("0"),
									Memory: resource.MustParse("5Gi"),
								},
								VolumeType: "standard",
								VolumeSize: resource.MustParse("30Gi"),
							},
						},
					},
				}
				shootSpecCloudOpenStack = garden.OpenStackCloud{
					Workers: []garden.OpenStackWorker{
						{
							Worker: garden.Worker{
								Name:          "test-worker-1",
								MachineType:   machineTypeNameOpenStack,
								AutoScalerMax: 1,
								AutoScalerMin: 1,
							},
						},
					},
				}
			)

			BeforeEach(func() {
				cloudProfile.Spec.OpenStack = &cloudProfileOpenStack
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				shoot.Spec.Cloud.GCP = nil
				shoot.Spec.Cloud.OpenStack = &shootSpecCloudOpenStack
			})

			It("should pass because quota is sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for extending the lifetime of a Shoot", func() {
			BeforeEach(func() {
				annotations := map[string]string{
					common.ShootExpirationTimestamp: "2018-01-01T00:00:00+00:00",
				}
				shoot.Annotations = annotations
				oldShoot = *shoot.DeepCopy()
			})

			It("should pass because no quota prescribe a clusterLifetime", func() {
				quotaProject.Spec.ClusterLifetimeDays = nil
				quotaSecret.Spec.ClusterLifetimeDays = nil
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaSecret)

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass as shoot expiration time can be extended", func() {
				shoot.Annotations[common.ShootExpirationTimestamp] = "2018-01-02T00:00:00+00:00" // plus 1 day
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail as shoots expiration time can’t be extended, because requested time higher then quota allows", func() {
				shoot.Annotations[common.ShootExpirationTimestamp] = "2018-01-09T00:00:00+00:00" // plus 8 days
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Alicloud Shoots, which have special logic for collecting volume and machine types", func() {
			var (
				cloudProfileAlicloudBase = garden.AlicloudProfile{
					Constraints: garden.AlicloudConstraints{
						MachineTypes: []garden.AlicloudMachineType{
							{
								MachineType: garden.MachineType{
									Name:   machineTypeName,
									CPU:    resource.MustParse("2"),
									GPU:    resource.MustParse("0"),
									Memory: resource.MustParse("5Gi"),
								},
							},
						},
						VolumeTypes: []garden.AlicloudVolumeType{
							{
								VolumeType: garden.VolumeType{
									Name:  volumeTypeName,
									Class: "standard",
								},
							},
						},
						Kubernetes: garden.KubernetesConstraints{
							Versions: []string{
								"1.0.1",
								"1.1.1",
							},
						},
					},
				}

				shootSpecCloudAlicloud = garden.Alicloud{
					Workers: []garden.AlicloudWorker{
						{
							Worker: garden.Worker{
								Name:          "test-worker-1",
								MachineType:   machineTypeName,
								AutoScalerMax: 1,
								AutoScalerMin: 1,
							},
							VolumeType: volumeTypeName,
							VolumeSize: "30Gi",
						},
					},
				}
			)

			BeforeEach(func() {
				cloudProfile.Spec.Alicloud = &cloudProfileAlicloudBase
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				shoot.Spec.Cloud.GCP = nil
				shoot.Spec.Cloud.Alicloud = &shootSpecCloudAlicloud
			})

			It("should pass because quota is sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because can update non worker property although quota is exceeded", func() {
				oldShoot = *shoot.DeepCopy()
				quotaProject.Spec.Metrics[garden.QuotaMetricCPU] = resource.MustParse("1")
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)

				shoot.Spec.Kubernetes.Version = "1.1.1"
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Update, false, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
