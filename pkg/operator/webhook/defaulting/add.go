// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package defaulting

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/operator/webhook/defaulting/extension"
	"github.com/gardener/gardener/pkg/operator/webhook/defaulting/garden"
)

// AddToManager adds all webhook handlers to the given manager.
func AddToManager(mgr manager.Manager) error {
	if err := (&garden.Handler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", garden.HandlerName, err)
	}

	if err := (&extension.Handler{
		Decoder: admission.NewDecoder(mgr.GetScheme()),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding %s webhook handler: %w", extension.HandlerName, err)
	}

	return nil
}
