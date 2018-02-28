// Copyright 2018 The Gardener Authors.
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
			admissionHandler      *RejectShootIfQuotaExceeded
			gardenInformerFactory gardeninformers.SharedInformerFactory
			shoot                 garden.Shoot
			crossSB               garden.CrossSecretBinding
			quota                 garden.Quota
			cloudProfile          garden.CloudProfile
			namespace             string = "test"
			trialNamespace        string = "trial"
			clusterLifeTime       int    = 7
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

			quotaProject = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "project-quota",
				},
				Spec: garden.QuotaSpec{
					ClusterLifetimeDays: &clusterLifeTime,
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

			crossSBBase = garden.CrossSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-crossSB",
				},
				Quotas: []corev1.ObjectReference{
					corev1.ObjectReference{
						Namespace: trialNamespace,
						Name:      "project-quota",
					},
				},
			}

			shootSpecCloudGCEBase = garden.GCPCloud{
				Workers: []garden.GCPWorker{
					garden.GCPWorker{
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
						SecretBindingRef: corev1.ObjectReference{
							Kind: "CrossSecretBinding",
							Name: "test-crossSB",
						},
						GCP: &shootSpecCloudGCEBase,
					},
				},
			}
		)

		BeforeSuite(func() {
			logger.Logger = logger.NewLogger("")
		})

		BeforeEach(func() {
			admissionHandler, _ = New()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)

			shoot = shootBase
			cloudProfile = cloudProfileBase
			crossSB = crossSBBase
			quota = quotaProject

			shoot.Spec.Cloud.GCP.Workers[0].AutoScalerMax = 1
		})

		Context("tests for quota validation common cases", func() {
			It("should pass because shoot is intended to get deleted", func() {
				var now metav1.Time
				now.Time = time.Now()
				shoot.DeletionTimestamp = &now
				shoot.Annotations = map[string]string{
					common.ConfirmationDeletionTimestamp: now.Time.Format(time.RFC3339),
				}

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because shoots secret binding has no quotas referenced", func() {
				crossSB.Quotas = make([]corev1.ObjectReference, 0)
				gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

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
						ClusterLifetimeDays: &clusterLifeTime,
						Scope:               garden.QuotaScopeProject,
						Metrics:             corev1.ResourceList{},
					},
				}
				crossSB.Quotas = []corev1.ObjectReference{
					{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
				}

				gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&emptyQuota)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for shoots with CrossSecretBindings, which have only one quota referenced", func() {
			It("should pass because quota is sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail because quota limits are exceeded", func() {
				shoot.Spec.Cloud.GCP.Workers[0].AutoScalerMax = 2
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
			})

		})

		Context("tests for shoots with PrivateSecretBindings, which have multiple quotas referenced", func() {
			var (
				quota2 = garden.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: trialNamespace,
						Name:      "secret-quota",
					},
					Spec: garden.QuotaSpec{
						ClusterLifetimeDays: &clusterLifeTime,
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

				privateSB = garden.PrivateSecretBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "test-privateSB",
					},
					Quotas: []corev1.ObjectReference{
						{
							Namespace: trialNamespace,
							Name:      "secret-quota",
						},
						{
							Namespace: trialNamespace,
							Name:      "project-quota",
						},
					},
				}
			)

			BeforeEach(func() {
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)
				gardenInformerFactory.Garden().InternalVersion().PrivateSecretBindings().Informer().GetStore().Add(&privateSB)
				shoot.Spec.Cloud.SecretBindingRef = corev1.ObjectReference{
					Kind: "PrivateSecretBinding",
					Name: "test-privateSB",
				}
			})

			It("should pass because all quotas are sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())

			})

			It("should fail because limits of one quota is exceeded", func() {
				shoot.Spec.Cloud.GCP.Workers[0].AutoScalerMax = 2
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for OpenStack shoots, which have special logic for collecting volume and machine types", func() {
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
				cloudProfile.Spec.GCP = nil
				cloudProfile.Spec.OpenStack = &cloudProfileOpenStack
				shoot.Spec.Cloud.GCP = nil
				shoot.Spec.Cloud.OpenStack = &shootSpecCloudOpenStack
			})

			AfterEach(func() {
				cloudProfile.Spec.GCP = &cloudProfileGCEBase
				cloudProfile.Spec.OpenStack = nil
				shoot.Spec.Cloud.GCP = &shootSpecCloudGCEBase
				shoot.Spec.Cloud.OpenStack = nil
			})

			It("should pass because quota is sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)
				Expect(err).NotTo(HaveOccurred())
			})

		})
	})
})
