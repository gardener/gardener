// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authorizationv1 "k8s.io/api/authorization/v1"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/auth/seed"
)

var _ = Describe("Response", func() {
	var (
		reason        = "reason"
		code    int32 = 123
		fakeErr       = fmt.Errorf("fake")
	)

	Describe("#Allowed", func() {
		It("should return the expected status", func() {
			Expect(Allowed()).To(Equal(authorizationv1.SubjectAccessReviewStatus{
				Allowed: true,
			}))
		})
	})

	Describe("#Denied", func() {
		It("should return the expected status", func() {
			Expect(Denied(reason)).To(Equal(authorizationv1.SubjectAccessReviewStatus{
				Allowed: false,
				Denied:  true,
				Reason:  reason,
			}))
		})
	})

	Describe("#NoOpinion", func() {
		It("should return the expected status", func() {
			Expect(NoOpinion(reason)).To(Equal(authorizationv1.SubjectAccessReviewStatus{
				Allowed: false,
				Reason:  reason,
			}))
		})
	})

	Describe("#Errored", func() {
		It("should return the expected status", func() {
			Expect(Errored(code, fakeErr)).To(Equal(authorizationv1.SubjectAccessReviewStatus{
				EvaluationError: fmt.Sprintf("%d %s", code, fakeErr),
			}))
		})
	})
})
