// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package stale_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/stale"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx       = context.TODO()
		fakeClock *testing.FakeClock

		fakeClient client.Client

		projectName            = "foo"
		namespaceName          = "garden-foo"
		secretName             = "secret"
		internalSecretName     = "internal-secret"
		workloadIdentityName   = "workloadidentity"
		secretBindingName      = "secretbinding"
		credentialsBindingName = "credentialsbinding"
		quotaName              = "quotaMeta"

		minimumLifetimeDays     = 5
		staleGracePeriodDays    = 10
		staleExpirationTimeDays = 15
		staleSyncPeriod         = metav1.Duration{Duration: time.Second}

		project            *gardencorev1beta1.Project
		namespace          *corev1.Namespace
		shoot              *gardencorev1beta1.Shoot
		secretBinding      *gardencorev1beta1.SecretBinding
		credentialsBinding *securityv1alpha1.CredentialsBinding
		cfg                controllermanagerconfigv1alpha1.ProjectControllerConfiguration
		request            reconcile.Request

		reconciler reconcile.Reconciler
	)

	BeforeEach(func() {
		fakeClock = testing.NewFakeClock(time.Now())

		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&gardencorev1beta1.Project{}).
			Build()

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: projectName},
			Spec:       gardencorev1beta1.ProjectSpec{Namespace: &namespaceName},
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		}

		Expect(fakeClient.Create(ctx, project)).To(Succeed())
		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: namespaceName},
			Spec:       gardencorev1beta1.ShootSpec{SecretBindingName: ptr.To(secretBindingName)},
		}
		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: secretBindingName},
			SecretRef:  corev1.SecretReference{Namespace: namespaceName, Name: secretName},
			Quotas:     []corev1.ObjectReference{{}, {Namespace: namespaceName, Name: quotaName}},
		}
		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta:     metav1.ObjectMeta{Namespace: namespaceName, Name: credentialsBindingName},
			CredentialsRef: corev1.ObjectReference{Kind: "Secret", APIVersion: "v1", Namespace: namespaceName, Name: secretName},
			Quotas:         []corev1.ObjectReference{{}, {Namespace: namespaceName, Name: quotaName}},
		}
		cfg = controllermanagerconfigv1alpha1.ProjectControllerConfiguration{
			MinimumLifetimeDays:     &minimumLifetimeDays,
			StaleGracePeriodDays:    &staleGracePeriodDays,
			StaleExpirationTimeDays: &staleExpirationTimeDays,
			StaleSyncPeriod:         &staleSyncPeriod,
		}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}}

		reconciler = &Reconciler{Client: fakeClient, Config: cfg, Clock: fakeClock}
	})

	// secretObj creates a corev1.Secret with the specified labels for the fake client.
	secretObj := func(ns, name string, labels map[string]string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
				Labels:    labels,
			},
		}
	}

	// internalSecretObj creates a gardencorev1beta1.InternalSecret.
	internalSecretObj := func(ns, name string, labels map[string]string) *gardencorev1beta1.InternalSecret {
		return &gardencorev1beta1.InternalSecret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
				Labels:    labels,
			},
		}
	}

	// workloadIdentityObj creates a securityv1alpha1.WorkloadIdentity.
	workloadIdentityObj := func(ns, name string, labels map[string]string) *securityv1alpha1.WorkloadIdentity {
		return &securityv1alpha1.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
				Labels:    labels,
			},
		}
	}

	// quotaObj creates a gardencorev1beta1.Quota.
	quotaObj := func(ns, name string) *gardencorev1beta1.Quota {
		return &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      name,
			},
		}
	}

	Describe("#Reconcile", func() {
		Context("early exit", func() {
			It("should do nothing because the project has no namespace", func() {
				project.Spec.Namespace = nil

				_, result := reconciler.Reconcile(ctx, request)
				Expect(result).To(Succeed())
			})
		})

		It("should mark the project as 'not stale' because the namespace has the skip-stale-check annotation", func() {
			fakeClock.SetTime(time.Date(100, 1, 1, 0, 0, 0, 0, time.UTC))
			namespace.Annotations = map[string]string{v1beta1constants.ProjectSkipStaleCheck: "true"}

			Expect(fakeClient.Update(ctx, namespace)).To(Succeed())

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			Expect(project.Status.StaleSinceTimestamp).To(BeNil())
			Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
		})

		It("should mark the project as 'not stale' because it is younger than the configured MinimumLifetimeDays", func() {
			fakeClock.SetTime(time.Date(1, 1, minimumLifetimeDays+1, 0, 0, 0, 0, time.UTC))
			project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}
			Expect(fakeClient.Update(ctx, project)).To(Succeed())

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			Expect(project.Status.StaleSinceTimestamp).To(BeNil())
			Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
		})

		It("should mark the project as 'not stale' because the last activity was before the MinimumLifetimeDays", func() {
			fakeClock.SetTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))
			project.Status.LastActivityTimestamp = &metav1.Time{Time: time.Date(1, 1, minimumLifetimeDays-1, 0, 0, 0, 0, time.UTC)}

			Expect(fakeClient.Status().Update(ctx, project)).To(Succeed())

			_, result := reconciler.Reconcile(ctx, request)
			Expect(result).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			Expect(project.Status.StaleSinceTimestamp).To(BeNil())
			Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
		})

		Context("project older than the configured MinimumLifetimeDays", func() {
			BeforeEach(func() {
				fakeClock.SetTime(time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC))
				project.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)}
			})

			Describe("project should be marked as not stale", func() {
				It("has shoots", func() {
					shootInNs := &gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: namespaceName},
					}

					Expect(fakeClient.Create(ctx, shootInNs)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has backupentries", func() {
					backupEntry := &gardencorev1beta1.BackupEntry{
						ObjectMeta: metav1.ObjectMeta{Name: "be1", Namespace: namespaceName},
						Spec:       gardencorev1beta1.BackupEntrySpec{BucketName: "bucket1"},
					}
					Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has secrets referenced by secret binding that are used by shoots in the same namespace", func() {
					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelSecretBindingReference: "true",
					})
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, secretBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, shoot.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has secrets referenced by credentials binding that are used by shoots in the same namespace", func() {
					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					shootWithCB := shoot.DeepCopy()
					shootWithCB.Spec.SecretBindingName = nil
					shootWithCB.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, shootWithCB)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has secrets referenced by secret binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelSecretBindingReference: "true",
					})
					sbInOther := secretBinding.DeepCopy()
					sbInOther.Namespace = otherNamespace
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, sbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has secrets referenced by credentials binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					cbInOther := credentialsBinding.DeepCopy()
					cbInOther.Namespace = otherNamespace
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace
					shootInOther.Spec.SecretBindingName = nil
					shootInOther.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has quotas referenced by secret binding that are used by shoots in the same namespace", func() {
					quota := quotaObj(namespaceName, quotaName)
					Expect(fakeClient.Create(ctx, quota)).To(Succeed())
					Expect(fakeClient.Create(ctx, secretBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, shoot.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has quotas referenced by credentials binding that are used by shoots in the same namespace", func() {
					quota := quotaObj(namespaceName, quotaName)
					shootWithCB := shoot.DeepCopy()
					shootWithCB.Spec.SecretBindingName = nil
					shootWithCB.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)
					Expect(fakeClient.Create(ctx, quota)).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, shootWithCB)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has quotas referenced by secret binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					quota := quotaObj(namespaceName, quotaName)
					sbInOther := secretBinding.DeepCopy()
					sbInOther.Namespace = otherNamespace
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, quota)).To(Succeed())
					Expect(fakeClient.Create(ctx, sbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has quotas referenced by credentials binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					quota := quotaObj(namespaceName, quotaName)
					cbInOther := credentialsBinding.DeepCopy()
					cbInOther.Namespace = otherNamespace
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace
					shootInOther.Spec.SecretBindingName = nil
					shootInOther.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, quota)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has internal secrets referenced by credentials binding that are used by shoots in the same namespace", func() {
					internalSecret := internalSecretObj(namespaceName, internalSecretName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					cbWithInternalSecret := credentialsBinding.DeepCopy()
					cbWithInternalSecret.CredentialsRef = corev1.ObjectReference{Kind: "InternalSecret", APIVersion: "core.gardener.cloud/v1beta1", Namespace: namespaceName, Name: internalSecretName}
					shootWithCB := shoot.DeepCopy()
					shootWithCB.Spec.SecretBindingName = nil
					shootWithCB.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, internalSecret)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbWithInternalSecret)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootWithCB)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has internal secrets referenced by credentials binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					internalSecret := internalSecretObj(namespaceName, internalSecretName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					cbInOther := credentialsBinding.DeepCopy()
					cbInOther.Namespace = otherNamespace
					cbInOther.CredentialsRef = corev1.ObjectReference{Kind: "InternalSecret", APIVersion: "core.gardener.cloud/v1beta1", Namespace: namespaceName, Name: internalSecretName}
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace
					shootInOther.Spec.SecretBindingName = nil
					shootInOther.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, internalSecret)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has workload identities referenced by credentials binding that are used by shoots in the same namespace", func() {
					wi := workloadIdentityObj(namespaceName, workloadIdentityName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					cbWithWI := credentialsBinding.DeepCopy()
					cbWithWI.CredentialsRef = corev1.ObjectReference{Kind: "WorkloadIdentity", APIVersion: "security.gardener.cloud/v1alpha1", Namespace: namespaceName, Name: workloadIdentityName}
					shootWithCB := shoot.DeepCopy()
					shootWithCB.Spec.SecretBindingName = nil
					shootWithCB.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, wi)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbWithWI)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootWithCB)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})

				It("has workload identities referenced by credentials binding that are used by shoots in another namespace", func() {
					otherNamespace := namespaceName + "other"
					otherNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespace}}

					wi := workloadIdentityObj(namespaceName, workloadIdentityName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					cbInOther := credentialsBinding.DeepCopy()
					cbInOther.Namespace = otherNamespace
					cbInOther.CredentialsRef = corev1.ObjectReference{Kind: "WorkloadIdentity", APIVersion: "security.gardener.cloud/v1alpha1", Namespace: namespaceName, Name: workloadIdentityName}
					shootInOther := shoot.DeepCopy()
					shootInOther.Namespace = otherNamespace
					shootInOther.Spec.SecretBindingName = nil
					shootInOther.Spec.CredentialsBindingName = ptr.To(credentialsBindingName)

					Expect(fakeClient.Create(ctx, otherNs)).To(Succeed())
					Expect(fakeClient.Create(ctx, wi)).To(Succeed())
					Expect(fakeClient.Create(ctx, cbInOther)).To(Succeed())
					Expect(fakeClient.Create(ctx, shootInOther)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).To(BeNil())
				})
			})

			Describe("project should be marked as stale", func() {
				It("has secrets that are unused", func() {
					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelSecretBindingReference: "true",
					})
					// SecretBinding references the secret, but no shoot uses this secretBinding
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, secretBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleSinceTimestamp.UTC()).To(Equal(fakeClock.Now().UTC()))
				})

				It("has secrets that are unused (secret binding & credentials binding are nil for shoot)", func() {
					secret := secretObj(namespaceName, secretName, map[string]string{
						v1beta1constants.LabelSecretBindingReference: "true",
					})
					//  SecretBinding and CredentialsBinding reference the secret, but no shoots use them
					Expect(fakeClient.Create(ctx, secret)).To(Succeed())
					Expect(fakeClient.Create(ctx, secretBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleSinceTimestamp.UTC()).To(Equal(fakeClock.Now().UTC()))
				})

				It("has quotas that are unused", func() {
					quota := quotaObj(namespaceName, quotaName)
					// SecretBinding and CredentialsBinding reference the quota, but no shoots use them
					Expect(fakeClient.Create(ctx, quota)).To(Succeed())
					Expect(fakeClient.Create(ctx, secretBinding.DeepCopy())).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleSinceTimestamp.UTC()).To(Equal(fakeClock.Now().UTC()))
				})

				It("has workload identities that are unused", func() {
					wi := workloadIdentityObj(namespaceName, workloadIdentityName, map[string]string{
						v1beta1constants.LabelCredentialsBindingReference: "true",
					})
					// CredentialsBinding references the workload identity, but no shoot uses it
					Expect(fakeClient.Create(ctx, wi)).To(Succeed())
					Expect(fakeClient.Create(ctx, credentialsBinding.DeepCopy())).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleSinceTimestamp.UTC()).To(Equal(fakeClock.Now().UTC()))
				})

				It("it is not used", func() {
					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleSinceTimestamp.UTC()).To(Equal(fakeClock.Now().UTC()))
				})

				It("should not set the auto delete timestamp because stale grace period is not exceeded", func() {
					staleSinceTimestamp := metav1.Time{Time: fakeClock.Now().Add(-24*time.Hour*time.Duration(staleGracePeriodDays) + time.Hour)}
					project.Status.StaleSinceTimestamp = &staleSinceTimestamp

					// Set the stale status on the project
					p := &gardencorev1beta1.Project{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), p)).To(Succeed())
					p.Status.StaleSinceTimestamp = &staleSinceTimestamp
					Expect(fakeClient.Status().Update(ctx, p)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleAutoDeleteTimestamp).To(BeNil())
				})

				It("should set the auto delete timestamp because stale grace period is exceeded", func() {
					var (
						staleSinceTimestamp      = metav1.Time{Time: fakeClock.Now().Add(-24 * time.Hour * time.Duration(staleGracePeriodDays))}
						staleAutoDeleteTimestamp = metav1.Time{Time: staleSinceTimestamp.Add(24 * time.Hour * time.Duration(staleExpirationTimeDays))}
					)
					project.Status.StaleSinceTimestamp = &staleSinceTimestamp

					p := &gardencorev1beta1.Project{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), p)).To(Succeed())
					p.Status.StaleSinceTimestamp = &staleSinceTimestamp
					Expect(fakeClient.Status().Update(ctx, p)).To(Succeed())

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
					Expect(project.Status.StaleSinceTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleAutoDeleteTimestamp).ToNot(BeNil())
					Expect(project.Status.StaleAutoDeleteTimestamp.UTC()).To(Equal(staleAutoDeleteTimestamp.UTC()))
				})

				It("should delete the project if the auto delete timestamp is exceeded", func() {
					var (
						staleSinceTimestamp      = metav1.Time{Time: fakeClock.Now().Add(-24 * time.Hour * 3 * time.Duration(staleExpirationTimeDays))}
						staleAutoDeleteTimestamp = metav1.Time{Time: fakeClock.Now()}
					)

					project.Status.StaleSinceTimestamp = &staleSinceTimestamp
					project.Status.StaleAutoDeleteTimestamp = &staleAutoDeleteTimestamp

					p := &gardencorev1beta1.Project{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), p)).To(Succeed())
					p.Status.StaleSinceTimestamp = &staleSinceTimestamp
					p.Status.StaleAutoDeleteTimestamp = &staleAutoDeleteTimestamp
					Expect(fakeClient.Status().Update(ctx, p)).To(Succeed())

					defer test.WithVar(&gardenerutils.TimeNow, func() time.Time {
						return time.Date(1, 1, minimumLifetimeDays+1, 1, 0, 0, 0, time.UTC)
					})()

					_, result := reconciler.Reconcile(ctx, request)
					Expect(result).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(BeNotFoundError())
				})
			})
		})
	})
})
