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

package errors_test

import (
	"errors"
	"fmt"

	utilerrors "github.com/gardener/gardener/pkg/utils/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unwrap", func() {
	var (
		singleErr  = errors.New("single")
		wrappedErr = fmt.Errorf("outer err:%w", singleErr)
		nilErr     error
	)

	Describe("#SingleError", func() {
		It("should return the input error as root error if single error is passed", func() {
			Expect(utilerrors.Unwrap(singleErr)).To(BeIdenticalTo(singleErr))
		})
	})

	Describe("#WrappedError", func() {
		It("should return the root error if wrapped error is passed", func() {
			Expect(utilerrors.Unwrap(wrappedErr)).To(BeIdenticalTo(singleErr))
		})
	})

	Describe("#NilError", func() {
		It("should return nil if nil error is passed", func() {
			Expect(utilerrors.Unwrap(nilErr)).To(BeNil())
		})
	})
})
