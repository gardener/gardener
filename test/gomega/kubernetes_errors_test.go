// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gomega_test

import (
	"fmt"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("BeNotFoundError", func() {
	It("should be true when error is not found", func() {
		err := apierrors.NewNotFound(schema.GroupResource{Group: "baz", Resource: "bar"}, "foo")
		Expect(err).To(BeNotFoundError())
	})

	It("should be false when error is not k8s not found error", func() {
		err := apierrors.NewGone("opsie")
		Expect(err).ToNot(BeNotFoundError())
	})

	It("should be false when error is random error", func() {
		err := fmt.Errorf("not k8s error")
		Expect(err).ToNot(BeNotFoundError())
	})

	It("should be false when error is nil", func() {
		Expect(nil).ToNot(BeNotFoundError())
	})

	It("should throw error when actual is not error", func() {
		success, err := BeNotFoundError().Match("not an error")

		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})
})

var _ = Describe("BeForbiddenError", func() {
	It("should be true when error is fobidden", func() {
		err := apierrors.NewForbidden(schema.GroupResource{Group: "baz", Resource: "bar"}, "foo", fmt.Errorf("got err"))
		Expect(err).To(BeForbiddenError())
	})

	It("should be false when error is not k8s forbidden", func() {
		err := apierrors.NewGone("opsie")
		Expect(err).ToNot(BeForbiddenError())
	})

	It("should be false when error is random error", func() {
		err := fmt.Errorf("not k8s error")
		Expect(err).ToNot(BeForbiddenError())
	})

	It("should be false when error is nil", func() {
		Expect(nil).ToNot(BeForbiddenError())
	})

	It("should throw error when actual is not error", func() {
		success, err := BeForbiddenError().Match("not an error")

		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})
})

var _ = Describe("BeBadRequestError", func() {
	It("should be true when error is bad request", func() {
		err := apierrors.NewBadRequest("some reason")
		Expect(err).To(BeBadRequestError())
	})

	It("should be false when error is not k8s bad request", func() {
		err := apierrors.NewGone("opsie")
		Expect(err).ToNot(BeBadRequestError())
	})

	It("should be false when error is random error", func() {
		err := fmt.Errorf("not k8s error")
		Expect(err).ToNot(BeBadRequestError())
	})

	It("should be false when error is nil", func() {
		Expect(nil).ToNot(BeBadRequestError())
	})

	It("should throw error when actual is not error", func() {
		success, err := BeBadRequestError().Match("not an error")

		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})
})
