// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"context"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler creates a lease in the kube-system namespace of the shoot.
type Reconciler struct {
	Client               client.Client
	LeaseDurationSeconds int32
	Namespace            string
	Clock                clock.Clock
}

// Reconcile renews the heartbeat lease resource.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.NodeAgentLeaseName(node.GetName()),
			Namespace: r.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, lease, func() error {
		if err := controllerutil.SetControllerReference(node, lease, r.Client.Scheme()); err != nil {
			log.Error(err, "Unable to set controller reference for Lease", "lease", client.ObjectKeyFromObject(lease))
		}

		lease.Spec = coordinationv1.LeaseSpec{
			HolderIdentity:       &lease.Name,
			LeaseDurationSeconds: &r.LeaseDurationSeconds,
			RenewTime:            &metav1.MicroTime{Time: r.Clock.Now().UTC()},
		}
		return nil
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	log.V(1).Info("Heartbeat Lease", "lease", client.ObjectKeyFromObject(lease), "operation", op)
	return reconcile.Result{RequeueAfter: time.Duration(r.LeaseDurationSeconds) * time.Second / 4}, nil
}
