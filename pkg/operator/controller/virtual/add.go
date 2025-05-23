// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/virtual/access"
	virtualcluster "github.com/gardener/gardener/pkg/operator/controller/virtual/cluster"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

// AddToManagerFuncs returns all virtual garden cluster controllers for a registration via the controller registrar.
func AddToManagerFuncs(cfg *operatorconfigv1alpha1.OperatorConfiguration, storeCluster virtualcluster.StoreCluster) []controllerregistrar.Controller {
	var (
		channel                  = make(chan event.TypedGenericEvent[*rest.Config])
		virtualClusterReconciler = &virtualcluster.Reconciler{
			StoreCluster:            storeCluster,
			VirtualClientConnection: cfg.VirtualClientConnection,
		}
	)

	return []controllerregistrar.Controller{
		{
			Name: virtualcluster.ControllerName,
			AddToManagerFunc: func(_ context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
				return true, virtualClusterReconciler.AddToManager(mgr, channel)
			},
		},
		{
			Name: access.ControllerName,
			AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
				log := logf.FromContext(ctx)

				if !operator.IsGardenSuccessfullyReconciled(garden) {
					log.Info("Garden is still being reconciled, waiting for it to finish")
					return false, nil
				}

				return true, (&access.Reconciler{
					Channel: channel,
				}).AddToManager(mgr, v1beta1constants.GardenNamespace, v1beta1constants.SecretNameGardenerInternal)
			},
		},
	}
}
