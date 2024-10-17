// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	gardenaccess "github.com/gardener/gardener/pkg/operator/controller/garden/access"
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

	if err := (&controllerregistrar.Reconciler{
		Controllers: []controllerregistrar.Controller{
			{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
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
			}},
			{AddToManagerFunc: func(_ context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
				return true, (&vpaevictionrequirements.Reconciler{
					ConcurrentSyncs: cfg.Controllers.VPAEvictionRequirements.ConcurrentSyncs,
				}).AddToManager(mgr, mgr)
			}},
			{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
				return true, (&required.Reconciler{
					Config: cfg,
				}).AddToManager(ctx, mgr)
			}},
			{AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
				log := logf.FromContext(ctx)
				secretName := v1beta1constants.SecretNameGardener

				// Prefer the internal host if available
				addr, err := net.LookupHost(fmt.Sprintf("virtual-garden-%s.%s.svc.cluster.local", v1beta1constants.DeploymentNameKubeAPIServer, v1beta1constants.GardenNamespace))
				if len(addr) == 0 && !gardenerutils.IsGardenSuccessfullyReconciled(garden) {
					log.Info("Service DNS name lookup of virtual-garden-kube-apiserver is tried again because garden is still being created")
					return false, nil
				} else if err != nil {
					log.Info("Service DNS name lookup of virtual-garden-kube-apiserver failed, falling back to external kubeconfig", "error", err)
				} else {
					log.Info("Service DNS name lookup of virtual-garden-kube-apiserver successfull, using internal kubeconfig", "error", err)
					secretName = v1beta1constants.SecretNameGardenerInternal
				}

				return true, (&gardenaccess.Reconciler{
					Config: cfg,
				}).AddToManager(mgr, v1beta1constants.GardenNamespace, secretName)
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
