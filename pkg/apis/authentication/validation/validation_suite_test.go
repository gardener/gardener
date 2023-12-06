/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validation_test

import (
	"math"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/authentication"
	"github.com/gardener/gardener/pkg/apis/authentication/validation"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIs Authentication Validation Suite")
}

var _ = Describe("ValidateAdminKubeconfigRequest", func() {
	var req *authentication.AdminKubeconfigRequest

	BeforeEach(func() {
		req = &authentication.AdminKubeconfigRequest{}
	})

	It("should fail when expirationSeconds is negative", func() {
		req.Spec.ExpirationSeconds = -1

		errors := validation.ValidateAdminKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeInvalid),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should fail when expirationSeconds is less than 10 minutes", func() {
		req.Spec.ExpirationSeconds = int64((time.Minute * 9).Seconds() + (time.Second * 59).Seconds())

		errors := validation.ValidateAdminKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeInvalid),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should fail when expirationSeconds is more than 2^32 seconds", func() {
		req.Spec.ExpirationSeconds = math.MaxUint32 + 1

		errors := validation.ValidateAdminKubeconfigRequest(req)

		Expect(errors).To(HaveLen(1))
		Expect(errors).To(ConsistOfFields(Fields{
			"Type":  Equal(field.ErrorTypeTooLong),
			"Field": Equal("spec.expirationSeconds"),
		}))
	})

	It("should succeed when expirationSeconds is more than 10 minutes, but less than 2^32 seconds", func() {
		req.Spec.ExpirationSeconds = int64((time.Minute * 10).Seconds()) + 1

		errors := validation.ValidateAdminKubeconfigRequest(req)

		Expect(errors).To(BeEmpty())
	})
})
