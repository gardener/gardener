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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

// LegacyControllerFactory starts gardenlet's legacy controllers under leader election of the given manager for
// the purpose of gradually migrating to native controller-runtime controllers.
// Deprecated: this will be replaced by adding native controllers directly to the manager.
// New controllers should be implemented as native controller-runtime controllers right away and should be added to
// the manager directly.
type LegacyControllerFactory struct {
	GardenCluster  cluster.Cluster
	SeedCluster    cluster.Cluster
	ShootClientMap clientmap.ClientMap
	Log            logr.Logger
	Config         *config.GardenletConfiguration
}

// Start starts all legacy controllers.
func (f *LegacyControllerFactory) Start(ctx context.Context) error {
	log := f.Log.WithName("controller")

	log.Info("gardenlet initialized")

	// block until shutting down
	<-ctx.Done()
	return nil
}
