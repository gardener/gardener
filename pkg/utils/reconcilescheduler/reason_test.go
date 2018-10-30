// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reconcilescheduler_test

import (
	"fmt"

	. "github.com/gardener/gardener/pkg/utils/reconcilescheduler"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("reconcilescheduler", func() {
	Context("Reason", func() {
		DescribeTable("#NewReason",
			func(code int, msgFmt string, args ...interface{}) {
				reason := NewReason(code, msgFmt, args...)

				Expect(reason.Code()).To(Equal(code))
				Expect(reason.Message()).To(Equal(fmt.Sprintf(msgFmt, args...)))
			},

			Entry("code with unformatted message", CodeOther, "some nice message"),
			Entry("code with unformatted message", CodeParentUnknown, "some %s format", "foo"),
		)

		var (
			code    = CodeParentActive
			message = "must wait for parent"
			reason  = NewReason(code, message)
		)

		Describe("#String", func() {
			It("should return the correct string representation", func() {
				Expect(reason.String()).To(Equal(message))
			})
		})

		Describe("#Code", func() {
			It("should return the correct code", func() {
				Expect(reason.Code()).To(Equal(code))
			})
		})

		Describe("#Message", func() {
			It("should return the correct message", func() {
				Expect(reason.Message()).To(Equal(message))
			})
		})
	})
})
