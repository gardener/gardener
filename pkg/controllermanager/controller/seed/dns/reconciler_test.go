// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/dns"
)

var _ = Describe("Reconciler", func() {
	const (
		seedName           = "test"
		syncPeriodDuration = 30 * time.Second
	)

	var (
		ctx context.Context
		c   client.Client

		seed   *gardencorev1beta1.Seed
		secret *corev1.Secret

		reconciler *dns.Reconciler
		request    reconcile.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
		}
		request = reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)}

		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			Build()

		reconciler = &dns.Reconciler{
			Client:          c,
			GardenNamespace: "garden",
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "internal-domain",
				Namespace: "garden",
				Annotations: map[string]string{
					"dns.gardener.cloud/provider": "providerType",
					"dns.gardener.cloud/domain":   "internal.example.com",
					"dns.gardener.cloud/zone":     "zone-1",
				},
				Labels: map[string]string{
					"gardener.cloud/role": "internal-domain",
				},
			},
			Data: map[string][]byte{},
		}
	})

	It("should default internal DNS from internal domain secret if not set", func() {
		Expect(c.Create(ctx, seed)).To(Succeed())
		Expect(c.Create(ctx, secret)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())

		updatedSeed := &gardencorev1beta1.Seed{}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
		Expect(updatedSeed).To(Equal(&gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:            seedName,
				ResourceVersion: "2",
			},
			Spec: gardencorev1beta1.SeedSpec{
				DNS: gardencorev1beta1.SeedDNS{
					Internal: &gardencorev1beta1.SeedDNSProviderConf{
						Type:   "providerType",
						Domain: "internal.example.com",
						Zone:   ptr.To("zone-1"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "garden",
							Name:       "internal-domain",
						},
					},
				},
			},
		}))
	})

	It("should not default internal DNS if already set in the seed", func() {
		seed.Spec.DNS = gardencorev1beta1.SeedDNS{
			Internal: &gardencorev1beta1.SeedDNSProviderConf{
				Type:   "custom-provider",
				Domain: "custom.example.com",
				Zone:   ptr.To("custom-zone"),
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "custom-secret",
				},
			},
		}

		Expect(c.Create(ctx, seed)).To(Succeed())
		Expect(c.Create(ctx, secret)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())

		updatedSeed := &gardencorev1beta1.Seed{}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(seed), updatedSeed)).To(Succeed())
		Expect(updatedSeed).To(Equal(&gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:            seedName,
				ResourceVersion: "1",
			},
			Spec: gardencorev1beta1.SeedSpec{
				DNS: gardencorev1beta1.SeedDNS{
					Internal: &gardencorev1beta1.SeedDNSProviderConf{
						Type:   "custom-provider",
						Domain: "custom.example.com",
						Zone:   ptr.To("custom-zone"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  "garden",
							Name:       "custom-secret",
						},
					},
				},
			},
		}))
	})
})
