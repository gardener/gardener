// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/authentication"
	"github.com/gardener/gardener/pkg/apis/authentication/validation"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ValidateKubeconfigRequest", func() {
	var req *authentication.KubeconfigRequest

	BeforeEach(func() {
		req = &authentication.KubeconfigRequest{}
	})

	It("should fail when expirationSeconds is negative", func() {
		req.Spec.ExpirationSeconds = -1

		errors := validation.ValidateKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeInvalid),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should fail when expirationSeconds is less than 10 minutes", func() {
		req.Spec.ExpirationSeconds = int64((time.Minute * 9).Seconds() + (time.Second * 59).Seconds())

		errors := validation.ValidateKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeInvalid),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should fail when expirationSeconds is more than 2^32 seconds", func() {
		req.Spec.ExpirationSeconds = math.MaxUint32 + 1

		errors := validation.ValidateKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeTooLong),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should succeed when expirationSeconds is more than 10 minutes, but less than 2^32 seconds", func() {
		req.Spec.ExpirationSeconds = int64((time.Minute * 10).Seconds()) + 1

		errors := validation.ValidateKubeconfigRequest(req)

		Expect(errors).To(BeEmpty())
	})
})
