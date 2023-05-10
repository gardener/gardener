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

package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reference Matcher", func() {
	test := func(actual, expected interface{}) {
		It("should be true if objects share the same reference", func() {
			sameRef := actual

			Expect(actual).To(ShareSameReferenceAs(sameRef))
		})

		It("should be false if objects don't share the same reference", func() {
			Expect(actual).NotTo(ShareSameReferenceAs(expected))
		})
	}

	Context("when values are maps", func() {
		test(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"})
	})

	Context("when values are slices", func() {
		test([]string{"foo", "bar"}, []string{"foo", "bar"})
	})

	Context("when values are pointers", func() {
		test(pointer.String("foo"), pointer.String("foo"))
	})
})
