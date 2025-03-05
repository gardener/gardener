// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/operator/webhook/validation/garden"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager) error {
	if err := (&garden.Handler{
		Logger: mgr.GetLogger().WithName("webhook").WithName(garden.HandlerName),
	}).AddToManager(mgr); err != nil {
		return err
	}

	return nil
}
