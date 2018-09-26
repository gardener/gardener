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

package utils_test

import (
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils"

	"errors"
	"io/ioutil"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	errwrap "github.com/pkg/errors"
)

var (
	testErr = errors.New("severe")
)

var _ = Describe("utils", func() {
	Context("#RetryUntil", func() {
		It("should abort immediately on a severe error and return it", func() {
			ct := 0
			err := RetryUntil(0*time.Second, NeverStop, func() (ok, severe bool, err error) {
				if ct > 0 {
					Fail("Function called multiple times although should have already failed")
				}
				ct++
				return false, true, testErr
			})

			Expect(err).To(Equal(testErr))
		})

		It("should not error if the function exits cleanly", func() {
			err := RetryUntil(0*time.Second, NeverStop, func() (ok, severe bool, err error) {
				return true, false, nil
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("#Retry", func() {
		BeforeSuite(func() {
			logger.NewLogger("")
			logger.Logger.Out = ioutil.Discard
		})

		It("should fail due to a timeout containing the last error", func() {
			err := Retry(0*time.Second, 0*time.Second, func() (ok, severe bool, err error) {
				return false, false, testErr
			})

			Expect(err).To(HaveOccurred())
			Expect(errwrap.Cause(err)).To(Equal(testErr))
		})

		It("should fail due to a timeout containing no last error", func() {
			err := Retry(0*time.Second, 0*time.Second, func() (ok, severe bool, err error) {
				return false, false, nil
			})

			Expect(err).To(HaveOccurred())
			Expect(errwrap.Cause(err)).To(Equal(err))
		})

		It("should timeout early and don't use the value of the delayed function", func() {
			err := Retry(0*time.Second, 0*time.Second, func() (ok, severe bool, err error) {
				time.Sleep(10 * time.Second)
				return true, false, nil
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
