// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package conversion

import (
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ConvertToAdmissionControllerResourceAdmissionConfiguration converts the given 'ResourceAdmissionConfiguration' into
// a 'admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration' object.
// Note: References from the given config are re-used, the function does not deep copy the data.
func ConvertToAdmissionControllerResourceAdmissionConfiguration(config *operatorv1alpha1.ResourceAdmissionConfiguration) *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration {
	if config == nil {
		return nil
	}

	out := &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
		UnrestrictedSubjects: config.UnrestrictedSubjects,
		OperationMode:        (*admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode)(config.OperationMode),
	}

	for _, limit := range config.Limits {
		if config.Limits == nil {
			out.Limits = make([]admissioncontrollerconfigv1alpha1.ResourceLimit, 0, len(config.Limits))
		}

		out.Limits = append(out.Limits, admissioncontrollerconfigv1alpha1.ResourceLimit{
			APIGroups:   limit.APIGroups,
			APIVersions: limit.APIVersions,
			Resources:   limit.Resources,
			Size:        limit.Size,
		})
	}

	return out
}
