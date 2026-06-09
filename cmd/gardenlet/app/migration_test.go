// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/cmd/gardenlet/app"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("cleanupHashVersioningSecrets", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	It("should do nothing when there are no shoot namespaces", func() {
		Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())
	})

	It("should not error when shoot namespaces exist but the secret is absent", func() {
		ns := shootNamespace("shoot--foo--bar")
		Expect(fakeClient.Create(ctx, ns)).To(Succeed())

		Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())
	})

	It("should delete the secret in a shoot namespace", func() {
		ns := shootNamespace("shoot--foo--bar")
		Expect(fakeClient.Create(ctx, ns)).To(Succeed())

		secret := oscHashSecret("shoot--foo--bar")
		Expect(fakeClient.Create(ctx, secret)).To(Succeed())

		Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(MatchError(ContainSubstring("not found")))
	})

	It("should delete secrets across multiple shoot namespaces", func() {
		for _, name := range []string{"shoot--p--a", "shoot--p--b", "shoot--p--c"} {
			Expect(fakeClient.Create(ctx, shootNamespace(name))).To(Succeed())
			Expect(fakeClient.Create(ctx, oscHashSecret(name))).To(Succeed())
		}

		Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

		for _, name := range []string{"shoot--p--a", "shoot--p--b", "shoot--p--c"} {
			secret := oscHashSecret(name)
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())
		}
	})

	It("should handle a mix of namespaces with and without the secret", func() {
		for _, name := range []string{"shoot--p--a", "shoot--p--b"} {
			Expect(fakeClient.Create(ctx, shootNamespace(name))).To(Succeed())
		}
		Expect(fakeClient.Create(ctx, oscHashSecret("shoot--p--a"))).To(Succeed())

		Expect(CleanupHashVersioningSecrets(ctx, fakeClient)).To(Succeed())

		secretA := oscHashSecret("shoot--p--a")
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretA), secretA)).To(BeNotFoundError())
	})
})

func shootNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
		},
	}
}

func oscHashSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatingsystemconfig.WorkerPoolHashesSecretName,
			Namespace: namespace,
		},
	}
}
