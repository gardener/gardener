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

package exposureclass

import (
	"context"
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewReconciler creates a new instance of a reconciler which reconciles ExposureClass.
func NewReconciler(l logr.Logger, gardenClient client.Client, recorder record.EventRecorder) *exposureClassReconciler {
	return &exposureClassReconciler{
		logger:       l,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type exposureClassReconciler struct {
	logger       logr.Logger
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *exposureClassReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("exposureclass", request)

	exposureClass := &gardencorev1alpha1.ExposureClass{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, exposureClass); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	if exposureClass.DeletionTimestamp != nil {
		return r.delete(ctx, exposureClass, logger)
	}

	return r.reconcile(ctx, exposureClass, logger)
}

func (r *exposureClassReconciler) reconcile(ctx context.Context, exposureClass *gardencorev1alpha1.ExposureClass, logger logr.Logger) (reconcile.Result, error) {
	if err := controllerutils.PatchAddFinalizers(ctx, r.gardenClient, exposureClass, gardencorev1alpha1.GardenerName); err != nil {
		r.logger.Error(err, "Could not add finalizer to ExposureClass")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *exposureClassReconciler) delete(ctx context.Context, exposureClass *gardencorev1alpha1.ExposureClass, logger logr.Logger) (reconcile.Result, error) {
	// Ignore the exposure class if it has no gardener finalizer.
	if !sets.NewString(exposureClass.Finalizers...).Has(gardencorev1alpha1.GardenerName) {
		return reconcile.Result{}, nil
	}

	// Lookup shoots which reference the exposure class. The finalizer will be only
	// removed if there is no Shoot referencing the exposure class anymore.
	associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.gardenClient, exposureClass)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(associatedShoots) == 0 {
		logger.Info("No Shoots are referenced to the ExposureClass, deletion accepted.", exposureClass.Name)
		if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, exposureClass, gardencorev1alpha1.GardenerName); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Could not remove finalizer from ExposureClass")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	message := fmt.Sprintf("Can't delete ExposureClasss because it is still associated by the following Shoots: %+v", associatedShoots)
	logger.Info(message)
	r.recorder.Event(exposureClass, corev1.EventTypeNormal, v1alpha1constants.EventResourceReferenced, message)

	return reconcile.Result{}, fmt.Errorf("ExposureClass %q still has references", exposureClass.Name)
}
