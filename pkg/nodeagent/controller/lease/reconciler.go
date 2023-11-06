// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package lease

import (
	"context"
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
)

// Reconciler creates a lease in the kube-system namespace of the shoot.
type Reconciler struct {
	Client               client.Client
	RenewIntervalSeconds int32
	NodeName             string
	Namespace            string
	Clock                clock.Clock
}

// Reconcile renews the heartbeat lease resource.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	node := &metav1.PartialObjectMetadata{}
	node.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener-node-agent-" + node.GetName(),
			Namespace: r.Namespace,
		},
	}
	leaseSpec := coordinationv1.LeaseSpec{
		HolderIdentity:       &lease.Name,
		LeaseDurationSeconds: &r.RenewIntervalSeconds,
		RenewTime:            &metav1.MicroTime{Time: r.Clock.Now().UTC()},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
		if apierrors.IsNotFound(err) {
			lease.Spec = leaseSpec
			if err := controllerutil.SetControllerReference(node, lease, r.Client.Scheme()); err != nil {
				return reconcile.Result{}, err
			}
			log.V(1).Info("Creating heartbeat Lease", "lease", client.ObjectKeyFromObject(lease))
			return reconcile.Result{RequeueAfter: time.Duration(r.RenewIntervalSeconds) * time.Second}, r.Client.Create(ctx, lease)
		}
		return reconcile.Result{}, err
	}

	lease.Spec = leaseSpec

	log.V(1).Info("Renewing heartbeat Lease", "lease", client.ObjectKeyFromObject(lease))
	return reconcile.Result{
		RequeueAfter: time.Duration(r.RenewIntervalSeconds) * time.Second / 3,
	}, r.Client.Update(ctx, lease)
}
