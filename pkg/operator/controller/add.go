// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/gardener/gardener/pkg/operator/controller/garden"
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

	if err := (&extension.Reconciler{
		Config: *cfg,
	}).AddToManager(ctx, mgr, gardenClientMap); err != nil {
		return fmt.Errorf("failed adding Extension controller: %w", err)
	}

	if err := (&controllerregistrar.Reconciler{
		Controllers: []controllerregistrar.Controller{
			{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) error {
				var nodes []string
				if garden.Spec.RuntimeCluster.Networking.Nodes != nil {
					nodes = []string{*garden.Spec.RuntimeCluster.Networking.Nodes}
				}

				return (&networkpolicy.Reconciler{
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
			}},
			{AddToManagerFunc: func(_ context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) error {
				return (&vpaevictionrequirements.Reconciler{
					ConcurrentSyncs: cfg.Controllers.VPAEvictionRequirements.ConcurrentSyncs,
				}).AddToManager(mgr, mgr)
			}},
		},
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding NetworkPolicy Registrar controller: %w", err)
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
