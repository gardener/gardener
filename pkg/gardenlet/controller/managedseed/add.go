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

package managedseed

import (
	"context"

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "managedseed"

	// GardenletDefaultKubeconfigSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.KubeconfigSecret.Name
	GardenletDefaultKubeconfigSecretName = "gardenlet-kubeconfig"
	// GardenletDefaultKubeconfigBootstrapSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.BootstrapKubeconfig.Name
	GardenletDefaultKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(
	mgr manager.Manager,
	cfg config.GardenletConfiguration,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	shootClientMap clientmap.ClientMap,
	imageVector imagevector.ImageVector,
) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	valuesHelper := NewValuesHelper(&cfg, imageVector)

	r.Actuator = newActuator(gardenCluster.GetConfig(),
		gardenCluster.GetAPIReader(),
		gardenCluster.GetClient(),
		seedCluster.GetClient(),
		shootClientMap,
		valuesHelper,
		gardenCluster.GetEventRecorderFor(ControllerName+"-controller"),
		r.ChartsPath,
		r.GardenNamespace,
	)

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

	seedName := confighelper.SeedNameFromSeedConfig(cfg.SeedConfig)

	if err := c.Watch(
		source.NewKindWithCache(&seedmanagementv1alpha1.ManagedSeed{}, gardenCluster.GetCache()),
		controllerutils.EnqueueWithJitterDelay(r.Config),
		r.ManagedSeedFilterPredicate(seedName),
		&predicate.GenerationChangedPredicate{},
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.Seed{}, gardenCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapSeedToManagedSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.ManagedSeedFilterPredicate(seedName),
	)
}

// ManagedSeedFilterPredicate returns the predicate for ManagedSeed and Seed events.
func (r *Reconciler) ManagedSeedFilterPredicate(seedName string) predicate.Predicate {
	return &managedSeedFilterPredicate{
		seedName: seedName,
		reader:   r.GardenClient,
	}
}

type managedSeedFilterPredicate struct {
	ctx      context.Context
	reader   client.Reader
	seedName string
}

func (p *managedSeedFilterPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *managedSeedFilterPredicate) Create(e event.CreateEvent) bool {
	return p.filterManagedSeed(e.Object)
}

func (p *managedSeedFilterPredicate) Update(e event.UpdateEvent) bool {
	return p.filterManagedSeed(e.ObjectNew)
}

func (p *managedSeedFilterPredicate) Delete(e event.DeleteEvent) bool {
	return p.filterManagedSeed(e.Object)
}

func (p *managedSeedFilterPredicate) Generic(_ event.GenericEvent) bool { return false }

// filterManagedSeed checks if the ManagedSeed references a Shoot scheduled on a Seed, for which the gardenlet is responsible.
func (p *managedSeedFilterPredicate) filterManagedSeed(obj client.Object) bool {
	var managedSeed *seedmanagementv1alpha1.ManagedSeed
	switch obj := obj.(type) {
	case *seedmanagementv1alpha1.ManagedSeed:
		managedSeed = obj
	case *gardencorev1beta1.Seed:
		managedSeed = &seedmanagementv1alpha1.ManagedSeed{}
		if err := p.reader.Get(p.ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, obj.Name), managedSeed); err != nil {
			return false
		}
	default:
		return false
	}

	if managedSeed.Spec.Shoot == nil || managedSeed.Spec.Shoot.Name == "" {
		return false
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := p.reader.Get(p.ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return false
	}

	if shoot.Spec.SeedName == nil {
		return false
	}

	if shoot.Status.SeedName == nil || *shoot.Spec.SeedName == *shoot.Status.SeedName {
		return *shoot.Spec.SeedName == p.seedName
	}

	return *shoot.Status.SeedName == p.seedName
}

// MapSeedToManagedSeed is a mapper.MapFunc for mapping a Seed to the owning ManagedSeed.
func (r *Reconciler) MapSeedToManagedSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: gardencorev1beta1constants.GardenNamespace, Name: obj.GetName()}}}
}
