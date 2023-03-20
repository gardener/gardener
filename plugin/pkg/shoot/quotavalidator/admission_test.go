// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	mocktime "github.com/gardener/gardener/pkg/utils/time/mock"
	. "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
)

var _ = Describe("quotavalidator", func() {
	Describe("#Admit", func() {
		var (
			ctrl *gomock.Controller

			admissionHandler    *QuotaValidator
			coreInformerFactory gardencoreinformers.SharedInformerFactory
			timeOps             *mocktime.MockOps
			shoot               core.Shoot
			oldShoot            core.Shoot
			secretBinding       core.SecretBinding
			quotaProject        core.Quota
			quotaSecret         core.Quota
			cloudProfile        core.CloudProfile
			namespace           string = "test"
			trialNamespace      string = "trial"
			machineTypeName     string = "n1-standard-2"
			machineTypeName2    string = "machtype2"
			volumeTypeName      string = "pd-standard"

			cloudProfileBase = core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: core.CloudProfileSpec{
					MachineTypes: []core.MachineType{
						{
							Name:   machineTypeName,
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("5Gi"),
						},
						{
							Name:   machineTypeName2,
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("5Gi"),
							Storage: &core.MachineTypeStorage{
								Class: core.VolumeClassStandard,
							},
						},
					},
					VolumeTypes: []core.VolumeType{
						{
							Name:  volumeTypeName,
							Class: "standard",
						},
					},
				},
			}

			quotaProjectLifetime int32 = 1
			quotaProjectBase           = core.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "project-quota",
				},
				Spec: core.QuotaSpec{
					ClusterLifetimeDays: &quotaProjectLifetime,
					Scope: corev1.ObjectReference{
						APIVersion: "core.gardener.cloud/v1beta1",
						Kind:       "Project",
					},
					Metrics: corev1.ResourceList{
						core.QuotaMetricCPU:             resource.MustParse("2"),
						core.QuotaMetricGPU:             resource.MustParse("0"),
						core.QuotaMetricMemory:          resource.MustParse("5Gi"),
						core.QuotaMetricStorageStandard: resource.MustParse("30Gi"),
						core.QuotaMetricStoragePremium:  resource.MustParse("0Gi"),
						core.QuotaMetricLoadbalancer:    resource.MustParse("2"),
					},
				},
			}

			quotaSecretLifetime int32 = 7
			quotaSecretBase           = core.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "secret-quota",
				},
				Spec: core.QuotaSpec{
					ClusterLifetimeDays: &quotaSecretLifetime,
					Scope: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					Metrics: corev1.ResourceList{
						core.QuotaMetricCPU:             resource.MustParse("4"),
						core.QuotaMetricGPU:             resource.MustParse("0"),
						core.QuotaMetricMemory:          resource.MustParse("10Gi"),
						core.QuotaMetricStorageStandard: resource.MustParse("60Gi"),
						core.QuotaMetricStoragePremium:  resource.MustParse("0Gi"),
						core.QuotaMetricLoadbalancer:    resource.MustParse("4"),
					},
				},
			}

			secretBindingBase = core.SecretBinding{
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

			workersBase = []core.Worker{
				{
					Name: "test-worker-1",
					Machine: core.Machine{
						Type: machineTypeName,
					},
					Maximum: 1,
					Minimum: 1,
					Volume: &core.Volume{
						VolumeSize: "30Gi",
						Type:       &volumeTypeName,
					},
				},
			}

			workersBase2 = []core.Worker{
				{
					Name: "test-worker-1",
					Machine: core.Machine{
						Type: machineTypeName,
					},
					Maximum: 1,
					Minimum: 1,
					Volume: &core.Volume{
						VolumeSize: "30Gi",
						Type:       &volumeTypeName,
					},
				},
				{
					Name: "test-worker-2",
					Machine: core.Machine{
						Type: machineTypeName,
					},
					Maximum: 1,
					Minimum: 1,
					Volume: &core.Volume{
						VolumeSize: "30Gi",
						Type:       &volumeTypeName,
					},
				},
			}

			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-shoot",
				},
				Spec: core.ShootSpec{
					CloudProfileName:  "profile",
					SecretBindingName: "test-binding",
					Provider: core.Provider{
						Workers: workersBase,
					},
					Kubernetes: core.Kubernetes{
						Version: "1.0.1",
					},
					Addons: &core.Addons{
						NginxIngress: &core.NginxIngress{
							Addon: core.Addon{
								Enabled: true,
							},
						},
					},
				},
			}
		)

		BeforeEach(func() {
			shoot = *shootBase.DeepCopy()
			cloudProfile = *cloudProfileBase.DeepCopy()
			secretBinding = *secretBindingBase.DeepCopy()
			quotaProject = *quotaProjectBase.DeepCopy()
			quotaSecret = *quotaSecretBase.DeepCopy()

			ctrl = gomock.NewController(GinkgoT())
			timeOps = mocktime.NewMockOps(ctrl)

			admissionHandler, _ = New(timeOps)
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)
			Expect(coreInformerFactory.Core().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())
			Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quotaSecret)).To(Succeed())
			Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBindingBase)).To(Succeed())
		})

		Context("tests for Shoots, which have at least one Quota referenced", func() {
			It("should pass because all quotas limits are sufficient", func() {
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail because the limits of at least one quota are exceeded", func() {
				shoot.Spec.Provider.Workers[0].Maximum = 2
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should fail because other shoots exhaust quota limits", func() {
				shoot2 := *shoot.DeepCopy()
				shoot2.Name = "test-shoot-2"
				Expect(coreInformerFactory.Core().InternalVersion().Shoots().Informer().GetStore().Add(&shoot2)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should fail because shoot with 2 workers exhaust quota limits", func() {
				shoot.Spec.Provider.Workers = workersBase2
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should pass because can update non worker property although quota is exceeded", func() {
				oldShoot = *shoot.DeepCopy()
				quotaProject.Spec.Metrics[core.QuotaMetricCPU] = resource.MustParse("1")
				Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())

				shoot.Spec.Kubernetes.Version = "1.1.1"
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("machine type in cloud profile defines storage", func() {
				It("should pass because quota is large enough", func() {
					shoot2 := *shoot.DeepCopy()
					shoot2.Spec.Provider.Workers[0].Machine.Type = machineTypeName2
					shoot2.Spec.Provider.Workers[0].Volume.VolumeSize = "19Gi"

					quotaProject.Spec.Metrics[core.QuotaMetricStorageStandard] = resource.MustParse("20Gi")

					attrs := admission.NewAttributesRecord(&shoot2, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should fail because quota is not large enough", func() {
					shoot2 := *shoot.DeepCopy()
					shoot2.Spec.Provider.Workers[0].Machine.Type = machineTypeName2
					shoot2.Spec.Provider.Workers[0].Volume.VolumeSize = "21Gi"

					quotaProject.Spec.Metrics[core.QuotaMetricStorageStandard] = resource.MustParse("20Gi")

					attrs := admission.NewAttributesRecord(&shoot2, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("quota limits exceeded"))
				})
			})
		})

		Context("tests for Quota validation corner cases", func() {
			It("should pass because shoot is intended to get deleted", func() {
				var now metav1.Time
				now.Time = time.Now()
				shoot.DeletionTimestamp = &now

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass because shoots secret binding has no quotas referenced", func() {
				secretBinding.Quotas = make([]corev1.ObjectReference, 0)
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass shoots secret binding having quota with no metrics", func() {
				emptyQuotaName := "empty-quota"
				emptyQuota := core.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
					Spec: core.QuotaSpec{
						ClusterLifetimeDays: &quotaProjectLifetime,
						Scope: corev1.ObjectReference{
							APIVersion: "core.gardener.cloud/v1beta1",
							Kind:       "Project",
						},
						Metrics: corev1.ResourceList{},
					},
				}
				secretBinding.Quotas = []corev1.ObjectReference{
					{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
				}

				Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&emptyQuota)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("tests for extending the lifetime of a Shoot", func() {
			BeforeEach(func() {
				oldShoot = *shoot.DeepCopy()
			})

			It("should pass because no quota prescribe a clusterLifetime", func() {
				quotaProject.Spec.ClusterLifetimeDays = nil
				quotaSecret.Spec.ClusterLifetimeDays = nil
				Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())
				Expect(coreInformerFactory.Core().InternalVersion().Quotas().Informer().GetStore().Add(&quotaSecret)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass as shoot expiration time can be extended", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, "2018-01-02T00:00:00+00:00") // plus 1 day compared to time.Now()
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				now, err := time.Parse(time.RFC3339, "2018-01-01T00:00:00+00:00")
				Expect(err).NotTo(HaveOccurred())
				timeOps.EXPECT().Now().Return(now)

				err = admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail as shoots expiration time can’t be extended, because requested time higher then the minimum .spec.clusterLifetimeDays among the quotas", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, "2018-01-05T00:00:00+00:00") // plus 4 days compared to time.Now()
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				now, err := time.Parse(time.RFC3339, "2018-01-01T00:00:00+00:00")
				Expect(err).NotTo(HaveOccurred())
				timeOps.EXPECT().Now().Return(now)

				err = admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should fail as shoots expiration time can’t be extended, because requested time higher then the maximum .spec.clusterLifetimeDays among the quotas", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, "2018-01-09T00:00:00+00:00") // plus 8 days compared to time.Now()
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				now, err := time.Parse(time.RFC3339, "2018-01-01T00:00:00+00:00")
				Expect(err).NotTo(HaveOccurred())
				timeOps.EXPECT().Now().Return(now)

				err = admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
