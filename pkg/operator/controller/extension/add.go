// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension"
)

// AddToManager adds the extension controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg *operatorconfigv1alpha1.OperatorConfiguration, gardenClientMap clientmap.ClientMap) error {
	if err := (&extension.Reconciler{
		Config:          *cfg,
		GardenClientMap: gardenClientMap,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	return nil
}
