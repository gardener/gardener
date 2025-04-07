// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quotavalidator_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	mocktime "github.com/gardener/gardener/pkg/utils/time/mock"
	. "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
)

var _ = Describe("quotavalidator", func() {
	Describe("#Admit", func() {
		var (
			ctrl *gomock.Controller

			admissionHandler        *QuotaValidator
			coreInformerFactory     gardencoreinformers.SharedInformerFactory
			securityInformerFactory securityinformers.SharedInformerFactory
			timeOps                 *mocktime.MockOps
			shoot                   core.Shoot
			oldShoot                core.Shoot
			secretBinding           gardencorev1beta1.SecretBinding
			credentialsBinding      securityv1alpha1.CredentialsBinding
			quotaProject            gardencorev1beta1.Quota
			quotaSecret             gardencorev1beta1.Quota
			cloudProfile            gardencorev1beta1.CloudProfile
			namespace               = "test"
			trialNamespace          = "trial"
			machineTypeName         = "n1-standard-2"
			machineTypeName2        = "machtype2"
			volumeTypeName          = "pd-standard"

			cloudProfileBase = gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineTypes: []gardencorev1beta1.MachineType{
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
							Storage: &gardencorev1beta1.MachineTypeStorage{
								Class: gardencorev1beta1.VolumeClassStandard,
							},
						},
					},
					VolumeTypes: []gardencorev1beta1.VolumeType{
						{
							Name:  volumeTypeName,
							Class: "standard",
						},
					},
				},
			}

			quotaProjectLifetime int32 = 1
			quotaProjectBase           = gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "project-quota",
				},
				Spec: gardencorev1beta1.QuotaSpec{
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
			quotaSecretBase           = gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: trialNamespace,
					Name:      "secret-quota",
				},
				Spec: gardencorev1beta1.QuotaSpec{
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

			secretBindingBase = gardencorev1beta1.SecretBinding{
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
			credentialsBindingBase = securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-credentials-binding",
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
					CloudProfileName:  ptr.To("profile"),
					SecretBindingName: ptr.To("test-binding"),
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

			versionedShootBase = gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					CloudProfileName:  ptr.To("profile"),
					SecretBindingName: ptr.To("test-binding"),
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "test-worker-1",
								Machine: gardencorev1beta1.Machine{
									Type: machineTypeName,
								},
								Maximum: 1,
								Minimum: 1,
								Volume: &gardencorev1beta1.Volume{
									VolumeSize: "30Gi",
									Type:       &volumeTypeName,
								},
							},
						},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.0.1",
					},
					Addons: &gardencorev1beta1.Addons{
						NginxIngress: &gardencorev1beta1.NginxIngress{
							Addon: gardencorev1beta1.Addon{
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
			credentialsBinding = *credentialsBindingBase.DeepCopy()
			quotaProject = *quotaProjectBase.DeepCopy()
			quotaSecret = *quotaSecretBase.DeepCopy()

			ctrl = gomock.NewController(GinkgoT())
			timeOps = mocktime.NewMockOps(ctrl)

			admissionHandler, _ = New(timeOps)
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&quotaSecret)).To(Succeed())
			Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBindingBase)).To(Succeed())

			securityInformerFactory = securityinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetSecurityInformerFactory(securityInformerFactory)

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
				shoot2 := *versionedShootBase.DeepCopy()
				shoot2.Name = "test-shoot-2"

				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot2)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err2 := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err2).To(HaveOccurred())
			})

			It("should fail because other shoots exhaust quota limits via credentials binding", func() {
				shoot2 := *versionedShootBase.DeepCopy()
				shoot2.Name = "test-shoot-2"
				shoot2.Spec.SecretBindingName = nil
				shoot2.Spec.CredentialsBindingName = ptr.To(credentialsBinding.Name)

				Expect(securityInformerFactory.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(&credentialsBinding)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot2)).To(Succeed())

				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err2 := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err2).To(HaveOccurred())
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
				Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())

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
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should pass shoots secret binding having quota with no metrics", func() {
				emptyQuotaName := "empty-quota"
				emptyQuota := gardencorev1beta1.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: trialNamespace,
						Name:      emptyQuotaName,
					},
					Spec: gardencorev1beta1.QuotaSpec{
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

				Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&emptyQuota)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(&secretBinding)).To(Succeed())
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
				Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&quotaProject)).To(Succeed())
				Expect(coreInformerFactory.Core().V1beta1().Quotas().Informer().GetStore().Add(&quotaSecret)).To(Succeed())

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

			It("should fail as shoots expiration time can’t be extended, because requested time higher than the minimum .spec.clusterLifetimeDays among the quotas", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, "2018-01-05T00:00:00+00:00") // plus 4 days compared to time.Now()
				attrs := admission.NewAttributesRecord(&shoot, &oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				now, err := time.Parse(time.RFC3339, "2018-01-01T00:00:00+00:00")
				Expect(err).NotTo(HaveOccurred())
				timeOps.EXPECT().Now().Return(now)

				err = admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should fail as shoots expiration time can’t be extended, because requested time higher than the maximum .spec.clusterLifetimeDays among the quotas", func() {
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
