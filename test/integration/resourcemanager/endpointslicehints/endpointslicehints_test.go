// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package endpointslicehints_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("EndpointSliceHints tests", func() {
	var endpointSlice *discoveryv1.EndpointSlice

	BeforeEach(func() {
		endpointSlice = &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels: map[string]string{
					"endpoint-slice-hints.resources.gardener.cloud/consider": "true",
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
		}
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, endpointSlice)).To(Succeed())
	})

	It("should not mutate when the EndpointSlice does not have the required consider label", func() {
		endpointSlice.Labels = nil
		endpointSlice.Endpoints = []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      ptr.To("europe-1a"),
			},
		}
		Expect(testClient.Create(ctx, endpointSlice)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(endpointSlice), endpointSlice)).To(Succeed())
		Expect(endpointSlice.Endpoints).To(Equal([]discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      ptr.To("europe-1a"),
			},
		}))
	})

	It("should not default the hints for endpoint without a zone", func() {
		endpointSlice.Endpoints = []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      nil,
			},
			{
				Addresses: []string{"10.1.2.4"},
				Zone:      ptr.To(""),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Zone:      ptr.To("europe-1c"),
			},
		}
		Expect(testClient.Create(ctx, endpointSlice)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(endpointSlice), endpointSlice)).To(Succeed())
		Expect(endpointSlice.Endpoints).To(Equal([]discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Hints:     nil,
				Zone:      nil,
			},
			{
				Addresses: []string{"10.1.2.4"},
				Hints:     nil,
				Zone:      ptr.To(""),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1c"}},
				},
				Zone: ptr.To("europe-1c"),
			},
		}))
	})

	It("should default the hints when endpoint has a zone", func() {
		endpointSlice.Endpoints = []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      ptr.To("europe-1a"),
			},
			{
				Addresses: []string{"10.1.2.4"},
				Zone:      ptr.To("europe-1b"),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Zone:      ptr.To("europe-1c"),
			},
		}
		Expect(testClient.Create(ctx, endpointSlice)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(endpointSlice), endpointSlice)).To(Succeed())
		Expect(endpointSlice.Endpoints).To(Equal([]discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1a"}},
				},
				Zone: ptr.To("europe-1a"),
			},
			{
				Addresses: []string{"10.1.2.4"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1b"}},
				},
				Zone: ptr.To("europe-1b"),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1c"}},
				},
				Zone: ptr.To("europe-1c"),
			},
		}))
	})
})
