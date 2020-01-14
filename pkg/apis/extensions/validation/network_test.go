// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("Network validation tests", func() {
	var network *extensionsv1alpha1.Network

	BeforeEach(func() {
		network = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-network",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.NetworkSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				PodCIDR:     "10.20.30.40/26",
				ServiceCIDR: "10.30.40.50/26",
			},
		}
	})

	Describe("#ValidNetwork", func() {
		It("should forbid empty Network resources", func() {
			errorList := ValidateNetwork(&extensionsv1alpha1.Network{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		It("should forbid Network with invalid CIDRs", func() {
			c := network.DeepCopy()
			c.Spec.PodCIDR = "this-is-no-cidr"
			c.Spec.ServiceCIDR = "this-is-still-no-cidr"

			errorList := ValidateNetwork(c)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		It("should forbid Network with overlapping pod and service CIDRs", func() {
			c := network.DeepCopy()
			c.Spec.PodCIDR = network.Spec.ServiceCIDR

			errorList := ValidateNetwork(c)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		It("should allow valid network resources", func() {
			errorList := ValidateNetwork(network)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidNetworkUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			network.DeletionTimestamp = &now

			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.DeletionTimestamp = &now
			newNetwork.Spec.ProviderConfig = nil

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})

		It("should prevent updating the type or the cidrs", func() {
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.Type = "changed-type"
			newNetwork.Spec.PodCIDR = "10.21.30.40/26"
			newNetwork.Spec.ServiceCIDR = "10.31.40.50/26"

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.podCIDR"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.serviceCIDR"),
			}))))
		})

		It("should allow updating the provider config", func() {
			newNetwork := prepareNetworkForUpdate(network)
			newNetwork.Spec.ProviderConfig = nil

			errorList := ValidateNetworkUpdate(newNetwork, network)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareNetworkForUpdate(obj *extensionsv1alpha1.Network) *extensionsv1alpha1.Network {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
