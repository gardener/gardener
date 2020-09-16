// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/common"
)

func setProjectPhase(phase gardencorev1beta1.ProjectPhase) func(*gardencorev1beta1.Project) (*gardencorev1beta1.Project, error) {
	return func(project *gardencorev1beta1.Project) (*gardencorev1beta1.Project, error) {
		project.Status.Phase = phase
		return project, nil
	}
}

func namespaceLabelsFromProject(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleProject,
		common.ProjectName:          project.Name,
	}
}

func namespaceLabelsFromProjectDeprecated(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleProject,
		common.ProjectNameDeprecated:          project.Name,
	}
}

func namespaceAnnotationsFromProject(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		common.NamespaceProject:           string(project.UID),
		common.NamespaceProjectDeprecated: string(project.UID),
	}
}

func namespaceAnnotationsFromProjectDeprecated(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		common.NamespaceProjectDeprecated: string(project.UID),
	}
}
