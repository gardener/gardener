// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ControllerName is the name of this controller.
const ControllerName = "backupbucket"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
			RateLimiter:             r.RateLimiter,
		},
	)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.BackupBucket{}, gardenCluster.GetCache()),
		controllerutils.EnqueueCreateEventsOncePer24hDuration(r.Clock),
		&predicate.GenerationChangedPredicate{},
		r.SeedNamePredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.BackupBucket{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapExtensionBackupBucketToCoreBackupBucket), mapper.UpdateWithNew, c.GetLogger()),
		ExtensionStatusChanged(),
	)
}

// SeedNamePredicate returns a predicate which returns true when the object belongs to this seed.
func (r *Reconciler) SeedNamePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}
		return pointer.StringDeref(backupBucket.Spec.SeedName, "") == r.SeedName
	})
}

// MapExtensionBackupBucketToCoreBackupBucket is a mapper.MapFunc for mapping a extensions.gardener.cloud/v1alpha1.BackupBucket to the owning
// core.gardener.cloud/v1beta1.BackupBucket.
func (r *Reconciler) MapExtensionBackupBucketToCoreBackupBucket(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

// ExtensionStatusChanged returns a predicate which returns true when the status of the extension object has changed.
func ExtensionStatusChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// If the object has the operation annotation, this means it's not picked up by the extension controller.
			if hasOperationAnnotation(e.Object) {
				return false
			}

			// If a lastOperation is not recorded yet, we skip enqueueing.
			if lastOperationNotPresent(e.Object) {
				return false
			}

			// If any of relevant status fields changed or lastError is present then we admit reconciliation.
			if lastErrorPresent(e.Object) {
				return true
			}

			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			// If the object has the operation annotation, this means it's not picked up by the extension controller.
			if hasOperationAnnotation(e.ObjectNew) {
				return false
			}

			// If a lastOperation is not recorded yet, we skip enqueueing.
			if lastOperationNotPresent(e.ObjectNew) {
				return false
			}

			// If any of relevant status fields changed or lastError is present then we admit reconciliation.
			if statusChanged(e.ObjectOld, e.ObjectNew) {
				return true
			}

			return false
		},

		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func lastErrorPresent(obj client.Object) bool {
	acc, err := extensions.Accessor(obj)
	if err != nil {
		return false
	}

	return acc.GetExtensionStatus().GetLastError() != nil
}

func lastOperationNotPresent(obj client.Object) bool {
	acc, err := extensions.Accessor(obj)
	if err != nil {
		return false
	}

	return acc.GetExtensionStatus().GetLastOperation() == nil
}

func statusChanged(oldObj, newObj client.Object) bool {
	oldAcc, err := extensions.Accessor(oldObj)
	if err != nil {
		return false
	}
	newAcc, err := extensions.Accessor(newObj)
	if err != nil {
		return false
	}

	return !reflect.DeepEqual(oldAcc.GetExtensionStatus(), newAcc.GetExtensionStatus())
}

func hasOperationAnnotation(obj client.Object) bool {
	return obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
		obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
		obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
}
