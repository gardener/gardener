// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package updaterestriction_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/updaterestriction"
)

var _ = Describe("handler", func() {
	var (
		ctx     context.Context
		handler *updaterestriction.Handler
		req     admission.Request
	)

	BeforeEach(func() {
		ctx = context.TODO()
		handler = &updaterestriction.Handler{}
		req.UserInfo = authenticationv1.UserInfo{
			Username: "gardenlet",
			Groups:   []string{"gardener.cloud:system:seeds"},
		}
		req.Resource = metav1.GroupVersionResource{
			Resource: "configmaps",
		}
	})

	Describe("#Handle", func() {
		It("should allow the request as it is made by a gardenlet", func() {
			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{
					Code:    int32(200),
					Reason:  "",
					Message: "",
				},
			}))
		})

		It("should deny the request as it is not made by a gardenlet", func() {
			req.UserInfo = authenticationv1.UserInfo{
				Username: "not-gardenlet",
			}
			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    int32(403),
					Reason:  "Forbidden",
					Message: "user \"not-gardenlet\" is not allowed to modify system configmaps",
				},
			}))
		})
	})
})
