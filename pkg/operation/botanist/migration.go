// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AnnotateExtensionCRsForMigration annotates extension CRs with migrate operation annotation
func (b *Botanist) AnnotateExtensionCRsForMigration(ctx context.Context) (err error) {
	fns, err := b.applyFuncToAllExtensionCRs(ctx, annotateObjectForMigrationFunc(ctx, b.K8sSeedClient.DirectClient()))
	if err != nil {
		return err
	}

	fns = append(fns,
		b.Shoot.Components.Extensions.ContainerRuntime.Migrate,
		b.Shoot.Components.Extensions.ControlPlane.Migrate,
		b.Shoot.Components.Extensions.ControlPlaneExposure.Migrate,
		b.Shoot.Components.Extensions.Extension.Migrate,
		b.Shoot.Components.Extensions.Infrastructure.Migrate,
		b.Shoot.Components.Extensions.Network.Migrate,
		b.Shoot.Components.Extensions.Worker.Migrate,
	)

	return flow.Parallel(fns...)(ctx)
}

func annotateObjectForMigrationFunc(ctx context.Context, client client.Client) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		extensionObj := obj.(extensionsv1alpha1.Object)
		return common.AnnotateExtensionObjectWithOperation(ctx, client, extensionObj, v1beta1constants.GardenerOperationMigrate)
	}
}

// WaitForExtensionsOperationMigrateToSucceed waits until extension CRs has lastOperation Migrate Succeeded
func (b *Botanist) WaitForExtensionsOperationMigrateToSucceed(ctx context.Context) error {
	fns, err := b.applyFuncToAllExtensionCRs(ctx, func(obj runtime.Object) error {
		extensionObj, err := extensions.Accessor(obj)
		if err != nil {
			return err
		}
		return common.WaitUntilExtensionCRMigrated(
			ctx,
			b.K8sSeedClient.DirectClient(),
			func() extensionsv1alpha1.Object { return extensionObj },
			extensionObj.GetName(),
			extensionObj.GetNamespace(),
			5*time.Second,
			300*time.Second,
		)
	})
	if err != nil {
		return err
	}

	fns = append(fns,
		b.Shoot.Components.Extensions.ContainerRuntime.WaitMigrate,
		b.Shoot.Components.Extensions.ControlPlane.WaitMigrate,
		b.Shoot.Components.Extensions.ControlPlaneExposure.WaitMigrate,
		b.Shoot.Components.Extensions.Extension.WaitMigrate,
		b.Shoot.Components.Extensions.Infrastructure.WaitMigrate,
		b.Shoot.Components.Extensions.Network.WaitMigrate,
		b.Shoot.Components.Extensions.Worker.WaitMigrate,
	)

	return flow.Parallel(fns...)(ctx)
}

// DeleteAllExtensionCRs deletes all extension CRs from the Shoot namespace
func (b *Botanist) DeleteAllExtensionCRs(ctx context.Context) error {
	fns, err := b.applyFuncToAllExtensionCRs(ctx, func(obj runtime.Object) error {
		extensionObj, err := extensions.Accessor(obj)
		if err != nil {
			return err
		}
		return common.DeleteExtensionCR(ctx, b.K8sSeedClient.Client(), func() extensionsv1alpha1.Object { return extensionObj }, extensionObj.GetNamespace(), extensionObj.GetName())
	})
	if err != nil {
		return err
	}

	fns = append(fns,
		b.Shoot.Components.Extensions.ContainerRuntime.Destroy,
		b.Shoot.Components.Extensions.ControlPlane.Destroy,
		b.Shoot.Components.Extensions.ControlPlaneExposure.Destroy,
		b.Shoot.Components.Extensions.Extension.Destroy,
		b.Shoot.Components.Extensions.Infrastructure.Destroy,
		b.Shoot.Components.Extensions.Network.Destroy,
		b.Shoot.Components.Extensions.Worker.Destroy,
	)

	return flow.Parallel(fns...)(ctx)
}

func (b *Botanist) applyFuncToAllExtensionCRs(ctx context.Context, applyFunc func(obj runtime.Object) error) ([]flow.TaskFn, error) {
	var fns []flow.TaskFn
	for _, listObj := range extensions.GetShootNamespacedCRsLists() {
		listObjCopy := listObj
		if err := b.K8sSeedClient.Client().List(ctx, listObjCopy, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
			return nil, err
		}
		fns = append(fns, func(ctx context.Context) error {
			return meta.EachListItem(listObjCopy, applyFunc)
		})
	}

	return fns, nil
}

func (b *Botanist) restoreExtensionObject(ctx context.Context, extensionObj extensionsv1alpha1.Object, resourceKind string) error {
	if err := b.K8sSeedClient.DirectClient().Get(ctx, kutil.KeyFromObject(extensionObj), extensionObj); err != nil {
		return err
	}

	if err := common.RestoreExtensionObjectState(ctx, b.K8sSeedClient.Client(), b.ShootState, b.Shoot.SeedNamespace, extensionObj, resourceKind); err != nil {
		return err
	}

	return common.AnnotateExtensionObjectWithOperation(ctx, b.K8sSeedClient.Client(), extensionObj, v1beta1constants.GardenerOperationRestore)
}

// AnnotateBackupEntryInSeedForMigration annotates the BackupEntry with gardener.cloud/operation=migrate
func (b *Botanist) AnnotateBackupEntryInSeedForMigration(ctx context.Context) error {
	name := common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID)
	return common.MigrateExtensionCR(
		ctx,
		b.K8sSeedClient.Client(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		name,
	)
}

// WaitForBackupEntryOperationMigrateToSucceed waits until BackupEntry has lastOperation equal to Migrate=Succeeded
func (b *Botanist) WaitForBackupEntryOperationMigrateToSucceed(ctx context.Context) error {
	name := common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID)
	return common.WaitUntilExtensionCRMigrated(
		ctx,
		b.K8sSeedClient.DirectClient(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		name,
		5*time.Second,
		600*time.Second,
	)
}

// DeleteBackupEntryFromSeed deletes the migrated BackupEntry from the Seed
func (b *Botanist) DeleteBackupEntryFromSeed(ctx context.Context) error {
	name := common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID)
	return common.DeleteExtensionCR(
		ctx,
		b.K8sSeedClient.Client(),
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		name,
	)
}

func (b *Botanist) isRestorePhase() bool {
	return b.Shoot != nil &&
		b.Shoot.Info != nil &&
		b.Shoot.Info.Status.LastOperation != nil &&
		b.Shoot.Info.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
}
