// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shootstate"
	"github.com/gardener/gardener/pkg/healthz"
)

// AddControllersToManager adds all gardenlet controllers to the given manager.
func AddControllersToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	seedClientSet kubernetes.Interface,
	cfg *config.GardenletConfiguration,
	gardenNamespace *corev1.Namespace,
	gardenClusterIdentity string,
	healthManager healthz.Manager,
) error {
	identity, err := determineIdentity()
	if err != nil {
		return err
	}

	if err := controllerinstallation.AddToManager(mgr, gardenCluster, seedCluster, seedClientSet, *cfg, identity, gardenNamespace, gardenClusterIdentity); err != nil {
		return fmt.Errorf("failed adding ControllerInstallation controller: %w", err)
	}

	if err := seed.AddToManager(mgr, gardenCluster, seedCluster, seedClientSet, *cfg, healthManager); err != nil {
		return fmt.Errorf("failed adding Seed controller: %w", err)
	}

	if err := shootstate.AddToManager(mgr, gardenCluster, seedCluster, *cfg); err != nil {
		return fmt.Errorf("failed adding ShootState controller: %w", err)
	}

	return nil
}
