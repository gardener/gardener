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

package bastion

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "bastion-controller"
)

// AddToManager adds a new bastion controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.BastionControllerConfiguration,
) error {
	reconciler := &reconciler{
		logger:       mgr.GetLogger(),
		gardenClient: mgr.GetClient(),
		maxLifetime:  config.MaxLifetime.Duration,
	}

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	shootHandler := handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		// list all bastions that reference this shoot
		bastionList := operationsv1alpha1.BastionList{}
		listOptions := client.ListOptions{Namespace: obj.GetNamespace()}

		if err := mgr.GetClient().List(ctx, &bastionList, &listOptions); err != nil {
			mgr.GetLogger().Error(err, "Failed to list Bastions")
			return nil
		}

		requests := []reconcile.Request{}
		for _, bastion := range bastionList.Items {
			if bastion.Spec.ShootRef.Name == obj.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: bastion.Namespace,
						Name:      bastion.Name,
					},
				})
			}
		}

		return requests
	})

	// reconcile bastions
	bastion := &operationsv1alpha1.Bastion{}
	if err := c.Watch(&source.Kind{Type: bastion}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", bastion, err)
	}

	// whenever a shoot is deleted, cleanup the associated bastions
	isDeleted := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetDeletionTimestamp() != nil
	})

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, shootHandler, isDeleted); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", shoot, err)
	}

	return nil
}
