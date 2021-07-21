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
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "controllerregistration-controller"

	// FinalizerName is the finalizer used by this controller.
	FinalizerName = "core.gardener.cloud/controllerregistration"

	controllerRegistrationRequestKind     = "controllerregistration"
	controllerRegistrationSeedRequestKind = "controllerregistration-seed"
	seedRequestKind                       = "seed"
)

type multiplexReconciler struct {
	controllerRegistrationReconciler     reconcile.Reconciler
	controllerRegistrationSeedReconciler reconcile.Reconciler
	seedReconciler                       reconcile.Reconciler
}

func newMultiplexReconciler(
	controllerRegistrationReconciler reconcile.Reconciler,
	controllerRegistrationSeedReconciler reconcile.Reconciler,
	seedReconciler reconcile.Reconciler,
) reconcile.Reconciler {
	return &multiplexReconciler{
		controllerRegistrationReconciler:     controllerRegistrationReconciler,
		controllerRegistrationSeedReconciler: controllerRegistrationSeedReconciler,
		seedReconciler:                       seedReconciler,
	}
}

func (r *multiplexReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var target reconcile.Reconciler

	switch request.Namespace {
	case controllerRegistrationRequestKind:
		target = r.controllerRegistrationReconciler
	case controllerRegistrationSeedRequestKind:
		target = r.controllerRegistrationSeedReconciler
	case seedRequestKind:
		target = r.seedReconciler
	default:
		return reconcile.Result{}, fmt.Errorf("invalid request kind %s", request.Namespace)
	}

	return target.Reconcile(ctx, request)
}

// AddToManager adds a new controllerregistration controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.ControllerRegistrationControllerConfiguration,
) error {
	logger := mgr.GetLogger()

	controllerRegistrationReconciler := NewControllerRegistrationReconciler(logger.WithName("controllerregistration"), mgr.GetClient())
	controllerRegistrationSeedReconciler := NewControllerRegistrationReconciler(logger.WithName("controllerregistration-seed"), mgr.GetClient())
	seedReconciler := NewSeedReconciler(logger.WithName("seed"), mgr.GetClient())

	ctrlOptions := controller.Options{
		Reconciler: newMultiplexReconciler(
			controllerRegistrationReconciler,
			controllerRegistrationSeedReconciler,
			seedReconciler,
		),
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	// For all of these watched resource kinds, we eventually enqueue the related seed(s) and reconcile from there.

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := c.Watch(&source.Kind{Type: backupBucket}, newBackupBucketEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", backupBucket, err)
	}

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := c.Watch(&source.Kind{Type: backupEntry}, newBackupEntryEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", backupEntry, err)
	}

	controllerDeployment := &gardencorev1beta1.ControllerDeployment{}
	if err := c.Watch(&source.Kind{Type: controllerDeployment}, newControllerDeploymentEventHandler(ctx, mgr.GetClient(), logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", controllerDeployment, err)
	}

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := c.Watch(&source.Kind{Type: controllerInstallation}, newControllerInstallationEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", controllerInstallation, err)
	}

	controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
	if err := c.Watch(&source.Kind{Type: controllerRegistration}, newControllerRegistrationEventHandler(ctx, mgr.GetClient())); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", controllerRegistration, err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := c.Watch(&source.Kind{Type: seed}, newSeedEventHandler(ctx, mgr.GetClient())); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", seed, err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, newShootEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", shoot, err)
	}

	return nil
}

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
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      *backupBucket.Spec.SeedName,
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
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      *backupEntry.Spec.SeedName,
			},
		}}
	})
}

func newControllerDeploymentEventHandler(ctx context.Context, c client.Client, logger logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		controllerDeployment, ok := obj.(*gardencorev1beta1.ControllerDeployment)
		if !ok {
			return nil
		}

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
				if ref.Name == controllerDeployment.Name {
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
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      controllerInstallation.Spec.SeedRef.Name,
			},
		}}
	})
}

func newControllerRegistrationEventHandler(ctx context.Context, c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return nil
		}

		// if gardencorev1beta1helper.IsControllerInstallationRequired(*oldObject) == gardencorev1beta1helper.IsControllerInstallationRequired(*newObject) {
		// 	return
		// }

		requests := enqueueAllSeeds(ctx, c)
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: controllerInstallation.Spec.SeedRef.Name,
			},
		})

		return requests
	})
}

func newSeedEventHandler(ctx context.Context, c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: seedRequestKind,
				Name:      seed.Name,
			},
		}, {
			NamespacedName: types.NamespacedName{
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      seed.Name,
			},
		}}
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
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      *shoot.Spec.SeedName,
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
				Namespace: controllerRegistrationSeedRequestKind,
				Name:      seed.Name,
			},
		})
	}

	return requests
}
