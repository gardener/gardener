// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("AdvertisedAddresses", func() {
	var (
		botanist       *Botanist
		fakeClient     client.Client
		fakeSeedClient kubernetes.Interface
		ctx            = context.TODO()
		shootNamespace = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shoot--test--test"}}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedClient = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.SeedClientSet = fakeSeedClient
		botanist.Seed = &seedpkg.Seed{}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
		botanist.Shoot = &shootpkg.Shoot{
			ControlPlaneNamespace: shootNamespace.Name,
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#ToAdvertisedAddresses", func() {
		It("returns empty list when shoot is nil", func() {
			botanist.Shoot = nil

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(addresses).To(BeNil())
		})

		It("returns external address", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "external",
				URL:  "https://api.foo.bar",
			}))
		})

		It("returns internal and service-account-issuer addresses", func() {
			botanist.Shoot.InternalClusterDomain = ptr.To("baz.foo")

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
				{
					Name: "service-account-issuer",
					URL:  "https://api.baz.foo",
				},
			}))
		})

		It("returns unmanaged address", func() {
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "unmanaged",
				URL:  "https://bar.foo",
			}))
		})

		It("returns external, internal, service-account-issuer addresses in correct order", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = ptr.To("baz.foo")
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "external",
					URL:  "https://api.foo.bar",
				}, {
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
				{
					Name: "service-account-issuer",
					URL:  "https://api.baz.foo",
				},
			}))
		})

		It("returns external, internal addresses with addition to custom service-account-issuer address", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = ptr.To("baz.foo")
			botanist.Shoot.GetInfo().Spec.Kubernetes = gardencorev1beta1.Kubernetes{
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer: ptr.To("https://foo.bar.example.issuer"),
					},
				},
			}
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "external",
					URL:  "https://api.foo.bar",
				}, {
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
				{
					Name: "service-account-issuer",
					URL:  "https://foo.bar.example.issuer",
				},
			}))
		})

		It("returns external, internal addresses with addition to managed service-account-issuer address", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = ptr.To("baz.foo")
			botanist.Shoot.ServiceAccountIssuerHostname = ptr.To("managed.foo.bar")
			botanist.Garden = &garden.Garden{
				Project: &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-proj",
					},
				},
			}

			botanist.Shoot.GetInfo().ObjectMeta = metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testspace",
				UID:       "some-uid",
				Annotations: map[string]string{
					"authentication.gardener.cloud/issuer": "managed",
				},
			}
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "external",
					URL:  "https://api.foo.bar",
				}, {
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
				{
					Name: "service-account-issuer",
					URL:  "https://managed.foo.bar/projects/some-proj/shoots/some-uid/issuer",
				},
			}))
		})

		It("should return error because shoot wants managed issuer, but issuer hostname is not configured", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = ptr.To("baz.foo")

			botanist.Garden = &garden.Garden{
				Project: &gardencorev1beta1.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-proj",
					},
				},
			}

			botanist.Shoot.GetInfo().ObjectMeta = metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testspace",
				UID:       "some-uid",
				Annotations: map[string]string{
					"authentication.gardener.cloud/issuer": "managed",
				},
			}
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("shoot requires managed issuer, but gardener does not have shoot service account hostname configured"))
			Expect(addresses).To(BeNil())
		})
	})

	Describe("#GetIngressAdvertisedEndpoints", func() {
		It("returns nothing with no ingress resources", func() {
			items, err := botanist.GetIngressAdvertisedEndpoints(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(items).To(BeEmpty())
		})

		It("returns nothing when no ingress is labeled", func() {
			// Resource does not have the expected labels
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-1",
					Namespace: shootNamespace.Name,
				},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{"foo.example.org"},
						},
					},
				},
			}

			Expect(botanist.SeedClientSet.Client().Create(ctx, ingress)).To(Succeed())
			items, err := botanist.GetIngressAdvertisedEndpoints(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(items).To(BeEmpty())
		})

		It("returns valid endpoints from ingress resources", func() {
			ingressA := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-1",
					Namespace: shootNamespace.Name,
					Labels: map[string]string{
						v1beta1constants.LabelShootEndpointAdvertise: "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{"foo.example.org"},
						},
					},
				},
			}
			ingressB := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-2",
					Namespace: shootNamespace.Name,
					Labels: map[string]string{
						v1beta1constants.LabelShootEndpointAdvertise: "true",
					},
				},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{"bar.example.org"},
						},
					},
				},
			}

			Expect(botanist.SeedClientSet.Client().Create(ctx, ingressA)).To(Succeed())
			Expect(botanist.SeedClientSet.Client().Create(ctx, ingressB)).To(Succeed())
			items, err := botanist.GetIngressAdvertisedEndpoints(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(items).To(HaveExactElements([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "ingress/ingress-1/0/0",
					URL:  "https://foo.example.org",
				},
				{
					Name: "ingress/ingress-2/0/0",
					URL:  "https://bar.example.org",
				},
			}))
		})
	})
})
