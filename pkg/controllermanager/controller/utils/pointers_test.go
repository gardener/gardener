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

package utils

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("pointers", func() {
	DescribeTable("#BoolPtrDerefOr",
		func(b *bool, defaultValue bool, match types.GomegaMatcher) {
			Expect(BoolPtrDerefOr(b, defaultValue)).To(match)
		},

		Entry("with true value false default", pointer.BoolPtr(true), false, BeTrue()),
		Entry("with true value true default", pointer.BoolPtr(true), true, BeTrue()),
		Entry("with false value true default", pointer.BoolPtr(false), true, BeFalse()),
		Entry("with false value false default", pointer.BoolPtr(false), false, BeFalse()),
		Entry("with no value true default", nil, true, BeTrue()),
		Entry("with no value false default", nil, false, BeFalse()),
	)
})
