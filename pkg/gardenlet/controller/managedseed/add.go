// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	ctx context.Context,
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespaceGarden == "" {
		r.GardenNamespaceGarden = v1beta1constants.GardenNamespace
	}
	if r.GardenNamespaceShoot == "" {
		r.GardenNamespaceShoot = v1beta1constants.GardenNamespace
	}
	if r.ChartsPath == "" {
		r.ChartsPath = charts.Path
	}

	if r.Actuator == nil {
		r.Actuator = newActuator(
			gardenCluster.GetConfig(),
			gardenCluster.GetAPIReader(),
			gardenCluster.GetClient(),
			seedCluster.GetClient(),
			r.ShootClientMap,
			r.Clock,
			NewValuesHelper(&r.Config, r.ImageVector),
			gardenCluster.GetEventRecorderFor(ControllerName+"-controller"),
			r.ChartsPath,
			r.GardenNamespaceShoot,
		)
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.Controllers.ManagedSeed.ConcurrentSyncs, 0),
		}).
		Watches(
			source.NewKindWithCache(&seedmanagementv1alpha1.ManagedSeed{}, gardenCluster.GetCache()),
			r.EnqueueWithJitterDelay(),
			builder.WithPredicates(
				r.ManagedSeedPredicate(ctx, r.Config.SeedConfig.SeedTemplate.Name),
				&predicate.GenerationChangedPredicate{},
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.Seed{}, gardenCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapSeedToManagedSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.SeedOfManagedSeedPredicate(ctx, r.Config.SeedConfig.SeedTemplate.Name),
	)
}

// ManagedSeedPredicate returns the predicate for ManagedSeed events.
func (r *Reconciler) ManagedSeedPredicate(ctx context.Context, seedName string) predicate.Predicate {
	return &managedSeedPredicate{
		ctx:      ctx,
		reader:   r.GardenClient,
		seedName: seedName,
	}
}

type managedSeedPredicate struct {
	ctx      context.Context
	reader   client.Reader
	seedName string
}

func (p *managedSeedPredicate) Create(e event.CreateEvent) bool {
	return p.filterManagedSeed(e.Object)
}

func (p *managedSeedPredicate) Update(e event.UpdateEvent) bool {
	return p.filterManagedSeed(e.ObjectNew)
}

func (p *managedSeedPredicate) Delete(e event.DeleteEvent) bool {
	return p.filterManagedSeed(e.Object)
}

func (p *managedSeedPredicate) Generic(_ event.GenericEvent) bool { return false }

// filterManagedSeed is filtering func for ManagedSeeds that checks if the ManagedSeed references a Shoot scheduled on a Seed,
// for which the gardenlet is responsible.
func (p *managedSeedPredicate) filterManagedSeed(obj client.Object) bool {
	managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	return filterManagedSeed(p.ctx, p.reader, managedSeed, p.seedName)
}

// SeedOfManagedSeedPredicate returns the predicate for Seed events.
func (r *Reconciler) SeedOfManagedSeedPredicate(ctx context.Context, seedName string) predicate.Predicate {
	return &seedOfManagedSeedPredicate{
		ctx:             ctx,
		reader:          r.GardenClient,
		gardenNamespace: r.GardenNamespaceGarden,
		seedName:        seedName,
	}
}

type seedOfManagedSeedPredicate struct {
	ctx             context.Context
	reader          client.Reader
	gardenNamespace string
	seedName        string
}

func (p *seedOfManagedSeedPredicate) Create(e event.CreateEvent) bool {
	return p.filterSeedOfManagedSeed(e.Object)
}

func (p *seedOfManagedSeedPredicate) Update(e event.UpdateEvent) bool {
	return p.filterSeedOfManagedSeed(e.ObjectNew)
}

func (p *seedOfManagedSeedPredicate) Delete(e event.DeleteEvent) bool {
	return p.filterSeedOfManagedSeed(e.Object)
}

func (p *seedOfManagedSeedPredicate) Generic(_ event.GenericEvent) bool { return false }

// filterSeedOfManagedSeed is filtering func for Seeds that checks if the Seed is owned by a ManagedSeed that references a Shoot
// scheduled on a Seed, for which the gardenlet is responsible.
func (p *seedOfManagedSeedPredicate) filterSeedOfManagedSeed(obj client.Object) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := p.reader.Get(p.ctx, kubernetesutils.Key(p.gardenNamespace, seed.Name), managedSeed); err != nil {
		return false
	}

	return filterManagedSeed(p.ctx, p.reader, managedSeed, p.seedName)
}

func filterManagedSeed(ctx context.Context, reader client.Reader, managedSeed *seedmanagementv1alpha1.ManagedSeed, seedName string) bool {
	if managedSeed.Spec.Shoot == nil || managedSeed.Spec.Shoot.Name == "" {
		return false
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := reader.Get(ctx, kubernetesutils.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return false
	}

	specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(shoot)

	return gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) == seedName
}

// MapSeedToManagedSeed is a mapper.MapFunc for mapping a Seed to the owning ManagedSeed.
func (r *Reconciler) MapSeedToManagedSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: r.GardenNamespaceGarden, Name: obj.GetName()}}}
}

func reconcileRequest(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}

// RandomDurationWithMetaDuration is an alias for `utils.RandomDurationWithMetaDuration`. Exposed for unit tests.
var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random Jitter duration when the JitterUpdate
// is enabled in ManagedSeed controller configuration.
// All other events are normally enqueued.
func (r *Reconciler) EnqueueWithJitterDelay() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			managedSeed, ok := evt.Object.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			generationChanged := managedSeed.Generation != managedSeed.Status.ObservedGeneration

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.Object))
				return
			}

			if generationChanged {
				if *r.Config.Controllers.ManagedSeed.JitterUpdates {
					q.AddAfter(reconcileRequest(evt.Object), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
				} else {
					q.Add(reconcileRequest(evt.Object))
				}
				return
			}
			// Spread reconciliation of managed seeds (including gardenlet updates/rollouts) across the configured sync jitter
			// period to avoid overloading the gardener-apiserver if all gardenlets in all managed seeds are (re)starting
			// roughly at the same time.
			q.AddAfter(reconcileRequest(evt.Object), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
		},
		UpdateFunc: func(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
			managedSeed, ok := evt.ObjectNew.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			if managedSeed.Generation == managedSeed.Status.ObservedGeneration {
				return
			}

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.ObjectNew))
				return
			}

			if pointer.BoolDeref(r.Config.Controllers.ManagedSeed.JitterUpdates, false) {
				q.AddAfter(reconcileRequest(evt.ObjectNew), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
			} else {
				q.Add(reconcileRequest(evt.ObjectNew))
			}
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},
	}
}
