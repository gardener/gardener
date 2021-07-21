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
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewSeedReconciler creates a new instance of a reconciler which reconciles Seeds.
func NewSeedReconciler(logger logr.Logger, gardenClient client.Client) reconcile.Reconciler {
	return &seedReconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type seedReconciler struct {
	logger       logr.Logger
	gardenClient client.Client
}

func (r *seedReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("seed", request.Name)

	seed := &gardencorev1beta1.Seed{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	if seed.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.gardenClient.List(ctx, controllerInstallationList); err != nil {
			return reconcile.Result{}, err
		}

		for _, controllerInstallation := range controllerInstallationList.Items {
			if controllerInstallation.Spec.SeedRef.Name == seed.Name {
				return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of Seed %q because still found at least one ControllerInstallation", seed.Name)
			}
		}

		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, seed, FinalizerName)
	}

	return reconcile.Result{}, controllerutils.PatchAddFinalizers(ctx, r.gardenClient, seed, FinalizerName)
}
