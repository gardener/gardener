// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Cleanup", func() {
	const (
		testIdentity = "test"
		namespace    = "shoot--foo--bar"
	)

	var (
		ctx = context.TODO()

		m          *manager
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		mgr, err := New(ctx, logr.Discard(), clock.RealClock{}, fakeClient, namespace, testIdentity, Config{})
		Expect(err).NotTo(HaveOccurred())
		m = mgr.(*manager)
	})

	secretList := func(identity string) []*corev1.Secret {
		secrets := []*corev1.Secret{
			{ObjectMeta: metav1.ObjectMeta{Name: "secret1", Labels: map[string]string{"name": "first"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret2", Labels: map[string]string{"name": "first"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret3", Labels: map[string]string{"name": "first-bundle", "bundle-for": "first"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret4", Labels: map[string]string{"name": "second"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret5", Labels: map[string]string{"name": "third"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret6", Labels: map[string]string{"name": "third"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret7", Labels: map[string]string{"name": "fourth"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "secret8", Labels: map[string]string{"name": "fifth"}}},
		}

		for i := range secrets {
			secrets[i].Namespace = namespace
			secrets[i].Labels["managed-by"] = "secrets-manager"
			secrets[i].Labels["manager-identity"] = identity
		}

		return secrets
	}

	Describe("#Cleanup", func() {
		It("should do nothing because there are no secrets", func() {
			Expect(m.Cleanup(ctx)).To(Succeed())
		})

		It("should delete all secrets not part of the internal store", func() {
			secrets := secretList(testIdentity)
			for i := range secrets {
				Expect(fakeClient.Create(ctx, secrets[i])).To(Succeed())
			}

			Expect(m.addToStore("first", secrets[0], current)).To(Succeed())
			Expect(m.addToStore("first", secrets[1], old)).To(Succeed())
			Expect(m.addToStore("first", secrets[2], bundle)).To(Succeed())
			Expect(m.addToStore("second", secrets[3], current)).To(Succeed())
			Expect(m.addToStore("third", secrets[4], current)).To(Succeed())
			Expect(m.addToStore("third", secrets[5], old)).To(Succeed())

			Expect(m.Cleanup(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[0]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[1]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[2]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[3]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[4]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[5]), &corev1.Secret{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[6]), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[7]), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should not touch secrets from other manager instance", func() {
			secrets := secretList(testIdentity + "other")
			for i := range secrets {
				Expect(fakeClient.Create(ctx, secrets[i])).To(Succeed())
			}

			Expect(m.Cleanup(ctx)).To(Succeed())

			for i := range secrets {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secrets[i]), &corev1.Secret{})).To(Succeed())
			}
		})
	})
})
