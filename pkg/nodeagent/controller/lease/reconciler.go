// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"context"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	APIReader            client.Reader
	LeaseDurationSeconds int32
	Namespace            string
	Clock                clock.Clock
}

// Reconcile renews the heartbeat lease resource.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.APIReader.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardenerutils.NodeAgentLeaseName(node.GetName()),
			Namespace: r.Namespace,
		},
	}

	op := controllerutil.OperationResultUpdated
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed reading lease %s: %w", client.ObjectKeyFromObject(lease), err)
		}
		op = controllerutil.OperationResultCreated
	}

	if err := controllerutil.SetControllerReference(node, lease, r.Client.Scheme()); err != nil {
		log.Error(err, "Unable to set controller reference for Lease", "lease", client.ObjectKeyFromObject(lease))
	}
	lease.Spec = coordinationv1.LeaseSpec{
		HolderIdentity:       &lease.Name,
		LeaseDurationSeconds: &r.LeaseDurationSeconds,
		RenewTime:            &metav1.MicroTime{Time: r.Clock.Now().UTC()},
	}

	if op == controllerutil.OperationResultCreated {
		if err := r.Client.Create(ctx, lease); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed creating lease %s: %w", client.ObjectKeyFromObject(lease), err)
		}
	} else {
		if err := r.Client.Update(ctx, lease); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed updating lease %s: %w", client.ObjectKeyFromObject(lease), err)
		}
	}

	log.V(1).Info("Heartbeat Lease", "lease", client.ObjectKeyFromObject(lease), "operation", op)
	return reconcile.Result{RequeueAfter: time.Duration(r.LeaseDurationSeconds) * time.Second / 4}, nil
}
