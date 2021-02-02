// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// MigrateAllExtensionResources migrates all extension CRs.
func (b *Botanist) MigrateAllExtensionResources(ctx context.Context) (err error) {
	return b.runParallelTaskForEachExtensionComponent(ctx, func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Migrate
	})
}

// WaitUntilAllExtensionResourcesMigrated waits until all extension CRs were successfully migrated.
func (b *Botanist) WaitUntilAllExtensionResourcesMigrated(ctx context.Context) error {
	return b.runParallelTaskForEachExtensionComponent(ctx, func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.WaitMigrate
	})
}

// DestroyAllExtensionResources deletes all extension CRs from the Shoot namespace.
func (b *Botanist) DestroyAllExtensionResources(ctx context.Context) error {
	return b.runParallelTaskForEachExtensionComponent(ctx, func(c component.DeployMigrateWaiter) func(context.Context) error {
		return c.Destroy
	})
}

func (b *Botanist) runParallelTaskForEachExtensionComponent(ctx context.Context, fn func(component.DeployMigrateWaiter) func(context.Context) error) error {
	var fns []flow.TaskFn
	for _, component := range b.Shoot.GetExtensionComponents() {
		fns = append(fns, fn(component))
	}
	return flow.Parallel(fns...)(ctx)
}

func (b *Botanist) isRestorePhase() bool {
	return b.Shoot != nil &&
		b.Shoot.Info != nil &&
		b.Shoot.Info.Status.LastOperation != nil &&
		b.Shoot.Info.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
}
