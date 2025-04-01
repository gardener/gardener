// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	"context"
	"reflect"

	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerinstallation"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.GardenConfig == nil {
		r.GardenConfig = gardenCluster.GetConfig()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.HelmRegistry == nil {
		r.HelmRegistry = oci.NewHelmRegistry(r.GardenClient)
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ControllerInstallation.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(
			source.Kind[client.Object](gardenCluster.GetCache(),
				&gardencorev1beta1.ControllerInstallation{},
				&handler.EnqueueRequestForObject{},
				r.ControllerInstallationPredicate(),
				r.HelmTypePredicate(ctx, gardenCluster.GetClient())),
		).
		Complete(r)
}

// ControllerInstallationPredicate returns a predicate that evaluates to true in all cases except for 'Update' events.
// Here, it only returns true if the references change or the deletion timestamp gets set.
func (r *Reconciler) ControllerInstallationPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// enqueue on periodic cache resyncs
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return true
			}

			controllerInstallation, ok := e.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			oldControllerInstallation, ok := e.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			return (oldControllerInstallation.DeletionTimestamp == nil && controllerInstallation.DeletionTimestamp != nil) ||
				!reflect.DeepEqual(oldControllerInstallation.Spec.DeploymentRef, controllerInstallation.Spec.DeploymentRef) ||
				oldControllerInstallation.Spec.RegistrationRef.ResourceVersion != controllerInstallation.Spec.RegistrationRef.ResourceVersion ||
				oldControllerInstallation.Spec.SeedRef.ResourceVersion != controllerInstallation.Spec.SeedRef.ResourceVersion
		},
	}
}

// HelmTypePredicate is a predicate which checks whether the ControllerDeployment referenced in the
// ControllerInstallation has .type=helm.
func (r *Reconciler) HelmTypePredicate(ctx context.Context, reader client.Reader) predicate.Predicate {
	return &helmTypePredicate{
		ctx:    ctx,
		reader: reader,
	}
}

type helmTypePredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *helmTypePredicate) Create(e event.CreateEvent) bool   { return p.isResponsible(e.Object) }
func (p *helmTypePredicate) Update(e event.UpdateEvent) bool   { return p.isResponsible(e.ObjectNew) }
func (p *helmTypePredicate) Delete(e event.DeleteEvent) bool   { return p.isResponsible(e.Object) }
func (p *helmTypePredicate) Generic(e event.GenericEvent) bool { return p.isResponsible(e.Object) }

func (p *helmTypePredicate) isResponsible(obj client.Object) bool {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return false
	}

	if deploymentName := controllerInstallation.Spec.DeploymentRef; deploymentName != nil {
		controllerDeployment := &gardencorev1.ControllerDeployment{}
		if err := p.reader.Get(p.ctx, client.ObjectKey{Name: deploymentName.Name}, controllerDeployment); err != nil {
			return false
		}
		return controllerDeployment.Helm != nil
	}

	return false
}
