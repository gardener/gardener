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

package controllerregistration

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newBackupBucketEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return nil
		}

		// only buckets with a seed assigned can trigger a reconciliation
		if backupBucket.Spec.SeedName == nil {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: *backupBucket.Spec.SeedName,
			},
		}}
	})
}

func newBackupEntryEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return nil
		}

		// only entries with a seed assigned can trigger a reconciliation
		if backupEntry.Spec.SeedName == nil {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: *backupEntry.Spec.SeedName,
			},
		}}
	})
}

func newControllerDeploymentEventHandler(ctx context.Context, c client.Client, logger logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
		if err := c.List(ctx, controllerRegistrationList); err != nil {
			logger.Error(err, "Error listing controllerregistrations")
			return nil
		}

		for _, controllerReg := range controllerRegistrationList.Items {
			deployment := controllerReg.Spec.Deployment
			if deployment == nil {
				continue
			}

			for _, ref := range deployment.DeploymentRefs {
				if ref.Name == obj.GetName() {
					return enqueueAllSeeds(ctx, c)
				}
			}
		}

		return nil
	})
}

func newControllerInstallationEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return nil
		}

		// if gardencorev1beta1helper.IsControllerInstallationRequired(*oldObject) == gardencorev1beta1helper.IsControllerInstallationRequired(*newObject) {
		// 	return
		// }

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: controllerInstallation.Spec.SeedRef.Name,
			},
		}}
	})
}

func newControllerRegistrationEventHandler(ctx context.Context, c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		return enqueueAllSeeds(ctx, c)
	})
}

func newShootEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil
		}

		if shoot.Spec.SeedName == nil {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: *shoot.Spec.SeedName,
			},
		}}
	})
}

func enqueueAllSeeds(ctx context.Context, c client.Client) []reconcile.Request {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return nil
	}

	requests := []reconcile.Request{}

	for _, seed := range seedList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: seed.Name,
			},
		})
	}

	return requests
}
