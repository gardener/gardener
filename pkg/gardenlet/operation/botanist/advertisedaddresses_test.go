// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("AdvertisedAddresses", func() {
	var (
		botanist *Botanist
	)

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#ToAdvertisedAddresses", func() {
		It("returns empty list when shoot is nil", func() {
			botanist.Shoot = nil

			addresses, err := botanist.ToAdvertisedAddresses()
			Expect(err).ToNot(HaveOccurred())
			Expect(addresses).To(BeNil())
		})

		It("returns external address", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")

			addresses, err := botanist.ToAdvertisedAddresses()
			Expect(err).ToNot(HaveOccurred())
			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "external",
				URL:  "https://api.foo.bar",
			}))
		})

		It("returns internal and service-account-issuer addresses", func() {
			botanist.Shoot.InternalClusterDomain = "baz.foo"

			addresses, err := botanist.ToAdvertisedAddresses()
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

			addresses, err := botanist.ToAdvertisedAddresses()
			Expect(err).ToNot(HaveOccurred())

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "unmanaged",
				URL:  "https://bar.foo",
			}))
		})

		It("returns external, internal, service-account-issuer addresses in correct order", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = "baz.foo"
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses()
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
			botanist.Shoot.InternalClusterDomain = "baz.foo"
			botanist.Shoot.GetInfo().Spec.Kubernetes = gardencorev1beta1.Kubernetes{
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer: ptr.To("https://foo.bar.example.issuer"),
					},
				},
			}
			botanist.APIServerAddress = "bar.foo"

			addresses, err := botanist.ToAdvertisedAddresses()
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
			botanist.Shoot.InternalClusterDomain = "baz.foo"
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

			addresses, err := botanist.ToAdvertisedAddresses()
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
			botanist.Shoot.InternalClusterDomain = "baz.foo"

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

			addresses, err := botanist.ToAdvertisedAddresses()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("shoot requires managed issuer, but gardener does not have shoot service account hostname configured"))
			Expect(addresses).To(BeNil())
		})
	})
})
