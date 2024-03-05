// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// CheckCertificate checks whether the given certificate object is healthy.
func CheckCertificate(cert *certv1alpha1.Certificate) error {
	for _, condition := range cert.Status.Conditions {
		if condition.Type == certv1alpha1.CertificateConditionReady {
			if err := checkConditionState(condition.Type, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
				return err
			}
			break
		}
	}

	if certState := cert.Status.State; certState != certv1alpha1.StateReady {
		return fmt.Errorf("certificate state is %q (%q expected)", certState, certv1alpha1.StateReady)
	}
	return nil
}
