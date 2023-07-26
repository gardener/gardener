// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/operator/controller/garden/garden"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Garden controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	cfg *config.OperatorConfiguration,
	identity *gardencorev1beta1.Gardener,
) error {
	var (
		componentImageVectors imagevectorutils.ComponentImageVectors
		err                   error
	)

	if path := os.Getenv(imagevectorutils.ComponentOverrideEnv); path != "" {
		componentImageVectors, err = imagevectorutils.ReadComponentOverwriteFile(path)
		if err != nil {
			return fmt.Errorf("failed reading component-specific image vector override: %w", err)
		}
	}

	gardenClientMap, err := clientmapbuilder.
		NewGardenClientMapBuilder().
		WithRuntimeClient(mgr.GetClient()).
		WithClientConnectionConfig(&cfg.VirtualClientConnection).
		Build(mgr.GetLogger())
	if err != nil {
		return fmt.Errorf("failed to build garden ClientMap: %w", err)
	}
	if err := mgr.Add(gardenClientMap); err != nil {
		return err
	}

	if err := (&garden.Reconciler{
		Config:                *cfg,
		Identity:              identity,
		ComponentImageVectors: componentImageVectors,
		GardenClientMap:       gardenClientMap,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	if err := (&care.Reconciler{
		Config:          *cfg,
		GardenClientMap: gardenClientMap,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding Garden-Care controller: %w", err)
	}

	return nil
}
