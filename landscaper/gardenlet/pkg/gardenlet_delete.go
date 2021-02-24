// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package pkg

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/landscaper/gardenlet/pkg/applier"
)

// Delete removes all deployed Gardenlet resources from the Seed cluster.
func (g Landscaper) Delete(ctx context.Context) error {
	shootList := &gardencorev1beta1.ShootList{}
	err := g.gardenClient.Client().List(ctx, shootList)
	if err != nil {
		return fmt.Errorf("cannot delete Seed '%s'. Failed to check if Seed is still in use by shoot(s): %v", g.gardenletConfiguration.SeedConfig.Name, err)
	}

	seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{
		Name: g.gardenletConfiguration.SeedConfig.Name,
	}}

	exists, err := g.seedExists(ctx, seed)
	if err != nil {
		return fmt.Errorf("failed to check if Seed %q exists in the Garden cluster. Aborting. %v", g.gardenletConfiguration.SeedConfig.Name, err)
	}

	if exists {
		if isSeedUsedByAnyShoot(g.gardenletConfiguration.SeedConfig.Name, shootList.Items) {
			return fmt.Errorf("cannot delete seed '%s' which is still used by at least one shoot", g.gardenletConfiguration.SeedConfig.Name)
		}

		err := g.waitForSeedDeletion(ctx, seed)
		if err != nil {
			return err
		}
	}

	chartApplier := g.seedClient.ChartApplier()
	applier := applier.NewGardenletChartApplier(chartApplier, map[string]interface{}{}, GetChartPath())
	if err := applier.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to delete the Gardenlet resources from the Seed cluster %q: %v", g.gardenletConfiguration.SeedConfig.Name, err)
	}

	g.log.Infof("Successfully deleted Gardenlet resources for Seed %q", g.gardenletConfiguration.SeedConfig.Name)
	return nil
}

// waitForSeedDeletion waits until the Seed resource has been deleted from the Garden cluster
func (g Landscaper) waitForSeedDeletion(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	if err := retry.UntilTimeout(ctx, 10*time.Second, 2*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = g.gardenClient.Client().Delete(ctx, seed)
		if err != nil {
			g.log.Infof("Error setting deletion timestamp on seed %q: %v", g.gardenletConfiguration.SeedConfig.Name, err)
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return fmt.Errorf("failed setting deletion timestamp on seed %q: %v", g.gardenletConfiguration.SeedConfig.Name, err)
	}

	if err := retry.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(ctx context.Context) (done bool, err error) {
		seedExists, err := g.seedExists(ctx, seed)
		if err != nil {
			g.log.Infof("Error while waiting for seed to be deleted: %s", err.Error())
			return retry.MinorError(err)
		}

		if !seedExists {
			g.log.Infof("seed %q has been deleted successfully", seed.Name)
			return retry.Ok()
		}

		g.log.Infof("waiting for seed %s to be deleted", seed.Name)
		return retry.MinorError(fmt.Errorf("seed %q still exists", seed.Name))
	}); err != nil {
		return fmt.Errorf("failed waiting for the deletion of Seed %q: %v", g.gardenletConfiguration.SeedConfig.Name, err)
	}
	return nil
}

// seedExists checks if the given Seed resource exists in the garden cluster
func (g *Landscaper) seedExists(ctx context.Context, seed *gardencorev1beta1.Seed) (bool, error) {
	err := g.gardenClient.Client().Get(ctx, client.ObjectKey{Name: seed.Name}, seed)
	if err != nil {
		if apierrors.IsNotFound(err) || g.isIntegrationTest {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isSeedUsedByAnyShoot checks whether there is a shoot cluster referencing the provided seed name
func isSeedUsedByAnyShoot(seedName string, shoots []gardencorev1beta1.Shoot) bool {
	for _, shoot := range shoots {
		if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seedName {
			return true
		}
		if shoot.Status.SeedName != nil && *shoot.Status.SeedName == seedName {
			return true
		}
	}
	return false
}
