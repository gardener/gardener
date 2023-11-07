// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// Reconcile executes all defined healtchecks
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconcile Healthchecks")

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		return reconcile.Result{}, err
	}

	var taskFns []flow.TaskFn
	for _, healthChecker := range r.HealthCheckers {
		f := healthChecker
		taskFns = append(taskFns, func(ctx context.Context) error { return f.Check(ctx, node.DeepCopy()) })
	}

	err := flow.Parallel(taskFns...)(ctx)

	return reconcile.Result{RequeueAfter: time.Duration(r.HealthCheckIntervalSeconds) * time.Second}, err
}
