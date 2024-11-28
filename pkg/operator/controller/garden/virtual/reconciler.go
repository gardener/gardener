// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

// Request contains the information necessary to create the cluster.Cluster object for the virtual garden.
type Request struct {
	// RESTConfig is the rest.Config for the virtual garden cluster.
	RESTConfig *rest.Config
}

// Reconciler creates the cluster.Cluster object for the virtual garden cluster and adds it to the manager after it
// received the rest.Config via the Channel.
type Reconciler struct {
	Channel                 <-chan event.TypedGenericEvent[*rest.Config]
	Manager                 manager.Manager
	VirtualClientConnection componentbaseconfig.ClientConnectionConfiguration

	virtualCluster cluster.Cluster
}

// Reconcile creates the cluster.Cluster object for the virtual garden cluster and adds it to the manager after it
// received the rest.Config via the Channel.
func (r *Reconciler) Reconcile(ctx context.Context, request Request) (reconcile.Result, error) {
	var (
		log        = logf.FromContext(ctx)
		restConfig = rest.CopyConfig(request.RESTConfig)
	)

	if r.virtualCluster == nil {
		kubernetesclient.ApplyClientConnectionConfigurationToRESTConfig(&r.VirtualClientConnection, restConfig)

		virtualCluster, err := cluster.New(restConfig, func(opts *cluster.Options) {
			opts.Scheme = operatorclient.VirtualScheme
			opts.Logger = log
		})
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not instantiate virtual cluster: %w", err)
		}

		if err := r.Manager.Add(virtualCluster); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed adding virtual cluster to manager: %w", err)
		}

		r.virtualCluster = virtualCluster
	}

	return reconcile.Result{}, nil
}

// GetVirtualCluster returns the virtual cluster object.
func (r *Reconciler) GetVirtualCluster() cluster.Cluster {
	return r.virtualCluster
}
