// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

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
