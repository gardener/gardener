// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
)

var _ = Describe("Ignored", func() {
	Describe("#IsIgnored", func() {
		var obj *corev1.Secret

		BeforeEach(func() {
			obj = &corev1.Secret{}
		})

		It("should return false because annotation does not exist", func() {
			Expect(IsIgnored(obj)).To(BeFalse())
		})

		It("should return false because annotation value is not truthy", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/ignore": "foo"}
			Expect(IsIgnored(obj)).To(BeFalse())
		})

		It("should return true because annotation value is  truthy", func() {
			obj.Annotations = map[string]string{"resources.gardener.cloud/ignore": "true"}
			Expect(IsIgnored(obj)).To(BeTrue())
		})
	})
})
