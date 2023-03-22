// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
