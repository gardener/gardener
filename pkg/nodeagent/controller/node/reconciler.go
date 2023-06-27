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

package node

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const restartSystemdUnitAnnotation = "worker.gardener.cloud/restart-systemd-services"

// Reconciler checks for node annotation changes and restart the specified services
type Reconciler struct {
	Client     client.Client
	Recorder   record.EventRecorder
	SyncPeriod time.Duration
	NodeName   string
	Dbus       dbus.Dbus
}

// Reconcile checks for node annotation changes and restart the specified services
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if unit, ok := node.Annotations[restartSystemdUnitAnnotation]; ok {
		if err := r.Dbus.Restart(ctx, r.Recorder, node, unit); err != nil {
			return reconcile.Result{}, err
		}
		log.V(1).Info("Successfully restarted service", "service", unit)

		delete(node.Annotations, restartSystemdUnitAnnotation)
		if err := r.Client.Update(ctx, node); err != nil {
			return reconcile.Result{}, err
		}
	}

	log.V(1).Info("Requeuing", "requeueAfter", r.SyncPeriod)
	return reconcile.Result{RequeueAfter: r.SyncPeriod}, nil
}
