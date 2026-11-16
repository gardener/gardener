// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package admission_test

import (
	"fmt"
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

	Describe("#ForbidsRestrictedOperationAnnotation", func() {
		It("should forbid newly setting the operation annotation to force-redeploy", func() {
			response := ForbidsRestrictedOperationAnnotation(nil, map[string]string{"gardener.cloud/operation": "force-redeploy"}, "Gardenlet")

			Expect(response).NotTo(BeNil())
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Message).To(Equal(fmt.Sprintf("gardenlet must not set the %q annotation to %q on Gardenlet", "gardener.cloud/operation", "force-redeploy")))
		})

		It("should forbid newly setting the operation annotation to renew-kubeconfig", func() {
			response := ForbidsRestrictedOperationAnnotation(nil, map[string]string{"gardener.cloud/operation": "renew-kubeconfig"}, "ManagedSeed")

			Expect(response).NotTo(BeNil())
			Expect(response.Allowed).To(BeFalse())
			Expect(response.Result.Message).To(Equal(fmt.Sprintf("gardenlet must not set the %q annotation to %q on ManagedSeed", "gardener.cloud/operation", "renew-kubeconfig")))
		})

		It("should allow if no operation annotation is set", func() {
			Expect(ForbidsRestrictedOperationAnnotation(nil, map[string]string{"foo": "bar"}, "Gardenlet")).To(BeNil())
		})

		It("should allow if the operation annotation has a non-restricted value", func() {
			Expect(ForbidsRestrictedOperationAnnotation(nil, map[string]string{"gardener.cloud/operation": "reconcile"}, "Gardenlet")).To(BeNil())
		})

		It("should allow if the restricted operation annotation was already present and is unchanged", func() {
			annotations := map[string]string{"gardener.cloud/operation": "force-redeploy"}

			Expect(ForbidsRestrictedOperationAnnotation(annotations, annotations, "Gardenlet")).To(BeNil())
		})
	})
})
