// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/credentialsbinding/credentialsbinding"
	"github.com/gardener/gardener/pkg/controllermanager/controller/credentialsbinding/referencecleaner"
)

// AddToManager adds all CredentialsBinding controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg config.ControllerManagerConfiguration) error {
	if err := (&credentialsbinding.Reconciler{
		Config: *cfg.Controllers.CredentialsBinding,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding main reconciler: %w", err)
	}

	if err := (&referencecleaner.Reconciler{
		Config: *cfg.Controllers.CredentialsBindingReferenceCleaner,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding reference cleaner reconciler: %w", err)
	}

	return nil
}
