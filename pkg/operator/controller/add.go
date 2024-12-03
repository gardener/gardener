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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/controller/service"
	"github.com/gardener/gardener/pkg/controller/vpaevictionrequirements"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/extension"
	"github.com/gardener/gardener/pkg/operator/controller/extension/required"
	"github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/operator/controller/gardenlet"
	"github.com/gardener/gardener/pkg/operator/controller/virtual"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *config.OperatorConfiguration, gardenClientMap clientmap.ClientMap) error {
	identity, err := gardenerutils.DetermineIdentity()
	if err != nil {
		return err
	}

	if err := garden.AddToManager(ctx, mgr, cfg, identity, gardenClientMap); err != nil {
		return err
	}

	if err := extension.AddToManager(ctx, mgr, cfg, gardenClientMap); err != nil {
		return err
	}

	var virtualCluster cluster.Cluster

	addVirtualClusterControllerToManager := virtual.AddToManagerFuncs(cfg, func(cluster cluster.Cluster) {
		virtualCluster = cluster
	})

	if err := (&controllerregistrar.Reconciler{
		Controllers: append([]controllerregistrar.Controller{
			{
				Name: networkpolicy.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
					var nodes []string
					if garden.Spec.RuntimeCluster.Networking.Nodes != nil {
						nodes = []string{*garden.Spec.RuntimeCluster.Networking.Nodes}
					}

					return true, (&networkpolicy.Reconciler{
						ConcurrentSyncs:              cfg.Controllers.NetworkPolicy.ConcurrentSyncs,
						AdditionalNamespaceSelectors: cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
						RuntimeNetworks: networkpolicy.RuntimeNetworkConfig{
							// gardener-operator only supports IPv4 single-stack networking in the runtime cluster for now.
							IPFamilies: []gardencore.IPFamily{gardencore.IPFamilyIPv4},
							Nodes:      nodes,
							Pods:       []string{garden.Spec.RuntimeCluster.Networking.Pods},
							Services:   []string{garden.Spec.RuntimeCluster.Networking.Services},
							BlockCIDRs: garden.Spec.RuntimeCluster.Networking.BlockCIDRs,
						},
					}).AddToManager(ctx, mgr, mgr)
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
				Name: required.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					return true, (&required.Reconciler{
						Config: cfg,
					}).AddToManager(ctx, mgr)
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
