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
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/util/retry"
)

// UpdaterInterface is an interface used to update the Project manifest.
// For any use other than testing, clients should create an instance using NewRealUpdater.
type UpdaterInterface interface {
	UpdateProjectStatus(project *gardenv1beta1.Project) (*gardenv1beta1.Project, error)
}

// NewRealUpdater returns a UpdaterInterface that updates the Project manifest, using the supplied client and projectLister.
func NewRealUpdater(k8sGardenClient kubernetes.Interface, projectLister gardenlisters.ProjectLister) UpdaterInterface {
	return &realUpdater{k8sGardenClient, projectLister}
}

type realUpdater struct {
	k8sGardenClient kubernetes.Interface
	projectLister   gardenlisters.ProjectLister
}

// UpdateProjectStatus updates the Project manifest. Implementations are required to retry on conflicts,
// but fail on other errors. If the returned error is nil Project's manifest has been successfully set.
func (u *realUpdater) UpdateProjectStatus(project *gardenv1beta1.Project) (*gardenv1beta1.Project, error) {
	var (
		newProject *gardenv1beta1.Project
		status     = project.Status
		updateErr  error
	)

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		project.Status = status
		newProject, updateErr = u.k8sGardenClient.Garden().GardenV1beta1().Projects().UpdateStatus(project)
		if updateErr == nil {
			return nil
		}
		updated, err := u.projectLister.Get(project.Name)
		if err == nil {
			project = updated.DeepCopy()
		} else {
			logger.Logger.Errorf("error getting updated Project %s from lister: %v", project.Name, err)
		}
		return updateErr
	}); err != nil {
		return nil, err
	}
	return newProject, nil
}

var _ UpdaterInterface = &realUpdater{}
