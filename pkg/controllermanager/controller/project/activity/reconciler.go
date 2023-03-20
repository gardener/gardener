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

package activity

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
)

// Reconciler reconciles Projects and updates the lastActivityTimestamp in the status.
type Reconciler struct {
	Client client.Client
	Config config.ProjectControllerConfiguration
	Clock  clock.Clock
}

// Reconcile reconciles Projects and updates the lastActivityTimestamp in the status.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	project := &gardencorev1beta1.Project{}
	if err := r.Client.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	now := r.Clock.Now()
	log.Info("Updating Project's lastActivityTimestamp", "lastActivityTimestamp", now)

	patch := client.MergeFrom(project.DeepCopy())
	project.Status.LastActivityTimestamp = &metav1.Time{Time: now}
	if err := r.Client.Status().Patch(ctx, project, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating Project's lastActivityTimestamp: %w", err)
	}

	return reconcile.Result{}, nil
}
