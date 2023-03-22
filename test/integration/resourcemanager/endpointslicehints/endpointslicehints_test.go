// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package endpointslicehints_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
				Zone:      pointer.String("europe-1a"),
			},
		}
		Expect(testClient.Create(ctx, endpointSlice)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(endpointSlice), endpointSlice)).To(Succeed())
		Expect(endpointSlice.Endpoints).To(Equal([]discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      pointer.String("europe-1a"),
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
				Zone:      pointer.String(""),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Zone:      pointer.String("europe-1c"),
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
				Zone:      pointer.String(""),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1c"}},
				},
				Zone: pointer.String("europe-1c"),
			},
		}))
	})

	It("should default the hints when endpoint has a zone", func() {
		endpointSlice.Endpoints = []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.1.2.3"},
				Zone:      pointer.String("europe-1a"),
			},
			{
				Addresses: []string{"10.1.2.4"},
				Zone:      pointer.String("europe-1b"),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Zone:      pointer.String("europe-1c"),
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
				Zone: pointer.String("europe-1a"),
			},
			{
				Addresses: []string{"10.1.2.4"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1b"}},
				},
				Zone: pointer.String("europe-1b"),
			},
			{
				Addresses: []string{"10.1.2.5"},
				Hints: &discoveryv1.EndpointHints{
					ForZones: []discoveryv1.ForZone{{Name: "europe-1c"}},
				},
				Zone: pointer.String("europe-1c"),
			},
		}))
	})
})
