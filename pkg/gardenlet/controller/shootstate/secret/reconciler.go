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

package secret

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const finalizerName = "gardenlet.gardener.cloud/secret-controller"

// Reconciler reconciles secrets in seed.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       config.ShootSecretControllerConfiguration
}

// Reconcile reconciles `Secret`s having labels `managed-by=secrets-manager` and `persist=true` in
// the shoot namespaces in the seed cluster.
// It syncs them to the `ShootState` so that the secrets can be restored from there in case a shoot
// control plane has to be restored to another seed cluster (in case of migration).
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.SeedClient.Get(ctx, request.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	namespace := &corev1.Namespace{}
	if err := r.SeedClient.Get(ctx, kubernetesutils.Key(secret.Namespace), namespace); err != nil {
		return reconcile.Result{}, err
	}
	if namespace.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleShoot {
		return reconcile.Result{}, nil
	}

	shootState, shoot, err := extensions.GetShootStateForCluster(ctx, r.GardenClient, r.SeedClient, secret.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if controllerutil.ContainsFinalizer(secret, finalizerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.SeedClient, secret, finalizerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if secret.DeletionTimestamp != nil {
		return r.delete(ctx, log, secret, shootState, shoot)
	}
	return r.reconcile(ctx, log, secret, shootState)
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	secret *corev1.Secret,
	shootState *gardencorev1beta1.ShootState,
) (
	reconcile.Result,
	error,
) {
	log.Info("Reconciling secret information in ShootState and ensuring its finalizer")

	if !controllerutil.ContainsFinalizer(secret, finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.SeedClient, secret, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	dataJSON, err := json.Marshal(secret.Data)
	if err != nil {
		return reconcile.Result{}, err
	}

	patch := client.StrategicMergeFrom(shootState.DeepCopy())

	dataList := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)
	dataList.Upsert(&gardencorev1beta1.GardenerResourceData{
		Name:   secret.Name,
		Labels: secret.Labels,
		Type:   "secret",
		Data:   runtime.RawExtension{Raw: dataJSON},
	})

	// If the data list does not change, do not even try to send an empty PATCH request.
	if apiequality.Semantic.DeepEqual(shootState.Spec.Gardener, dataList) {
		return reconcile.Result{}, nil
	}

	shootState.Spec.Gardener = dataList
	return reconcile.Result{}, r.GardenClient.Patch(ctx, shootState, patch)
}

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	secret *corev1.Secret,
	shootState *gardencorev1beta1.ShootState,
	shoot *gardencorev1beta1.Shoot,
) (
	reconcile.Result,
	error,
) {
	if lastOp := shoot.Status.LastOperation; lastOp != nil && lastOp.Type == gardencorev1beta1.LastOperationTypeMigrate {
		log.Info("Keeping Secret in ShootState since Shoot is in migration but releasing the finalizer")
	} else {
		log.Info("Removing Secret from ShootState and releasing its finalizer")

		patch := client.StrategicMergeFrom(shootState.DeepCopy())

		dataList := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)
		dataList.Delete(secret.Name)
		shootState.Spec.Gardener = dataList

		if err := r.GardenClient.Patch(ctx, shootState, patch); err != nil {
			return reconcile.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(secret, finalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.SeedClient, secret, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
