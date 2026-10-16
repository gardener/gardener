// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/cmd/gardener-operator/app"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Migration", func() {
	Describe("#DeleteStaleShootAccessSecrets", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			fakeClient client.Client
		)

		accessSecret := func(name, serviceAccountName, serviceAccountNamespace, class string) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
						resourcesv1alpha1.ResourceManagerClass:   class,
					},
					Annotations: map[string]string{
						resourcesv1alpha1.ServiceAccountName:      serviceAccountName,
						resourcesv1alpha1.ServiceAccountNamespace: serviceAccountNamespace,
					},
				},
			}
		}

		staleSecret := func(serviceAccount string) *corev1.Secret {
			return accessSecret(gardenerutils.SecretNamePrefixShootAccess+serviceAccount, serviceAccount, metav1.NamespaceSystem, resourcesv1alpha1.ResourceManagerClassShoot)
		}

		replacementSecret := func(serviceAccount string) *corev1.Secret {
			secret := accessSecret(gardenerutils.SecretNamePrefixGardenAccess+serviceAccount, serviceAccount, metav1.NamespaceSystem, resourcesv1alpha1.ResourceManagerClassGarden)
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, "2026-10-07T00:00:00Z")
			return secret
		}

		exists := func(secret *corev1.Secret) bool {
			err := fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})
			if apierrors.IsNotFound(err) {
				return false
			}
			Expect(err).NotTo(HaveOccurred())
			return true
		}

		prepare := func(objs ...client.Object) {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).WithObjects(objs...).Build()
		}

		It("should succeed when there are no secrets at all", func() {
			prepare()
			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
		})

		It("should delete the stale secret when a valid replacement exists", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeFalse())
			Expect(exists(replacement)).To(BeTrue())
		})

		It("should not delete the stale secret when no replacement exists", func() {
			stale := staleSecret("foo")
			prepare(stale)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should not delete the stale secret when the replacement is not garden-class", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			replacement.Labels[resourcesv1alpha1.ResourceManagerClass] = resourcesv1alpha1.ResourceManagerClassShoot
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should not delete the stale secret when the replacement is not a token-requestor secret", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			delete(replacement.Labels, resourcesv1alpha1.ResourceManagerPurpose)
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should not delete the stale secret when the replacement targets a different service account name", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			replacement.Annotations[resourcesv1alpha1.ServiceAccountName] = "other"
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should not delete the stale secret when the replacement targets a different service account namespace", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			replacement.Annotations[resourcesv1alpha1.ServiceAccountNamespace] = "other"
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should not delete the stale secret when the replacement has not been reconciled yet (no renew timestamp)", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			delete(replacement.Annotations, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp)
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should leave genuine shoot-class secrets without a garden replacement untouched", func() {
			stale := staleSecret("foo")
			prepare(stale, replacementSecret("bar"))

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeTrue())
		})

		It("should be idempotent", func() {
			stale := staleSecret("foo")
			replacement := replacementSecret("foo")
			prepare(stale, replacement)

			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(DeleteStaleShootAccessSecrets(ctx, fakeClient, log)).To(Succeed())
			Expect(exists(stale)).To(BeFalse())
			Expect(exists(replacement)).To(BeTrue())
		})
	})
})
