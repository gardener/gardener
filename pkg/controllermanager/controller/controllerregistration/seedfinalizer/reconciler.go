// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedfinalizer

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Reconciler reconciles Seeds and manages the finalizer on these objects depending on whether ControllerInstallation
// objects exist in the system.
// It basically protects Seeds from being deleted, if there are still ControllerInstallations referencing it, to make
// sure we are able to cleanup ControllerInstallation objects of terminating Seeds.
type Reconciler struct {
	Client client.Client
}

// Reconcile reconciles Seeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if seed.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.Client.List(ctx, controllerInstallationList, client.MatchingFields{gardencore.SeedRefName: seed.Name}); err != nil {
			return reconcile.Result{}, err
		}

		if len(controllerInstallationList.Items) > 0 {
			return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of Seed %q because still found ControllerInstallations: %s", seed.Name, controllerutils.GetControllerInstallationNames(controllerInstallationList.Items))
		}

		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, seed, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, seed, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
