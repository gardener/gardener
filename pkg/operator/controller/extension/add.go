// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/extension/virtualcluster"
)

// AddToManager adds all Garden controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	cfg *config.OperatorConfiguration,
	gardenClientMap clientmap.ClientMap,
) error {
	if gardenClientMap == nil {
		return fmt.Errorf("gardenClientMap cannot be nil")
	}

	if err := (&virtualcluster.Reconciler{
		Config:          *cfg,
		GardenClientMap: gardenClientMap,
		GardenNamespace: v1beta1constants.GardenNamespace,
	}).AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("failed adding Garden controller: %w", err)
	}

	return nil
}