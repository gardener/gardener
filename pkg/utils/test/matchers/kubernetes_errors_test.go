// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BeNotFoundError", func() {
	It("should be true when error is not found", func() {
		err := apierrors.NewNotFound(schema.GroupResource{Group: "baz", Resource: "bar"}, "foo")
		Expect(err).To(BeNotFoundError())
	})

	It("should be false when error is not k8s not found error", func() {
		err := apierrors.NewResourceExpired("opsie")
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

var _ = Describe("BeAlreadyExistsError", func() {
	It("should be true when error is AlreadyExists", func() {
		err := apierrors.NewAlreadyExists(schema.GroupResource{Group: "baz", Resource: "bar"}, "foo")
		Expect(err).To(BeAlreadyExistsError())
	})

	It("should be false when error is not k8s AlreadyExists error", func() {
		err := apierrors.NewResourceExpired("opsie")
		Expect(err).ToNot(BeAlreadyExistsError())
	})

	It("should be false when error is random error", func() {
		err := fmt.Errorf("not k8s error")
		Expect(err).ToNot(BeAlreadyExistsError())
	})

	It("should be false when error is nil", func() {
		Expect(nil).ToNot(BeAlreadyExistsError())
	})

	It("should throw error when actual is not error", func() {
		success, err := BeAlreadyExistsError().Match("not an error")

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
		err := apierrors.NewResourceExpired("opsie")
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
		err := apierrors.NewResourceExpired("opsie")
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

var _ = Describe("BeNoMatchError", func() {
	It("should be true when error is NoKindMatch", func() {
		var err error = &meta.NoKindMatchError{}
		Expect(err).To(BeNoMatchError())
		err = &meta.NoResourceMatchError{}
		Expect(err).To(BeNoMatchError())
	})

	It("should be false when error is not a NoKindMatch", func() {
		err := apierrors.NewResourceExpired("opsie")
		Expect(err).ToNot(BeNoMatchError())
	})

	It("should be false when error is random error", func() {
		err := fmt.Errorf("not k8s error")
		Expect(err).ToNot(BeNoMatchError())
	})

	It("should be false when error is nil", func() {
		Expect(nil).ToNot(BeNoMatchError())
	})

	It("should throw error when actual is not error", func() {
		success, err := BeNoMatchError().Match("not an error")

		Expect(success).Should(BeFalse())
		Expect(err).Should(HaveOccurred())
	})
})
