// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/virtual/access"
	virtualcluster "github.com/gardener/gardener/pkg/operator/controller/virtual/cluster"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds all virtual garden cluster controllers to the given manager (via the controller registrar).
func AddToManager(cfg *config.OperatorConfiguration) ([]controllerregistrar.Controller, func() cluster.Cluster) {
	var (
		channel                  = make(chan event.TypedGenericEvent[*rest.Config])
		virtualClusterReconciler = &virtualcluster.Reconciler{
			Channel:                 channel,
			VirtualClientConnection: cfg.VirtualClientConnection,
		}
	)

	return []controllerregistrar.Controller{
		{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
			return true, virtualClusterReconciler.AddToManager(mgr)
		}},
		{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
			log := logf.FromContext(ctx)

			if !gardenerutils.IsGardenSuccessfullyReconciled(garden) {
				log.Info("Garden is being reconciled - adding the access reconciler will be tried again")
				return false, nil
			}

			return true, (&access.Reconciler{
				Channel: channel,
			}).AddToManager(mgr, v1beta1constants.GardenNamespace, clientmap.GardenerSecretName(log, v1beta1constants.GardenNamespace))
		}},
	}, virtualClusterReconciler.GetVirtualCluster
}
