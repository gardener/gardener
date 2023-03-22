// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/backupbucketscheck"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/extensionscheck"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/lifecycle"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/secrets"
)

// AddToManager adds all Seed controllers to the given manager.
func AddToManager(mgr manager.Manager, cfg config.ControllerManagerConfiguration) error {
	if err := (&backupbucketscheck.Reconciler{
		Config: *cfg.Controllers.SeedBackupBucketsCheck,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding backupbuckets check reconciler: %w", err)
	}

	if err := (&extensionscheck.Reconciler{
		Config: *cfg.Controllers.SeedExtensionsCheck,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding extensions check reconciler: %w", err)
	}

	if err := (&lifecycle.Reconciler{
		Config: *cfg.Controllers.Seed,
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding lifecycle reconciler: %w", err)
	}

	if err := (&secrets.Reconciler{}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding secrets reconciler: %w", err)
	}

	return nil
}
