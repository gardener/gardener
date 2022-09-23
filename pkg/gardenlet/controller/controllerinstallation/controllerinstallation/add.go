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

package controllerinstallation

import (
	"context"
	"reflect"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerinstallation"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
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
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.ControllerInstallation{}, gardenCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		r.ControllerInstallationPredicate(),
		r.HelmTypePredicate(gardenCluster.GetClient()),
	)
}

// ControllerInstallationPredicate returns a predicate which evaluates to true in all cases except for 'update' events.
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
func (r *Reconciler) HelmTypePredicate(reader client.Reader) predicate.Predicate {
	return &helmTypePredicate{reader: reader}
}

type helmTypePredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *helmTypePredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
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
		controllerDeployment := &gardencorev1beta1.ControllerDeployment{}
		if err := p.reader.Get(p.ctx, kutil.Key(deploymentName.Name), controllerDeployment); err != nil {
			return false
		}
		return controllerDeployment.Type == "helm"
	}

	return false
}
