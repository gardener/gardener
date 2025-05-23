// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Associations", func() {
	var (
		ctx        context.Context
		fakeClient client.Client

		namespace = "some-namespace"

		quota                  *gardencorev1beta1.Quota
		shoot                  *gardencorev1beta1.Shoot
		backupbucket           *gardencorev1beta1.BackupBucket
		secretBinding          *gardencorev1beta1.SecretBinding
		credentialsBinding     *securityv1alpha1.CredentialsBinding
		controllerinstallation *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithIndex(&gardencorev1beta1.BackupBucket{}, core.BackupBucketSeedName, indexer.BackupBucketSeedNameIndexerFunc).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.SeedRefName, indexer.ControllerInstallationSeedRefNameIndexerFunc).
			Build()

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: namespace,
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secretbinding",
				Namespace: namespace,
			},
		}

		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "credentialsbinding",
				Namespace: namespace,
			},
		}
	})

	DescribeTable("#DetermineShootsAssociatedTo",
		func(obj client.Object, mutateFunc func(shoot *gardencorev1beta1.Shoot, obj client.Object), errorMatcher gomegatypes.GomegaMatcher) {
			mutateFunc(shoot, obj)
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			shoots, err := DetermineShootsAssociatedTo(ctx, fakeClient, obj)
			Expect(err).To(errorMatcher)

			if err == nil {
				Expect(shoots).To(HaveLen(1))
				Expect(shoots).To(ConsistOf(shoot.Namespace + "/" + shoot.Name))
			} else {
				Expect(shoots).To(BeEmpty())
			}
		},

		Entry("should return shoots associated to cloudprofile by cloudprofilename",
			&gardencorev1beta1.CloudProfile{ObjectMeta: metav1.ObjectMeta{Name: "cloudprofile"}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.CloudProfileName = ptr.To(obj.GetName())
			}, BeNil()),
		Entry("should return shoots associated to cloudprofile by cloudprofile reference",
			&gardencorev1beta1.CloudProfile{ObjectMeta: metav1.ObjectMeta{Name: "cloudprofile"}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.CloudProfileName = nil
				s.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{Kind: "CloudProfile", Name: obj.GetName()}
			}, BeNil()),
		Entry("should return shoots associated to namespacedcloudprofile by cloudprofile reference",
			&gardencorev1beta1.NamespacedCloudProfile{ObjectMeta: metav1.ObjectMeta{Name: "namespacedcloudprofile"}, Spec: gardencorev1beta1.NamespacedCloudProfileSpec{Parent: gardencorev1beta1.CloudProfileReference{Kind: "CloudProfile", Name: "cloudprofile"}}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.CloudProfileName = nil
				s.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{Kind: "NamespacedCloudProfile", Name: obj.GetName()}
			}, BeNil()),
		Entry("should return shoots associated to seed",
			&gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed"}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.SeedName = ptr.To(obj.GetName())
			}, BeNil()),
		Entry("should return shoots associated to secretbinding",
			&gardencorev1beta1.SecretBinding{ObjectMeta: metav1.ObjectMeta{Name: "secretbinding", Namespace: namespace}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.SecretBindingName = ptr.To(obj.GetName())
			}, BeNil()),
		Entry("should return shoots associated to credentialsbinding",
			&securityv1alpha1.CredentialsBinding{ObjectMeta: metav1.ObjectMeta{Name: "credentialsbinding", Namespace: namespace}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.CredentialsBindingName = ptr.To(obj.GetName())
			}, BeNil()),
		Entry("should return shoots associated to exposureclass",
			&gardencorev1beta1.ExposureClass{ObjectMeta: metav1.ObjectMeta{Name: "exposureclass"}}, func(s *gardencorev1beta1.Shoot, obj client.Object) {
				s.Spec.ExposureClassName = ptr.To(obj.GetName())
			}, BeNil()),
		Entry("should return error if the object is of not supported type",
			&gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "backupbucket"}}, func(_ *gardencorev1beta1.Shoot, _ client.Object) {}, HaveOccurred()),
	)

	Describe("#DetermineSecretBindingAssociations", func() {
		It("should return secretBinding associated to quota", func() {
			quota = &gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quota",
					Namespace: namespace,
				},
			}

			secretBinding.Quotas = []corev1.ObjectReference{{Name: quota.Name, Namespace: quota.Namespace}}
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			secretBindings, err := DetermineSecretBindingAssociations(ctx, fakeClient, quota)
			Expect(err).ToNot(HaveOccurred())
			Expect(secretBindings).To(HaveLen(1))
			Expect(secretBindings).To(ConsistOf(secretBinding.Namespace + "/" + secretBinding.Name))
		})
	})

	Describe("#DetermineCredentialsBindingAssociations", func() {
		It("should return credentialsBinding associated to quota", func() {
			quota = &gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quota",
					Namespace: namespace,
				},
			}

			credentialsBinding.Quotas = []corev1.ObjectReference{{Name: quota.Name, Namespace: quota.Namespace}}
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			credentialsBindings, err := DetermineCredentialsBindingAssociations(ctx, fakeClient, quota)
			Expect(err).ToNot(HaveOccurred())
			Expect(credentialsBindings).To(HaveLen(1))
			Expect(credentialsBindings).To(ConsistOf(credentialsBinding.Namespace + "/" + credentialsBinding.Name))
		})
	})

	Describe("#DetermineBackupBucketAssociations", func() {
		It("should return backupbucket associated to seed", func() {
			backupbucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: "backupbucket",
				},
				Spec: gardencorev1beta1.BackupBucketSpec{
					SeedName: ptr.To("test"),
				},
			}

			Expect(fakeClient.Create(ctx, backupbucket)).To(Succeed())

			backupbuckets, err := DetermineBackupBucketAssociations(ctx, fakeClient, "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(backupbuckets).To(HaveLen(1))
			Expect(backupbuckets).To(ConsistOf(backupbucket.Name))
		})
	})

	Describe("#DetermineControllerInstallationAssociations", func() {
		It("should return controllerinstallation associated to seed", func() {
			controllerinstallation = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controllerinstallation",
				},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: corev1.ObjectReference{Name: "test"},
				},
			}

			Expect(fakeClient.Create(ctx, controllerinstallation)).To(Succeed())

			controllerinstallations, err := DetermineControllerInstallationAssociations(ctx, fakeClient, "test")
			Expect(err).ToNot(HaveOccurred())
			Expect(controllerinstallations).To(HaveLen(1))
			Expect(controllerinstallations).To(ConsistOf(controllerinstallation.Name))
		})
	})
})
