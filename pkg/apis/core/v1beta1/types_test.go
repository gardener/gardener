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

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("#IsIPv4SingleStack", func() {
	It("should return true for empty IP families", func() {
		Expect(IsIPv4SingleStack(nil)).To(BeTrue())
	})
	It("should return true for IPv4 single-stack", func() {
		Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv4})).To(BeTrue())
	})
	It("should return false for dual-stack", func() {
		Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv4, IPFamilyIPv6})).To(BeFalse())
		Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv6, IPFamilyIPv4})).To(BeFalse())
	})
	It("should return false for IPv6 single-stack", func() {
		Expect(IsIPv4SingleStack([]IPFamily{IPFamilyIPv6})).To(BeFalse())
	})
})

var _ = Describe("#IsIPv6SingleStack", func() {
	It("should return false for empty IP families", func() {
		Expect(IsIPv6SingleStack(nil)).To(BeFalse())
	})
	It("should return false for IPv4 single-stack", func() {
		Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv4})).To(BeFalse())
	})
	It("should return false for dual-stack", func() {
		Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv4, IPFamilyIPv6})).To(BeFalse())
		Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv6, IPFamilyIPv4})).To(BeFalse())
	})
	It("should return true for IPv6 single-stack", func() {
		Expect(IsIPv6SingleStack([]IPFamily{IPFamilyIPv6})).To(BeTrue())
	})
})
