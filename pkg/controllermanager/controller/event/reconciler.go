// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles Event.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.EventControllerConfiguration
	Clock  clock.Clock
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	event := &corev1.Event{}
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if isShootEvent(event) {
		return reconcile.Result{}, nil
	}

	deleteAt := event.LastTimestamp.Add(r.Config.TTLNonShootEvents.Duration)
	timeUntilDeletion := deleteAt.Sub(r.Clock.Now())
	if timeUntilDeletion > 0 {
		return reconcile.Result{RequeueAfter: timeUntilDeletion}, nil
	}

	return reconcile.Result{}, r.Client.Delete(ctx, event)
}

func isShootEvent(event *corev1.Event) bool {
	if gv, err := schema.ParseGroupVersion(event.InvolvedObject.APIVersion); err != nil || gv.Group != gardencorev1beta1.GroupName {
		return false
	}
	if event.InvolvedObject.Kind == "Shoot" {
		return true
	}
	return false
}
