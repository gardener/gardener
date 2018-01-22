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

package seed

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/util/retry"
)

// UpdaterInterface is an interface used to update the Seed manifest.
// For any use other than testing, clients should create an instance using NewRealUpdater.
type UpdaterInterface interface {
	UpdateSeedStatus(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error)
}

// NewRealUpdater returns a UpdaterInterface that updates the Seed manifest, using the supplied client and seedLister.
func NewRealUpdater(k8sGardenClient kubernetes.Client, seedLister gardenlisters.SeedLister) UpdaterInterface {
	return &realUpdater{k8sGardenClient, seedLister}
}

type realUpdater struct {
	k8sGardenClient kubernetes.Client
	seedLister      gardenlisters.SeedLister
}

// UpdateSeedStatus updates the Seed manifest. Implementations are required to retry on conflicts,
// but fail on other errors. If the returned error is nil Seed's manifest has been successfully set.
func (u *realUpdater) UpdateSeedStatus(seed *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
	var (
		newSeed   *gardenv1beta1.Seed
		status    = seed.Status
		updateErr error
	)

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		seed.Status = status
		newSeed, updateErr = u.k8sGardenClient.UpdateSeedStatus(seed)
		if updateErr == nil {
			return nil
		}
		updated, err := u.seedLister.Get(seed.Name)
		if err == nil {
			seed = updated.DeepCopy()
		} else {
			logger.Logger.Errorf("error getting updated Seed %s from lister: %v", seed.Name, err)
		}
		return updateErr
	}); err != nil {
		return nil, err
	}
	return newSeed, nil
}

var _ UpdaterInterface = &realUpdater{}
