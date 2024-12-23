package helper

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// IsControllerInstallationSuccessful returns true if a ControllerInstallation has been marked as "successfully"
// installed.
func IsControllerInstallationSuccessful(controllerInstallation gardencorev1beta1.ControllerInstallation) bool {
	var (
		installed      bool
		healthy        bool
		notProgressing bool
	)

	for _, condition := range controllerInstallation.Status.Conditions {
		if condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status == gardencorev1beta1.ConditionTrue {
			installed = true
		}
		if condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status == gardencorev1beta1.ConditionTrue {
			healthy = true
		}
		if condition.Type == gardencorev1beta1.ControllerInstallationProgressing && condition.Status == gardencorev1beta1.ConditionFalse {
			notProgressing = true
		}
	}

	return installed && healthy && notProgressing
}

// IsControllerInstallationRequired returns true if a ControllerInstallation has been marked as "required".
func IsControllerInstallationRequired(controllerInstallation gardencorev1beta1.ControllerInstallation) bool {
	for _, condition := range controllerInstallation.Status.Conditions {
		if condition.Type == gardencorev1beta1.ControllerInstallationRequired && condition.Status == gardencorev1beta1.ConditionTrue {
			return true
		}
	}
	return false
}
