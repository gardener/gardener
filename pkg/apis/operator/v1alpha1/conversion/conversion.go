// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package conversion

import (
	admissioncontrollerv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ConvertToAdmissionControllerResourceAdmissionConfiguration converts the given 'ResourceAdmissionConfiguration' into
// a 'admissioncontrollerv1alpha1.ResourceAdmissionConfiguration' object.
// Note: References from the given config are re-used, the function does not deep copy the data.
func ConvertToAdmissionControllerResourceAdmissionConfiguration(config *operatorv1alpha1.ResourceAdmissionConfiguration) *admissioncontrollerv1alpha1.ResourceAdmissionConfiguration {
	if config == nil {
		return nil
	}

	out := &admissioncontrollerv1alpha1.ResourceAdmissionConfiguration{
		UnrestrictedSubjects: config.UnrestrictedSubjects,
		OperationMode:        (*admissioncontrollerv1alpha1.ResourceAdmissionWebhookMode)(config.OperationMode),
	}

	for _, limit := range config.Limits {
		if config.Limits == nil {
			out.Limits = make([]admissioncontrollerv1alpha1.ResourceLimit, 0, len(config.Limits))
		}

		out.Limits = append(out.Limits, admissioncontrollerv1alpha1.ResourceLimit{
			APIGroups:   limit.APIGroups,
			APIVersions: limit.APIVersions,
			Resources:   limit.Resources,
			Size:        limit.Size,
		})
	}

	return out
}
