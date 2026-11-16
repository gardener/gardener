// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"fmt"
	"net/http"
	"slices"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

// restrictedOperationAnnotationValues are the values of the `gardener.cloud/operation` annotation which a gardenlet
// must not set on `Gardenlet`/`ManagedSeed` resources, since they trigger a redeployment of the gardenlet from the
// object-supplied Helm chart.
var restrictedOperationAnnotationValues = []string{
	v1beta1constants.OperationForceRedeploy,
	v1beta1constants.GardenerOperationRenewKubeconfig,
}

// ForbidsRestrictedOperationAnnotation returns an error response if the new object sets the `gardener.cloud/operation`
// annotation to a restricted value which the old object did not already carry. It returns nil otherwise.
func ForbidsRestrictedOperationAnnotation(oldAnnotations, newAnnotations map[string]string, kind string) *admission.Response {
	newOperation := newAnnotations[v1beta1constants.GardenerOperation]
	if newOperation == oldAnnotations[v1beta1constants.GardenerOperation] || !slices.Contains(restrictedOperationAnnotationValues, newOperation) {
		return nil
	}

	response := admission.Errored(http.StatusForbidden, fmt.Errorf("gardenlet must not set the %q annotation to %q on %s", v1beta1constants.GardenerOperation, newOperation, kind))
	return &response
}
