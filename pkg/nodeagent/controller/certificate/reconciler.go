// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/nodeagent"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler checks when the certificate of gardener-node-agent expires and requests a new one in this case.
// When the certificate is renewed it saves the resulting kubeconfig on the disk, cancels its context to initiate a
// restart of gardener-node-agent.
type Reconciler struct {
	Cancel      context.CancelFunc
	Clock       clock.Clock
	FS          afero.Afero
	Config      *rest.Config
	MachineName string

	renewalDeadline *time.Time
}

// Reconcile requests a new certificate when the actual one is expiring. Then, it calls the cancel func.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	if r.renewalDeadline == nil {
		clientCertificate, err := kubernetesutils.ClientCertificateFromRESTConfig(r.Config)
		if err != nil {
			return reconcile.Result{}, err
		}

		totalDuration := float64(clientCertificate.Leaf.NotAfter.Sub(clientCertificate.Leaf.NotBefore))
		jitteredDuration := wait.Jitter(time.Duration(totalDuration*0.8), 0.1)
		r.renewalDeadline = ptr.To(clientCertificate.Leaf.NotBefore.Add(jitteredDuration))
		log.Info("Scheduling certificate renewal", "time", *r.renewalDeadline)
	}

	if r.Clock.Now().After(*r.renewalDeadline) {
		log.Info("Start rotating client certificate because renewal deadline exceeded", "renewalDeadline", *r.renewalDeadline)

		// Always read the gardener-node-agent config for the latest API server CA bundle.
		// This is needed to prevent races between this reconciler and the osc reconciler during in-place certificate rotation.
		// Otherwise, if the renewal deadline is reached right after the osc reconciler requests a new certificate, but node-agent has
		// not restarted yet, this reconciler could use the old CA bundle to request a new certificate. This could lead to osc reconciler
		// marking the certificate rotation as complete, while the persisted certificate is still using the old CA bundle.
		apiServerConfig, err := nodeagent.GetAPIServerConfig(r.FS)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed reading the API server config: %w", err)
		}
		r.Config.CAData = apiServerConfig.CABundle

		if err := nodeagent.RequestAndStoreKubeconfig(ctx, log, r.FS, r.Config, r.MachineName); err != nil {
			return reconcile.Result{}, fmt.Errorf("error rotating certificate: %w", err)
		}

		log.Info("Certificate rotation complete. Restarting gardener-node-agent")
		r.Cancel()
		return reconcile.Result{}, nil
	}

	return reconcile.Result{RequeueAfter: r.renewalDeadline.Sub(r.Clock.Now())}, nil
}
