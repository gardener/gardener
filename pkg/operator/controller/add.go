// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/controller/service"
	"github.com/gardener/gardener/pkg/controller/vpaevictionrequirements"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/extension"
	requiredruntime "github.com/gardener/gardener/pkg/operator/controller/extension/required/runtime"
	requiredvirtual "github.com/gardener/gardener/pkg/operator/controller/extension/required/virtual"
	"github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/operator/controller/gardenlet"
	"github.com/gardener/gardener/pkg/operator/controller/virtual"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(operatorCancel context.CancelFunc, mgr manager.Manager, cfg *operatorconfigv1alpha1.OperatorConfiguration, gardenClientMap clientmap.ClientMap) error {
	identity, err := gardenerutils.DetermineIdentity()
	if err != nil {
		return err
	}

	if err := garden.AddToManager(mgr, cfg, identity, gardenClientMap); err != nil {
		return err
	}

	if err := extension.AddToManager(mgr, cfg, gardenClientMap); err != nil {
		return err
	}

	var virtualCluster cluster.Cluster

	addVirtualClusterControllerToManager := virtual.AddToManagerFuncs(cfg, func(cluster cluster.Cluster) {
		virtualCluster = cluster
	})

	if err := (&controllerregistrar.Reconciler{
		OperatorCancel: operatorCancel,
		Controllers: append([]controllerregistrar.Controller{
			{
				Name: networkpolicy.ControllerName,
				AddToManagerFunc: func(_ context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
					return true, (&networkpolicy.Reconciler{
						ConcurrentSyncs:              cfg.Controllers.NetworkPolicy.ConcurrentSyncs,
						AdditionalNamespaceSelectors: cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
						RuntimeNetworks: networkpolicy.RuntimeNetworkConfig{
							// gardener-operator only supports IPv4 single-stack networking in the runtime cluster for now.
							IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
							Nodes:      garden.Spec.RuntimeCluster.Networking.Nodes,
							Pods:       garden.Spec.RuntimeCluster.Networking.Pods,
							Services:   garden.Spec.RuntimeCluster.Networking.Services,
							BlockCIDRs: garden.Spec.RuntimeCluster.Networking.BlockCIDRs,
						},
					}).AddToManager(mgr, mgr)
				},
			},
			{
				Name: vpaevictionrequirements.ControllerName,
				AddToManagerFunc: func(_ context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					return true, (&vpaevictionrequirements.Reconciler{
						ConcurrentSyncs: cfg.Controllers.VPAEvictionRequirements.ConcurrentSyncs,
					}).AddToManager(mgr, mgr)
				},
			},
			{
				Name: requiredruntime.ControllerName,
				AddToManagerFunc: func(_ context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					return true, (&requiredruntime.Reconciler{
						Config: cfg.Controllers.ExtensionRequiredRuntime,
					}).AddToManager(mgr)
				},
			},
			{
				Name: gardenlet.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					if virtualCluster == nil {
						logf.FromContext(ctx).Info("Virtual cluster object has not been created yet, cannot add Gardenlet reconciler")
						return false, nil
					}

					return true, (&gardenlet.Reconciler{
						Config: cfg.Controllers.GardenletDeployer,
					}).AddToManager(ctx, mgr, virtualCluster)
				},
			},
			{
				Name: requiredvirtual.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					if virtualCluster == nil {
						logf.FromContext(ctx).Info("Virtual cluster object has not been created yet, cannot add RequiredVirtual reconciler")
						return false, nil
					}

					return true, (&requiredvirtual.Reconciler{
						Config: cfg.Controllers.ExtensionRequiredVirtual,
					}).AddToManager(mgr, virtualCluster)
				},
			},
		}, addVirtualClusterControllerToManager...),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Registrar controller: %w", err)
	}

	if os.Getenv("GARDENER_OPERATOR_LOCAL") == "true" {
		virtualGardenIstioIngressPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: sharedcomponent.GetIstioZoneLabels(nil, nil)})
		if err != nil {
			return err
		}

		if err := (&service.Reconciler{IsMultiZone: true}).AddToManager(mgr, virtualGardenIstioIngressPredicate); err != nil {
			return fmt.Errorf("failed adding Service controller: %w", err)
		}
	}

	return nil
}
