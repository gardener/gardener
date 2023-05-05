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

package networkpolicyregistrar

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/operator/apis/config"
)

// Reconciler adds the NetworkPolicy controller to the manager.
type Reconciler struct {
	Manager manager.Manager
	Config  config.NetworkPolicyControllerConfiguration

	added bool
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	if r.added {
		return reconcile.Result{}, nil
	}

	garden := &operatorv1alpha1.Garden{}
	if err := r.Manager.GetClient().Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := (&networkpolicy.Reconciler{
		ConcurrentSyncs: r.Config.ConcurrentSyncs,
		RuntimeNetworks: networkpolicy.RuntimeNetworkConfig{
			Nodes:      garden.Spec.RuntimeCluster.Networking.Nodes,
			Pods:       garden.Spec.RuntimeCluster.Networking.Pods,
			Services:   garden.Spec.RuntimeCluster.Networking.Services,
			BlockCIDRs: garden.Spec.RuntimeCluster.Networking.BlockCIDRs,
		},
	}).AddToManager(r.Manager, r.Manager); err != nil {
		return reconcile.Result{}, err
	}

	r.added = true
	return reconcile.Result{}, nil
}
