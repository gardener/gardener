// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerremoval_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/global/finalizerremoval"
)

var _ = Describe("finalizerremoval", func() {
	Describe("#Admit", func() {
		var (
			ctx                       context.Context
			admissionHandler          *FinalizerRemoval
			gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory

			finalizers []string

			namespace              = "default"
			secretBindingName      = "binding-1"
			credentialsBindingName = "credentials-binding-1"
			shootName              = "shoot-1"

			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			ctx = context.Background()
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			finalizers = []string{core.GardenerName}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					CredentialsBindingName: ptr.To(credentialsBindingName),
					SecretBindingName:      ptr.To(secretBindingName),
				},
			}

			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(gardenCoreInformerFactory)
		})

		Context("SecretBinding", func() {
			var coreSecretBinding *core.SecretBinding

			BeforeEach(func() {
				coreSecretBinding = &core.SecretBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:       secretBindingName,
						Namespace:  namespace,
						Finalizers: finalizers,
					},
				}
			})

			It("should admit the removal because object is not used by any shoot", func() {
				attrs := admission.NewAttributesRecord(&core.SecretBinding{}, coreSecretBinding, core.Kind("SecretBinding").WithVersion("version"), "", coreSecretBinding.Name, core.Resource("SecretBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
			})

			It("should admit the removal because finalizer is irrelevant", func() {
				newSecretBinding := coreSecretBinding.DeepCopy()
				coreSecretBinding.Finalizers = append(coreSecretBinding.Finalizers, "irrelevant-finalizer")

				attrs := admission.NewAttributesRecord(newSecretBinding, coreSecretBinding, core.Kind("SecretBinding").WithVersion("version"), "", coreSecretBinding.Name, core.Resource("SecretBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
			})

			It("should reject the removal because object is not used by any shoot", func() {
				newSecretBinding := coreSecretBinding.DeepCopy()
				newSecretBinding.Finalizers = nil

				secondShoot := shoot.DeepCopy()
				secondShoot.Name = shootName + "-2"
				secondShoot.Spec.SecretBindingName = ptr.To(secretBindingName + "-2")

				Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(secondShoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(newSecretBinding, coreSecretBinding, core.Kind("SecretBinding").WithVersion("version"), "", coreSecretBinding.Name, core.Resource("SecretBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(MatchError(ContainSubstring("finalizer must not be removed")))
			})
		})

		Context("CredentialsBinding", func() {
			var coreCredentialsBinding *security.CredentialsBinding

			BeforeEach(func() {
				coreCredentialsBinding = &security.CredentialsBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:       credentialsBindingName,
						Namespace:  namespace,
						Finalizers: finalizers,
					},
				}
			})

			It("should admit the removal because object is not used by any shoot", func() {
				attrs := admission.NewAttributesRecord(&security.CredentialsBinding{}, coreCredentialsBinding, security.Kind("CredentialsBinding").WithVersion("version"), "", coreCredentialsBinding.Name, security.Resource("CredentialsBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
			})

			It("should admit the removal because finalizer is irrelevant", func() {
				newCredentialsBinding := coreCredentialsBinding.DeepCopy()
				coreCredentialsBinding.Finalizers = append(coreCredentialsBinding.Finalizers, "irrelevant-finalizer")

				attrs := admission.NewAttributesRecord(newCredentialsBinding, coreCredentialsBinding, security.Kind("CredentialsBinding").WithVersion("version"), "", coreCredentialsBinding.Name, security.Resource("CredentialsBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).NotTo(HaveOccurred())
			})

			It("should reject the removal because object is not used by any shoot", func() {
				newCredentialsBinding := coreCredentialsBinding.DeepCopy()
				newCredentialsBinding.Finalizers = nil

				secondShoot := shoot.DeepCopy()
				secondShoot.Name = shootName + "-2"
				secondShoot.Spec.CredentialsBindingName = ptr.To(secretBindingName + "-2")

				Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(shoot)).To(Succeed())
				Expect(gardenCoreInformerFactory.Core().V1beta1().Shoots().Informer().GetStore().Add(secondShoot)).To(Succeed())

				attrs := admission.NewAttributesRecord(newCredentialsBinding, coreCredentialsBinding, security.Kind("CredentialsBinding").WithVersion("version"), "", coreCredentialsBinding.Name, security.Resource("CredentialsBinding").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(MatchError(ContainSubstring("finalizer must not be removed")))
			})
		})

		Context("shoot", func() {
			var coreShoot *core.Shoot

			BeforeEach(func() {
				coreShoot = &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
					},
					Status: core.ShootStatus{
						TechnicalID: "some-id",
						LastOperation: &core.LastOperation{
							Type:     core.LastOperationTypeReconcile,
							State:    core.LastOperationStateSucceeded,
							Progress: 100,
						},
					},
				}
			})

			It("should allow the removal because finalizer is irrelevant", func() {
				newShoot := coreShoot.DeepCopy()
				coreShoot.Finalizers = append(coreShoot.Finalizers, "irrelevant-finalizer")

				attrs := admission.NewAttributesRecord(newShoot, coreShoot, security.Kind("Shoot").WithVersion("version"), "", coreShoot.Name, security.Resource("Shoot").WithVersion("version"), "", admission.Delete, &metav1.DeleteOptions{}, false, nil)

				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should admit the removal if the shoot deletion succeeded ", func() {
				newShoot := coreShoot.DeepCopy()
				newShoot.Finalizers = nil
				newShoot.Status.LastOperation.Type = core.LastOperationTypeDelete

				attrs := admission.NewAttributesRecord(newShoot, coreShoot, core.Kind("Shoot").WithVersion("version"), coreShoot.Namespace, coreShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should reject the removal if the shoot has not yet been deleted successfully", func() {
				newShoot := coreShoot.DeepCopy()
				newShoot.Finalizers = nil

				attrs := admission.NewAttributesRecord(newShoot, coreShoot, core.Kind("Shoot").WithVersion("version"), coreShoot.Namespace, coreShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(MatchError(ContainSubstring("shoot deletion has not completed successfully yet")))
			})

			It("should admit the removal if the shoot has not yet a last operation", func() {
				newShoot := coreShoot.DeepCopy()
				newShoot.Finalizers = nil
				newShoot.Status.LastOperation = nil

				attrs := admission.NewAttributesRecord(newShoot, coreShoot, core.Kind("Shoot").WithVersion("version"), coreShoot.Namespace, coreShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})

			It("should admit the removal if the shoot has not yet a technical id", func() {
				newShoot := coreShoot.DeepCopy()
				newShoot.Finalizers = nil
				newShoot.Status.TechnicalID = ""

				attrs := admission.NewAttributesRecord(newShoot, coreShoot, core.Kind("Shoot").WithVersion("version"), coreShoot.Namespace, coreShoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)
				Expect(admissionHandler.Admit(ctx, attrs, nil)).To(Succeed())
			})
		})
	})
})
