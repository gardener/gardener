// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

// StoreCluster is a function that stores the given cluster object.
type StoreCluster func(cluster.Cluster)

// Request contains the information necessary to create the cluster.Cluster object for the virtual garden.
type Request struct {
	// RESTConfig is the rest.Config for the virtual garden cluster.
	RESTConfig *rest.Config
}

// Reconciler creates the cluster.Cluster object for the virtual garden cluster and adds it to the manager after it
// received the rest.Config via the Channel.
type Reconciler struct {
	Manager                 manager.Manager
	StoreCluster            StoreCluster
	VirtualClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration

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
		log.Info("Instantiating cluster.Cluster object for virtual cluster")
		kubernetesclient.ApplyClientConnectionConfigurationToRESTConfig(&r.VirtualClientConnection, restConfig)

		virtualCluster, err := cluster.New(restConfig, func(opts *cluster.Options) {
			opts.Scheme = operatorclient.VirtualScheme
			opts.Logger = r.Manager.GetLogger()
		})
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not instantiate virtual cluster: %w", err)
		}

		log.Info("Adding field indexes to informers")
		if err := addAllFieldIndexes(ctx, virtualCluster.GetFieldIndexer()); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed adding indexes: %w", err)
		}

		if err := r.Manager.Add(virtualCluster); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed adding virtual cluster to manager: %w", err)
		}

		r.virtualCluster = virtualCluster
		r.StoreCluster(virtualCluster)
		log.Info("Cluster object has been added to the manager")
	}

	return reconcile.Result{}, nil
}

func addAllFieldIndexes(ctx context.Context, i client.FieldIndexer) error {
	for _, fn := range []func(context.Context, client.FieldIndexer) error{
		indexer.AddControllerInstallationRegistrationRefName,
	} {
		if err := fn(ctx, i); err != nil {
			return err
		}
	}

	return nil
}
