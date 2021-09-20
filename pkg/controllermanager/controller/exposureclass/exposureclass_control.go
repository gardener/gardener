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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) exposureClassAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.exposureClassQueue.Add(key)
}

func (c *Controller) exposureClassUpdate(_, newObj interface{}) {
	c.exposureClassAdd(newObj)
}

func (c *Controller) exposureClassDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.exposureClassQueue.Add(key)
}

// NewExposureClassReconciler creates a new instance of a reconciler which reconciles ExposureClass.
func NewExposureClassReconciler(l logrus.FieldLogger, gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &exposureClassReconciler{
		logger:       l,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type exposureClassReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *exposureClassReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	exposureClass := &gardencorev1alpha1.ExposureClass{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, exposureClass); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	if exposureClass.DeletionTimestamp != nil {
		return r.delete(ctx, exposureClass)
	}

	return r.reconcile(ctx, exposureClass)
}

func (r *exposureClassReconciler) reconcile(ctx context.Context, exposureClass *gardencorev1alpha1.ExposureClass) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(exposureClass, gardencorev1beta1.GardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, exposureClass, gardencorev1alpha1.GardenerName); err != nil {
			r.logger.Errorf("could not add finalizer to ExposureClass: %s", err.Error())
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *exposureClassReconciler) delete(ctx context.Context, exposureClass *gardencorev1alpha1.ExposureClass) (reconcile.Result, error) {
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
		r.logger.Infof("No Shoots are referenced to the ExposureClass %q. Deletion accepted.", exposureClass.Name)
		if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, exposureClass, gardencorev1alpha1.GardenerName); client.IgnoreNotFound(err) != nil {
			r.logger.Errorf("could not remove finalizer from ExposureClass: %s", err.Error())
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	message := fmt.Sprintf("Can't delete ExposureClasss because it is still associated by the following Shoots: %+v", associatedShoots)
	r.logger.Info(message)
	r.recorder.Event(exposureClass, corev1.EventTypeNormal, v1alpha1constants.EventResourceReferenced, message)

	return reconcile.Result{}, fmt.Errorf("ExposureClass %q still has references", exposureClass.Name)
}
