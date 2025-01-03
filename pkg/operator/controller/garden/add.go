// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/operator/controller/garden/garden"
	"github.com/gardener/gardener/pkg/operator/controller/garden/reference"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// AddToManager adds all Garden controllers to the given manager.
func AddToManager(
	mgr manager.Manager,
	cfg *operatorconfigv1alpha1.OperatorConfiguration,
	identity *gardencorev1beta1.Gardener,
	gardenClientMap clientmap.ClientMap,
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

	if err := (&garden.Reconciler{
		Config:                *cfg,
		Identity:              identity,
		ComponentImageVectors: componentImageVectors,
		GardenNamespace:       v1beta1constants.GardenNamespace,
	}).AddToManager(mgr, gardenClientMap); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	if err := (&care.Reconciler{
		Config: *cfg,
	}).AddToManager(mgr, gardenClientMap); err != nil {
		return fmt.Errorf("failed adding care reconciler: %w", err)
	}

	if err := reference.AddToManager(mgr, v1beta1constants.GardenNamespace); err != nil {
		return fmt.Errorf("failed adding reference reconciler: %w", err)
	}

	return nil
}
