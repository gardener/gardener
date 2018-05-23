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

package backupinfrastructure

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/util/retry"
)

// UpdaterInterface is an interface used to update the BackupInfrastructure manifest.
// For any use other than testing, clients should create an instance using NewRealUpdater.
type UpdaterInterface interface {
	UpdateBackupInfrastructureStatus(backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error)
}

// NewRealUpdater returns a UpdaterInterface that updates the BackupInfrastructure manifest, using the supplied client and backupInfrastructureLister.
func NewRealUpdater(k8sGardenClient kubernetes.Client, backupInfrastructureLister gardenlisters.BackupInfrastructureLister) UpdaterInterface {
	return &realUpdater{k8sGardenClient, backupInfrastructureLister}
}

type realUpdater struct {
	k8sGardenClient            kubernetes.Client
	backupInfrastructureLister gardenlisters.BackupInfrastructureLister
}

// UpdateBackupInfrastructureStatus updates the BackupInfrastructure manifest. Implementations are required to retry on conflicts,
// but fail on other errors. If the returned error is nil BackupInfrastructure's manifest has been successfully set.
func (u *realUpdater) UpdateBackupInfrastructureStatus(backupInfrastructure *gardenv1beta1.BackupInfrastructure) (*gardenv1beta1.BackupInfrastructure, error) {
	var (
		newBackupInfrastructure *gardenv1beta1.BackupInfrastructure
		status                  = backupInfrastructure.Status
		updateErr               error
	)

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		backupInfrastructure.Status = status
		newBackupInfrastructure, updateErr = u.k8sGardenClient.GardenClientset().GardenV1beta1().BackupInfrastructures(backupInfrastructure.Namespace).UpdateStatus(backupInfrastructure)
		if updateErr == nil {
			return nil
		}
		updated, err := u.backupInfrastructureLister.BackupInfrastructures(backupInfrastructure.Namespace).Get(backupInfrastructure.Name)
		if err == nil {
			backupInfrastructure = updated.DeepCopy()
		} else {
			logger.Logger.Errorf("error getting updated BackupInfrastructure %s from lister: %v", backupInfrastructure.Name, err)
		}
		return updateErr
	}); err != nil {
		return nil, err
	}
	return newBackupInfrastructure, nil
}

var _ UpdaterInterface = &realUpdater{}
