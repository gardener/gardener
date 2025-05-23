// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Allowed constructs a response indicating that the given operation is allowed (without any patches). In contrast to
// sigs.k8s.io/controller-runtime/pkg/webhook/admission.Allowed it does not set the `.status.result` but the
// `.status.message` field.
func Allowed(msg string) admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Code: int32(http.StatusOK),
			},
		},
	}
	if len(msg) > 0 {
		resp.Result.Message = msg
	}
	return resp
}
