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

package net

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("#GetBitLen", func() {
	It("should parse IPv4 address correctly", func() {
		ip := "10.10.0.26"
		Expect(GetBitLen(ip)).To(Equal(32))
	})
	It("should parse IPv6 address correctly", func() {
		ip := "2002:db8:3::"
		Expect(GetBitLen(ip)).To(Equal(128))
	})
	It("should fail parsing IPv4 address and return the default 32", func() {
		ip := "500.500.500.123"
		Expect(GetBitLen(ip)).To(Equal(32))
	})
	It("should fail parsing IPv6 address and return the default 32", func() {
		ip := "XYZ:db8:3::"
		Expect(GetBitLen(ip)).To(Equal(32))
	})
})
