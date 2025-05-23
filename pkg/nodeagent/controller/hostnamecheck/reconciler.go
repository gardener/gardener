// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hostnamecheck

import (
	"context"
	"fmt"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/nodeagent"
)

// Reconciler checks periodically whether the hostname changed. If yes, it calls the cancel func. This is required
// because gardener-node-agent uses a label selector for kubernetes.io/hostname=<hostname> which no longer works in case
// the hostname of the node has changed. Calling the cancel func leads to terminating (and eventually restarting) the
// gardener-node-agent such that it can fetch the hostname again during start-up.
type Reconciler struct {
	CancelContext context.CancelFunc
	HostName      string
}

// Reconcile checks periodically whether the hostname changed. If yes, it calls the cancel func.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed fetching hostname: %w", err)
	}

	if hostName != r.HostName {
		log.Info("Hostname has changed, calling the cancel func to trigger a restart of gardener-node-agent", "from", r.HostName, "to", hostName)
		r.CancelContext()
		return reconcile.Result{}, nil
	}

	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}
