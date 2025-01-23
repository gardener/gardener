// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles Projects and updates the lastActivityTimestamp in the status.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ProjectControllerConfiguration
	Clock  clock.Clock
}

// Reconcile reconciles Projects and updates the lastActivityTimestamp in the status.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

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
