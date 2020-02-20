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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AnnotateExtensionCRsForMigration annotates extension CRs with migrate operation annotation
func (b *Botanist) AnnotateExtensionCRsForMigration(ctx context.Context) (err error) {
	var fns []flow.TaskFn
	fns, err = b.applyFuncToAllExtensionCRs(ctx, annotateObjectForMigrationFunc(ctx, b.K8sSeedClient.Client()))
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

func annotateObjectForMigrationFunc(ctx context.Context, client client.Client) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		return kutil.TryUpdate(ctx, k8sretry.DefaultBackoff, client, obj, func() error {
			acc, err := extensions.Accessor(obj)
			if err != nil {
				return err
			}

			kutil.SetMetaDataAnnotation(acc, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate)
			return nil
		})
	}
}

// DeleteAllExtensionCRs deletes all extension CRs from the Shoot namespace
func (b *Botanist) DeleteAllExtensionCRs(ctx context.Context) error {
	fns, err := b.applyFuncToAllExtensionCRs(ctx, func(obj runtime.Object) error {
		if err := common.ConfirmDeletion(ctx, b.K8sSeedClient.Client(), obj); err != nil {
			return err
		}

		if err := client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, obj, kubernetes.DefaultDeleteOptions...)); err != nil {
			accObj, err := meta.Accessor(obj)
			if err != nil {
				return err
			}
			return fmt.Errorf("couldn't delete CR %s with name %s: %v", obj.GetObjectKind(), accObj.GetName(), err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flow.Parallel(fns...)(ctx)
}

// WaitForExtensionsOperationMigrateToSucceed waits until extension CRs has lastOperation Migrate Succeeded
func (b *Botanist) WaitForExtensionsOperationMigrateToSucceed(ctx context.Context) error {
	fns, err := b.applyFuncToAllExtensionCRs(ctx, func(obj runtime.Object) error {
		return retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
			acc, err := extensions.Accessor(obj)
			if err != nil {
				return retry.SevereError(err)
			}

			exrtensionObjStatus := acc.GetExtensionStatus()
			if exrtensionObjStatus != nil && exrtensionObjStatus.GetLastOperation() != nil {
				lastOperation := exrtensionObjStatus.GetLastOperation()
				if lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
					return retry.Ok()
				}
			}

			var exetnsionType string
			if extensionSpec := acc.GetExtensionSpec(); extensionSpec != nil {
				exetnsionType = extensionSpec.GetExtensionType()
			}
			return retry.MinorError(fmt.Errorf("extension CR %s with type %s lastOperation was not Migrate=Succeeded", acc.GetName(), exetnsionType))
		})
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

func (b *Botanist) applyFuncToAllExtensionCRs(ctx context.Context, applyFunc func(obj runtime.Object) error) ([]flow.TaskFn, error) {
	var fns []flow.TaskFn
	for _, listObj := range extensions.GetAllGardenerExtensionsLists() {
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

func (b *Botanist) restoreExtensionObject(ctx context.Context, client client.Client, obj runtime.Object, objMeta *metav1.ObjectMeta, objStatus *extensionsv1alpha1.DefaultStatus, resourceKind, resourceName string, purpose *string) error {
	if err := b.restoreExtensionObjectState(ctx, client, obj, objStatus, resourceKind, objMeta.GetName(), purpose); err != nil {
		return err
	}
	return b.annotateExtensionObjectWithOperationRestore(ctx, client, obj, objMeta)
}

func (b *Botanist) restoreExtensionObjectState(ctx context.Context, client client.Client, extensionObj runtime.Object, objStatus *extensionsv1alpha1.DefaultStatus, resourceKind, resourceName string, purpose *string) error {
	return kutil.TryUpdateStatus(ctx, k8sretry.DefaultBackoff, client, extensionObj, func() error {
		if b.ShootState.Spec.Extensions != nil {
			list := gardencorev1alpha1helper.ExtensionResourceStateList(b.ShootState.Spec.Extensions)
			еxtensionResourceState := list.Get(resourceKind, &resourceName, purpose)
			if еxtensionResourceState != nil {
				objStatus.State = &еxtensionResourceState.State
			}
		}
		return nil
	})
}

func (b *Botanist) annotateExtensionObjectWithOperationRestore(ctx context.Context, client client.Client, extensionObj runtime.Object, objMeta *metav1.ObjectMeta) error {
	return kutil.TryUpdate(ctx, k8sretry.DefaultBackoff, client, extensionObj, func() error {
		metav1.SetMetaDataAnnotation(objMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore)
		return nil
	})
}

func (b *Botanist) isRestorePhase() bool {
	if b.Shoot == nil {
		return false
	}

	if lastOperation := b.Shoot.Info.Status.LastOperation; lastOperation != nil {
		return lastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
	}
	return false
}
