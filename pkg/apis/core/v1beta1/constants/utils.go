// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

// GetShootUseAsSeedAnnotation fetches the value for AnnotationShootUseAsSeed annotation.
// If not present, it fallbacks to AnnotationShootUseAsSeedDeprecated.
func GetShootUseAsSeedAnnotation(annotations map[string]string) (string, bool) {
	return getDeprecatedAnnotation(annotations, AnnotationShootUseAsSeed, AnnotationShootUseAsSeedDeprecated)
}

// GetShootIgnoreAlertsAnnotation fetches the value for AnnotationShootIgnoreAlerts annotation.
// If not present, it fallbacks to AnnotationShootIgnoreAlertsDeprecated.
func GetShootIgnoreAlertsAnnotation(annotations map[string]string) (string, bool) {
	return getDeprecatedAnnotation(annotations, AnnotationShootIgnoreAlerts, AnnotationShootIgnoreAlertsDeprecated)
}

func getDeprecatedAnnotation(annotations map[string]string, annotationKey, deprecatedAnnotationKey string) (string, bool) {
	val, ok := annotations[annotationKey]
	if !ok {
		val, ok = annotations[deprecatedAnnotationKey]
	}

	return val, ok
}

// GetShootVPADeploymentNames returns the names of all VPA related deployments related to shoot clusters.
func GetShootVPADeploymentNames() []string {
	return []string{
		DeploymentNameVPAAdmissionController,
		DeploymentNameVPARecommender,
		DeploymentNameVPAUpdater,
	}
}
