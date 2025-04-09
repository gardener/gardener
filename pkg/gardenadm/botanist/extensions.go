// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ComputeExtensions takes a list of ControllerRegistrations and ControllerDeployments and computes a corresponding list
// of Extensions.
func ComputeExtensions(seedName string, controllerRegistrations []*gardencorev1beta1.ControllerRegistration, controllerDeployments []*gardencorev1.ControllerDeployment) ([]Extension, error) {
	var extensions []Extension

	for _, controllerRegistration := range controllerRegistrations {
		if controllerRegistration.Spec.Deployment == nil || len(controllerRegistration.Spec.Deployment.DeploymentRefs) != 1 {
			return nil, fmt.Errorf("ControllerRegistration %s has invalid deployment refs in its spec (must reference exactly one ControllerDeployment)", controllerRegistration.Name)
		}

		idx := slices.IndexFunc(controllerDeployments, func(controllerDeployment *gardencorev1.ControllerDeployment) bool {
			return controllerDeployment.Name == controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name
		})
		if idx == -1 {
			return nil, fmt.Errorf("ControllerDeployment %s referenced in ControllerRegistration %s was not found", controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name, controllerRegistration.Name)
		}

		var (
			controllerDeployment   = controllerDeployments[idx]
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				ObjectMeta: metav1.ObjectMeta{Name: controllerRegistration.Name},
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					RegistrationRef: corev1.ObjectReference{Name: controllerRegistration.Name},
					DeploymentRef:   &corev1.ObjectReference{Name: controllerDeployment.Name},
					SeedRef:         corev1.ObjectReference{Name: seedName},
				},
			}
		)

		extensions = append(extensions, Extension{
			ControllerRegistration: controllerRegistration,
			ControllerDeployment:   controllerDeployment,
			ControllerInstallation: controllerInstallation,
		})
	}

	return extensions, nil
}
