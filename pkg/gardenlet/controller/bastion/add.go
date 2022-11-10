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

package bastion

import (
	"context"
	"strings"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "bastion-controller"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
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
		source.NewKindWithCache(&operationsv1alpha1.Bastion{}, gardenCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		predicate.GenerationChangedPredicate{},
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.Bastion{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapExtensionsBastionToOperationsBastion), mapper.UpdateWithNew, c.GetLogger()),
		predicateutils.ExtensionStatusChanged(),
	)
}

// MapExtensionsBastionToOperationsBastion  is a mapper.MapFunc for mapping extensions Bastion in the seed cluster to operations Bastion in the project namespace.
func (r *Reconciler) MapExtensionsBastionToOperationsBastion(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	extensionsBastion, ok := obj.(*extensionsv1alpha1.Bastion)
	if !ok {
		return nil
	}

	projectNamespaceName := "garden-" + GetProjectNameFromTechincalId(extensionsBastion.Namespace)

	operationsBastion := &operationsv1alpha1.Bastion{}
	if err := reader.Get(ctx, kutil.Key(projectNamespaceName, extensionsBastion.Name), operationsBastion); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get operations Bastion for extensions Bastion", "extensionsBastion", client.ObjectKeyFromObject(extensionsBastion))
		}
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: operationsBastion.Namespace, Name: operationsBastion.Name}}}
}

// GetProjectNameFromTechincalId returns Shoot resource name from its UID.
func GetProjectNameFromTechincalId(shootTechnicalID string) string {
	tokens := strings.Split(shootTechnicalID, "--")
	projectName := tokens[len(tokens)-2]
	return projectName
}
