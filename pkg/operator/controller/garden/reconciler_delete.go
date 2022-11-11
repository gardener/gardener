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

package garden

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
) (
	reconcile.Result,
	error,
) {
	log.Info("Destroying gardener-resource-manager")
	gardenerResourceManager, err := r.newGardenerResourceManager(garden, secretsManager)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err := component.OpDestroyAndWait(gardenerResourceManager).Destroy(ctx); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Cleaning up secrets")
	if err := secretsManager.Cleanup(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if vpaEnabled(garden.Spec.RuntimeCluster.Settings) {
		log.Info("Destroying custom resource definition for VPA")
		applier := kubernetes.NewApplier(r.RuntimeClient, r.RuntimeClient.RESTMapper())

		if err := vpa.NewCRD(applier, nil).Destroy(ctx); err != nil {
			return reconcile.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(garden, finalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClient, garden, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
