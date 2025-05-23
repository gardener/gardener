// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission"
)

var _ = Describe("Response", func() {
	Describe("#Allowed", func() {
		It("should return the expected response (w/o msg)", func() {
			Expect(Allowed("")).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: int32(http.StatusOK),
					},
				},
			}))
		})

		It("should return the expected response (w/ msg)", func() {
			msg := "foo"

			Expect(Allowed(msg)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code:    int32(http.StatusOK),
						Message: msg,
					},
				},
			}))
		})
	})
})
