// Copyright 2018 The Gardener Authors.
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

package shoot

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/util/retry"
)

// UpdaterInterface is an interface used to update the Shoot manifest.
// For any use other than testing, clients should create an instance using NewRealUpdater.
type UpdaterInterface interface {
	UpdateShootStatus(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error)
	UpdateShootStatusIfNoOperation(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error)
}

// NewRealUpdater returns a UpdaterInterface that updates the Shoot manifest, using the supplied client and shootLister.
func NewRealUpdater(k8sGardenClient kubernetes.Client, shootLister gardenlisters.ShootLister) UpdaterInterface {
	return &realUpdater{k8sGardenClient, shootLister}
}

type realUpdater struct {
	k8sGardenClient kubernetes.Client
	shootLister     gardenlisters.ShootLister
}

// UpdateShootStatus updates the Shoot manifest. Implementations are required to retry on conflicts,
// but fail on other errors. If the returned error is nil Shoot's manifest has been successfully set.
func (u *realUpdater) UpdateShootStatus(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return u.update(shoot, func(shoot *gardenv1beta1.Shoot) bool { return false })
}

// UpdateShootStatusIfNoOperation updates the Shoot manifest, but retrying is only performed when the status
// does not indicate a running operations. If the returned error is nil Shoot's manifest has been
// successfully set.
func (u *realUpdater) UpdateShootStatusIfNoOperation(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
	return u.update(shoot, operationOngoing)
}

func (u *realUpdater) update(shoot *gardenv1beta1.Shoot, abortRetryFunc func(shoot *gardenv1beta1.Shoot) bool) (*gardenv1beta1.Shoot, error) {
	var (
		newShoot  *gardenv1beta1.Shoot
		status    = shoot.Status
		updateErr error
	)

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		shoot.Status = status
		newShoot, updateErr = u.k8sGardenClient.UpdateShootStatus(shoot)
		if updateErr == nil {
			return nil
		}
		updated, err := u.shootLister.Shoots(shoot.Namespace).Get(shoot.Name)
		if err == nil {
			shoot = updated.DeepCopy()
			if abortRetryFunc(shoot) {
				logger.Logger.Debugf("will not update the Shoot '%s'", shoot.Name)
				return nil
			}
		} else {
			logger.Logger.Errorf("error getting updated Shoot %s/%s from lister: %v", shoot.Namespace, shoot.Name, err)
		}
		return updateErr
	}); err != nil {
		return nil, err
	}
	return newShoot, nil
}

var _ UpdaterInterface = &realUpdater{}
