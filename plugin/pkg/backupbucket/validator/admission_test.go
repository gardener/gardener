// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/plugin/pkg/backupbucket/validator"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler        *ValidateBackupBucket
			securityInformerFactory gardensecurityinformers.SharedInformerFactory
			backupBucket            *gardencore.BackupBucket

			backupBucketName     = "backupbucket"
			namespaceName        = "garden-my-project"
			workloadIdentityName = "workload-identity"
			providerType         = "provider"

			backupBucketBase = gardencore.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: backupBucketName,
				},
			}
		)

		BeforeEach(func() {
			backupBucket = backupBucketBase.DeepCopy()

			var err error
			admissionHandler, err = New()
			Expect(err).ToNot(HaveOccurred())

			admissionHandler.AssignReadyFunc(func() bool { return true })
			securityInformerFactory = gardensecurityinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetSecurityInformerFactory(securityInformerFactory)
		})

		Context("BackupBucket Update", func() {
			var oldBackupBucket, newBackupBucket *gardencore.BackupBucket

			BeforeEach(func() {
				oldBackupBucket = backupBucket.DeepCopy()
				oldBackupBucket.Spec = gardencore.BackupBucketSpec{
					Provider: gardencore.BackupBucketProvider{
						Type: providerType,
					},
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "security.gardener.cloud/v1alpha1",
						Kind:       "WorkloadIdentity",
						Namespace:  namespaceName,
						Name:       workloadIdentityName,
					},
				}
				newBackupBucket = oldBackupBucket.DeepCopy()

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

			It("should allow WorkloadIdentity with provider same as BackupBucket provider to be used as backup credentials", func() {
				attrs := admission.NewAttributesRecord(newBackupBucket, oldBackupBucket, gardencore.Kind("BackupBucket").WithVersion("version"), "", backupBucket.Name, gardencore.Resource("backupbuckets").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
			})

			It("should forbid WorkloadIdentity of provider other than BackupBucket provider to be used as backup credentials", func() {
				newBackupBucket.Spec.Provider.Type = "anotherProvider"
				attrs := admission.NewAttributesRecord(newBackupBucket, oldBackupBucket, gardencore.Kind("BackupBucket").WithVersion("version"), "", backupBucket.Name, gardencore.Resource("backupbuckets").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("BackupBucket using backup of type \"anotherProvider\" cannot use WorkloadIdentity of type \"provider\"")))
			})
		})

		Context("BackupBucket Creation", func() {
			BeforeEach(func() {
				backupBucket = backupBucketBase.DeepCopy()
				backupBucket.Spec = gardencore.BackupBucketSpec{
					Provider: gardencore.BackupBucketProvider{
						Type: providerType,
					},
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

			It("should allow WorkloadIdentity with provider same as BackupBucket provider to be used as backup credentials", func() {
				attrs := admission.NewAttributesRecord(backupBucket, nil, gardencore.Kind("BackupBucket").WithVersion("version"), "", backupBucket.Name, gardencore.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
			})

			It("should forbid WorkloadIdentity of provider other than BackupBucket provider to be used as backup credentials", func() {
				backupBucket.Spec.Provider.Type = "anotherProvider"
				attrs := admission.NewAttributesRecord(backupBucket, nil, gardencore.Kind("BackupBucket").WithVersion("version"), "", backupBucket.Name, gardencore.Resource("backupbuckets").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Validate(context.TODO(), attrs, nil)
				Expect(err).To(BeForbiddenError())
				Expect(err).To(MatchError(ContainSubstring("BackupBucket using backup of type \"anotherProvider\" cannot use WorkloadIdentity of type \"provider\"")))

			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("BackupBucketValidator"))
		})
	})

	Describe("#New", func() {
		It("should handle only CREATE and UPDATE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).To(BeFalse())
			Expect(dr.Handles(admission.Delete)).To(BeFalse())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no WorkloadIdentityLister is set", func() {
			dr, _ := New()

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("missing workloadidentity lister"))
		})

		It("should not return error if WorkloadIdentityLister is set", func() {
			dr, _ := New()
			dr.SetSecurityInformerFactory(gardensecurityinformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})

})
