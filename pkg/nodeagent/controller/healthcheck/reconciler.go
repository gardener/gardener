// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Reconciler checks for containerd and kubelet health and restarts them if required.
type Reconciler struct {
	Client                     client.Client
	Recorder                   record.EventRecorder
	DBus                       dbus.DBus
	HealthCheckers             []HealthChecker
	HealthCheckIntervalSeconds int32
}

// Reconcile executes all defined health checks.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.V(2).Info("Performing health checks")

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	var taskFns []flow.TaskFn
	for _, healthChecker := range r.HealthCheckers {
		f := healthChecker

		taskFns = append(taskFns, func(ctx context.Context) error { return f.Check(ctx, node.DeepCopy()) })
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: time.Duration(r.HealthCheckIntervalSeconds) * time.Second}, nil
}
