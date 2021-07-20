// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package plant

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
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
	ControllerName = "exposureclass-controller"

	// FinalizerName is the name of the Plant finalizer.
	FinalizerName = "core.gardener.cloud/plant"
)

// AddToManager adds a new exposureclass controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	clientMap clientmap.ClientMap,
	config *config.PlantControllerConfiguration,
) error {
	ctrlOptions := controller.Options{
		Reconciler:              NewReconciler(mgr.GetLogger(), clientMap, mgr.GetClient(), config),
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	plant := &gardencorev1beta1.Plant{}
	if err := c.Watch(&source.Kind{Type: plant}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", plant, err)
	}

	secretHandler := handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		// Ignore non-secrets
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}

		// list all related plants
		plantList := gardencorev1beta1.PlantList{}

		// TODO: Can't this be restricted to the secret's namespace instead of listing _all_ plants
		// in _all_ namespaces?
		if err := mgr.GetClient().List(ctx, &plantList); err != nil {
			mgr.GetLogger().Error(err, "Failed to list Plants")
			return nil
		}

		requests := []reconcile.Request{}
		for _, plant := range plantList.Items {
			if isPlantSecret(plant, kutil.Key(secret.Namespace, secret.Name)) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: plant.Namespace,
						Name:      plant.Name,
					},
				})
			}
		}

		return requests
	})

	secret := &corev1.Secret{}
	if err := c.Watch(&source.Kind{Type: secret}, secretHandler); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", secret, err)
	}

	return nil
}
