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

package heartbeat

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/extensions"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	client               client.Client
	extensionName        string
	renewIntervalSeconds int32
	namespace            string
	clock                clock.Clock
}

// NewReconciler creates a new reconciler that will renew the heart beat lease resource.
func NewReconciler(extensionName string, namespace string, renewIntervalSeconds int32, clock clock.Clock) reconcile.Reconciler {
	return &reconciler{
		extensionName:        extensionName,
		renewIntervalSeconds: renewIntervalSeconds,
		namespace:            namespace,
		clock:                clock,
	}
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

// Reconcile renews the heart beat lease resource.
func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensions.HeartBeatResourceName,
			Namespace: r.namespace,
		},
	}

	if err := r.client.Get(ctx, client.ObjectKeyFromObject(lease), lease); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: time.Duration(r.renewIntervalSeconds) * time.Second}, r.client.Create(ctx, r.reconcileHeartbeat(lease))
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: time.Duration(r.renewIntervalSeconds) * time.Second}, r.client.Update(ctx, r.reconcileHeartbeat(lease))
}

func (r *reconciler) reconcileHeartbeat(lease *coordinationv1.Lease) *coordinationv1.Lease {
	lease.Spec = coordinationv1.LeaseSpec{
		HolderIdentity:       &r.extensionName,
		LeaseDurationSeconds: &r.renewIntervalSeconds,
		RenewTime:            &metav1.MicroTime{Time: r.clock.Now().UTC()},
	}
	return lease
}
