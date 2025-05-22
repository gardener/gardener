// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/seed/validator"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler        *ValidateSeed
			coreInformerFactory     gardencoreinformers.SharedInformerFactory
			securityInformerFactory gardensecurityinformers.SharedInformerFactory
			backupBucket            gardencorev1beta1.BackupBucket
			seed                    core.Seed
			shoot                   gardencorev1beta1.Shoot

			backupBucketName     = "backupbucket"
			seedName             = "seed"
			namespaceName        = "garden-my-project"
			workloadIdentityName = "workload-identity"
			providerType         = "provider"

			seedBase = core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}
			shootBase = gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespaceName,
				},
				Spec: gardencorev1beta1.ShootSpec{
					CloudProfileName:  ptr.To("profile"),
					Region:            "europe",
					SecretBindingName: ptr.To("my-secret"),
					SeedName:          &seedName,
				},
			}

			backupBucketBase = gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: backupBucketName,
				},
			}
		)

		BeforeEach(func() {
			backupBucket = backupBucketBase
			seed = seedBase
			shoot = *shootBase.DeepCopy()

			var err error
			admissionHandler, err = New()
			Expect(err).ToNot(HaveOccurred())

			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			securityInformerFactory = gardensecurityinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
			admissionHandler.SetSecurityInformerFactory(securityInformerFactory)
		})

		Context("Seed Update", func() {
			var oldSeed, newSeed *core.Seed

			Context("Zones", func() {
				BeforeEach(func() {
					oldSeed = seedBase.DeepCopy()
					newSeed = seedBase.DeepCopy()

					oldSeed.Spec.Provider.Zones = []string{"1", "2"}
					newSeed.Spec.Provider.Zones = []string{"2"}
				})

				It("should allow zone removal when there are no shoots", func() {
					attrs := admission.NewAttributesRecord(newSeed, oldSeed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should forbid zone removal when there are shoots", func() {
					Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
					attrs := admission.NewAttributesRecord(newSeed, oldSeed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(BeForbiddenError())
				})
			})

			Context("Backup provider with WorkloadIdentity credentials", func() {
				BeforeEach(func() {
					oldSeed = seedBase.DeepCopy()
					oldSeed.Spec.Backup = &core.Backup{
						Provider: providerType,
						CredentialsRef: &corev1.ObjectReference{
							APIVersion: "security.gardener.cloud/v1alpha1",
							Kind:       "WorkloadIdentity",
							Namespace:  namespaceName,
							Name:       workloadIdentityName,
						},
					}
					newSeed = oldSeed.DeepCopy()

					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						ObjectMeta: metav1.ObjectMeta{
							Name:      workloadIdentityName,
							Namespace: namespaceName,
						},
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: providerType,
							},
						},
					}
					Expect(securityInformerFactory.Security().V1alpha1().WorkloadIdentities().Informer().GetStore().Add(workloadIdentity)).To(Succeed())
				})

				It("should allow WorkloadIdentity with provider same as Seed backup provider to be used as backup credentials", func() {
					attrs := admission.NewAttributesRecord(newSeed, oldSeed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
				})

				It("should forbid WorkloadIdentity of provider other than Seed backup provider to be used as backup credentials", func() {
					newSeed.Spec.Backup.Provider = "anotherProvider"
					attrs := admission.NewAttributesRecord(newSeed, oldSeed, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).To(BeForbiddenError())
					Expect(err).To(MatchError(ContainSubstring("seed using backup of type \"anotherProvider\" cannot use WorkloadIdentity of type \"provider\"")))
				})
			})
		})

		// The verification of protection is independent of the Cloud Provider (being checked before).
		Context("Seed deletion", func() {
			BeforeEach(func() {
				shoot.Spec.SeedName = &seedName
			})

			It("should disallow seed deletion because it still hosts shoot clusters", func() {
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err).To(BeForbiddenError())
			})

			It("should allow seed deletion even though it is still referenced by a backupbucket (will be cleaned up during Seed reconciliation)", func() {
				backupBucket.Spec.SeedName = &seedName
				Expect(coreInformerFactory.Core().V1beta1().BackupBuckets().Informer().GetStore().Add(&backupBucket)).To(Succeed())
				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should disallow seed deletion because shoot migration is yet not finished", func() {
				shoot.Spec.SeedName = ptr.To(seedName + "-1")
				shoot.Status.SeedName = &seedName

				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
				Expect(err).To(BeForbiddenError())
			})

			It("should allow deletion of empty seed", func() {
				shoot.Spec.SeedName = ptr.To(seedName + "-1")
				Expect(coreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(&shoot)).To(Succeed())
				attrs := admission.NewAttributesRecord(&seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Seed Creation", func() {
			var seed *core.Seed

			BeforeEach(func() {
				seed = seedBase.DeepCopy()
				seed.Spec.Backup = &core.Backup{
					Provider: providerType,
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  namespaceName,
						Name:       workloadIdentityName,
					},
				}

				workloadIdentity := &securityv1alpha1.WorkloadIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workloadIdentityName,
						Namespace: namespaceName,
					},
					Spec: securityv1alpha1.WorkloadIdentitySpec{
						TargetSystem: securityv1alpha1.TargetSystem{
							Type: providerType,
						},
					},
				}
				Expect(securityInformerFactory.Security().V1alpha1().WorkloadIdentities().Informer().GetStore().Add(workloadIdentity)).To(Succeed())
			})

			It("should allow WorkloadIdentity with provider same as Seed backup provider to be used as backup credentials", func() {
				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
			})

			It("should forbid WorkloadIdentity of provider other than Seed backup provider to be used as backup credentials", func() {
				seed.Spec.Backup.Provider = "anotherProvider"
				attrs := admission.NewAttributesRecord(seed, nil, core.Kind("Seed").WithVersion("version"), "", seed.Name, core.Resource("seeds").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("seed using backup of type \"anotherProvider\" cannot use WorkloadIdentity of type \"provider\"")))

			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("SeedValidator"))
		})
	})

	Describe("#New", func() {
		It("should handle only CREATE, UPDATE, and DELETE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).To(BeFalse())
			Expect(dr.Handles(admission.Delete)).To(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ShootLister or SeedLister is set", func() {
			dr, _ := New()
			dr.SetSecurityInformerFactory(gardensecurityinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("missing seed lister"))
		})

		It("should return error if no WorkloadIdentityLister is set", func() {
			dr, _ := New()
			dr.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("missing WorkloadIdentity lister"))
		})

		It("should not return error if ShootLister, SeedLister, and WorkloadIdentityLister are set", func() {
			dr, _ := New()
			dr.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			dr.SetSecurityInformerFactory(gardensecurityinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})

})
