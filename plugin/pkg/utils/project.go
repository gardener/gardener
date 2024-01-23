// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
)

// ProjectForNamespaceFromExternalLister returns the Project responsible for a given <namespace>. It lists all Projects
// via the given lister, iterates over them and tries to identify the Project by looking for the namespace name
// in the project spec.
func ProjectForNamespaceFromExternalLister(projectLister gardencorev1beta1listers.ProjectLister, namespaceName string) (*core.Project, error) {
	projectList, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, project := range projectList {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			coreProject := &core.Project{}
			if err := gardencorev1beta1.Convert_v1beta1_Project_To_core_Project(project, coreProject, nil); err != nil {
				return nil, apierrors.NewInternalError(fmt.Errorf("could not convert v1beta1 project: %+v", err.Error()))
			}

			return coreProject, nil
		}
	}

	return nil, apierrors.NewNotFound(core.Resource("Project"), namespaceName)
}
