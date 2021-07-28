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

package managedseedset

import (
	"context"
	"fmt"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "managedseedset"
)

// AddToManager adds a new bastion controller to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	config *config.ManagedSeedSetControllerConfiguration,
) error {
	gardenClient := mgr.GetClient()
	logger := mgr.GetLogger()

	replicaFactory := ReplicaFactoryFunc(NewReplica)
	replicaGetter := NewReplicaGetter(gardenClient, mgr.GetAPIReader(), replicaFactory)
	actuator := NewActuator(gardenClient, replicaGetter, replicaFactory, config, mgr.GetEventRecorderFor(ControllerName+"-actuator"), logger)
	reconciler := NewReconciler(gardenClient, actuator, config, logger)

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	logger = c.GetLogger()
	reconciler.logger = logger
	actuator.logger = logger.WithName("actuator")

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := c.Watch(&source.Kind{Type: managedSeedSet}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", managedSeedSet, err)
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := c.Watch(&source.Kind{Type: managedSeed}, &handler.EnqueueRequestForOwner{OwnerType: managedSeedSet, IsController: true}, newManagedSeedPredicate(logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", managedSeed, err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, &handler.EnqueueRequestForOwner{OwnerType: managedSeedSet, IsController: true}, newShootPredicate(logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", shoot, err)
	}

	// Add event handler for controlled seeds
	seed := &gardencorev1beta1.Seed{}
	handler := &kutil.ControlledResourceEventHandler{
		ControllerTypes: []kutil.ControllerType{
			{
				Type:      &seedmanagementv1alpha1.ManagedSeed{},
				Namespace: pointer.String(v1beta1constants.GardenNamespace),
				NameFunc:  func(obj client.Object) string { return obj.GetName() },
			},
			{Type: &seedmanagementv1alpha1.ManagedSeedSet{}},
		},
		Ctx:    ctx,
		Reader: mgr.GetCache(),
		Scheme: kubernetes.GardenScheme,
	}
	if err := c.Watch(&source.Kind{Type: seed}, handler.ToHandler(), newSeedPredicate(logger)); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", seed, err)
	}

	return nil
}
