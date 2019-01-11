// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
)

func setProjectPhase(phase gardenv1beta1.ProjectPhase) func(*gardenv1beta1.Project) (*gardenv1beta1.Project, error) {
	return func(project *gardenv1beta1.Project) (*gardenv1beta1.Project, error) {
		project.Status.Phase = phase
		return project, nil
	}
}

func namespaceLabelsFromProject(project *gardenv1beta1.Project) map[string]string {
	return map[string]string{
		common.GardenRole:  common.GardenRoleProject,
		common.ProjectName: project.Name,
	}
}

func namespaceAnnotationsFromProject(project *gardenv1beta1.Project) map[string]string {
	return map[string]string{
		common.NamespaceProject: string(project.UID),
	}
}
