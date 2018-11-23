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

package utils

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"k8s.io/apimachinery/pkg/labels"
)

// GetProject returns the Project resource the passed Shoot is assigned to.
func GetProject(shoot *garden.Shoot, projectLister gardenlisters.ProjectLister) (*garden.Project, error) {
	projects, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == shoot.Namespace {
			return project, nil
		}
	}
	return nil, fmt.Errorf("no project for shoot namespace %q", shoot.Namespace)
}
