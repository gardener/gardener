// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package heartbeat

import (
	"context"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/extensions"
)

type reconciler struct {
	client               client.Client
	extensionName        string
	renewIntervalSeconds int32
	namespace            string
	clock                clock.Clock
}

// NewReconciler creates a new reconciler that will renew the heartbeat lease resource.
func NewReconciler(mgr manager.Manager, extensionName string, namespace string, renewIntervalSeconds int32, clock clock.Clock) reconcile.Reconciler {
	return &reconciler{
		client:               mgr.GetClient(),
		extensionName:        extensionName,
		renewIntervalSeconds: renewIntervalSeconds,
		namespace:            namespace,
		clock:                clock,
	}
}

// Reconcile renews the heartbeat lease resource.
func (r *reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensions.HeartBeatResourceName,
			Namespace: r.namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &r.extensionName,
			LeaseDurationSeconds: &r.renewIntervalSeconds,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.client, lease, func() error {
		lease.Spec.RenewTime = &metav1.MicroTime{Time: r.clock.Now().UTC()}
		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	log.V(1).Info("Heartbeat Lease", "lease", client.ObjectKeyFromObject(lease), "operation", op)
	// Ensure we update the lease much sooner to account for possible controller lag
	return reconcile.Result{RequeueAfter: time.Duration(r.renewIntervalSeconds) * time.Second}, nil
}
