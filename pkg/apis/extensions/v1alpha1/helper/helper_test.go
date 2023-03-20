// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	DescribeTable("#ClusterAutoscalerRequired",
		func(pools []extensionsv1alpha1.WorkerPool, expected bool) {
			Expect(ClusterAutoscalerRequired(pools)).To(Equal(expected))
		},

		Entry("no pools", []extensionsv1alpha1.WorkerPool{}, false),
		Entry("min=max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 1,
			Maximum: 1,
		}}, false),
		Entry("min<max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 0,
			Maximum: 1,
		}}, true),
	)

	DescribeTable("#GetDNSRecordType",
		func(address string, expected extensionsv1alpha1.DNSRecordType) {
			Expect(GetDNSRecordType(address)).To(Equal(expected))
		},

		Entry("valid IPv4 address", "1.2.3.4", extensionsv1alpha1.DNSRecordTypeA),
		Entry("valid IPv6 address", "2001:db8:f00::1", extensionsv1alpha1.DNSRecordTypeAAAA),
		Entry("anything else", "foo", extensionsv1alpha1.DNSRecordTypeCNAME),
	)

	DescribeTable("#GetDNSRecordTTL",
		func(ttl *int64, expected int64) {
			Expect(GetDNSRecordTTL(ttl)).To(Equal(expected))
		},

		Entry("nil value", nil, int64(120)),
		Entry("non-nil value", pointer.Int64(300), int64(300)),
	)

	Describe("#DeterminePrimaryIPFamily", func() {
		It("should return IPv4 for empty ipFamilies", func() {
			Expect(DeterminePrimaryIPFamily(nil)).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv4 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv6 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
		})
	})
})
