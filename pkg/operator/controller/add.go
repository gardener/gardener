// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/api/indexer"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/operator/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/controller/tokenrequestor"
	"github.com/gardener/gardener/pkg/controller/vpaevictionrequirements"
	"github.com/gardener/gardener/pkg/operator/controller/controllerregistrar"
	"github.com/gardener/gardener/pkg/operator/controller/extension"
	"github.com/gardener/gardener/pkg/operator/controller/extension/care"
	"github.com/gardener/gardener/pkg/operator/controller/extension/reference"
	requiredruntime "github.com/gardener/gardener/pkg/operator/controller/extension/required/runtime"
	requiredvirtual "github.com/gardener/gardener/pkg/operator/controller/extension/required/virtual"
	"github.com/gardener/gardener/pkg/operator/controller/garden"
	"github.com/gardener/gardener/pkg/operator/controller/gardenlet"
	"github.com/gardener/gardener/pkg/operator/controller/virtual"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
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

	if err := reference.AddToManager(mgr, cfg.Controllers.ExtensionReference); err != nil {
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
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
					if !gardenIsReady(virtualCluster, garden) {
						logf.FromContext(ctx).Info("Garden cluster is not ready yet, cannot add Gardenlet reconciler")
						return false, nil
					}

					return true, (&gardenlet.Reconciler{
						Config: cfg.Controllers.GardenletDeployer,
						// garden.Spec.VirtualCluster.DNS.Domains[0].Name is immutable and always set.
						DefaultGardenClusterAddress: fmt.Sprintf("https://%s", v1beta1helper.GetAPIServerDomain(garden.Spec.VirtualCluster.DNS.Domains[0].Name)),
					}).AddToManager(ctx, mgr, virtualCluster)
				},
			},
			{
				Name: requiredvirtual.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
					log := logf.FromContext(ctx)
					if !gardenIsReady(virtualCluster, garden) {
						logf.FromContext(ctx).Info("Garden cluster is not ready yet, cannot add RequiredVirtual reconciler")
						return false, nil
					}

					log.Info("Adding ControllerInstallation field index to informers")
					if err := indexer.AddControllerInstallationRegistrationRefName(ctx, virtualCluster.GetFieldIndexer()); err != nil {
						return false, err
					}

					return true, (&requiredvirtual.Reconciler{
						Config: cfg.Controllers.ExtensionRequiredVirtual,
					}).AddToManager(mgr, virtualCluster)
				},
			},
			{
				Name: care.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, garden *operatorv1alpha1.Garden) (bool, error) {
					if !gardenIsReady(virtualCluster, garden) {
						logf.FromContext(ctx).Info("Garden cluster is not ready yet, cannot add Care reconciler")
						return false, nil
					}

					return true, (&care.Reconciler{
						Config: *cfg,
					}).AddToManager(mgr, virtualCluster)
				},
			},
			{
				Name: tokenrequestor.ControllerName,
				AddToManagerFunc: func(ctx context.Context, mgr manager.Manager, _ *operatorv1alpha1.Garden) (bool, error) {
					isSelfHostedShootCluster, err := gardenerutils.ClusterIsSelfHostedShoot(ctx, mgr.GetAPIReader())
					if err != nil {
						return false, fmt.Errorf("failed checking whether the cluster is a self-hosted shoot cluster: %w", err)
					}
					// The Gardenlet of
					if isSelfHostedShootCluster {
						logf.FromContext(ctx).Info("Garden cluster is a self-hosted shoot cluster, skip adding TokenRequestor reconciler")
						return true, nil
					}

					if virtualCluster == nil {
						logf.FromContext(ctx).Info("Virtual cluster object has not been created yet, cannot add TokenRequestor reconciler")
						return false, nil
					}

					return true, (&tokenrequestor.Reconciler{
						ConcurrentSyncs:        ptr.Deref(cfg.Controllers.TokenRequestor.ConcurrentSyncs, 0),
						APIAudiences:           []string{v1beta1constants.GardenerAudience},
						Class:                  new(resourcesv1alpha1.ResourceManagerClassGarden),
						TargetDefaultNamespace: TargetNamespaceForTokenRequestorController(),
					}).AddToManager(mgr, mgr, virtualCluster)
				},
			},
		}, addVirtualClusterControllerToManager...),
	}).AddToManager(mgr); err != nil {
		return fmt.Errorf("failed adding Registrar controller: %w", err)
	}

	return nil
}

// TargetNamespaceForTokenRequestorController returns a function that determines the target namespace for the TokenRequestor controller based on whether the cluster is a seed cluster or not.\
// If it is a seed cluster, it computes the garden namespace based on the seed name; otherwise, it returns the default garden namespace.
func TargetNamespaceForTokenRequestorController() tokenrequestor.TargetDefaultNamespaceFn {
	var (
		mu       sync.Mutex
		seedName string
	)

	return func(ctx context.Context, c client.Client) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		isSeedCluster, err := gardenletutils.ClusterIsSeed(ctx, c)
		if err != nil {
			return "", fmt.Errorf("failed to determine if the cluster is a seed cluster: %w", err)
		}
		if !isSeedCluster {
			// Cluster is not a seed cluster, so the target namespace is the garden namespace.
			seedName = ""
			return v1beta1constants.GardenNamespace, nil
		}

		if seedName != "" {
			return gardenerutils.ComputeGardenNamespace(seedName), nil
		}

		seedName, err = extractSeedNameFromGardenlet(ctx, c)
		if err != nil {
			return "", err
		}
		return gardenerutils.ComputeGardenNamespace(seedName), nil
	}
}

func extractSeedNameFromGardenlet(ctx context.Context, c client.Client) (string, error) {
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenlet, Namespace: v1beta1constants.GardenNamespace}}
	if err := c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
		return "", fmt.Errorf("failed reading gardenlet deployment: %w", err)
	}

	configMapVolumeIndex := slices.IndexFunc(deployment.Spec.Template.Spec.Volumes, func(volume corev1.Volume) bool {
		return volume.Name == gardenletdeployer.GardenletConfigVolumeName
	})
	if configMapVolumeIndex < 0 || deployment.Spec.Template.Spec.Volumes[configMapVolumeIndex].ConfigMap == nil {
		return "", fmt.Errorf("gardenlet deployment has no volume named %q with a ConfigMap source", gardenletdeployer.GardenletConfigVolumeName)
	}
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: deployment.Spec.Template.Spec.Volumes[configMapVolumeIndex].ConfigMap.Name, Namespace: v1beta1constants.GardenNamespace}}
	if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
		return "", fmt.Errorf("failed reading gardenlet ConfigMap: %w", err)
	}
	gardenletConfig, err := encoding.DecodeGardenletConfigurationFromBytes([]byte(configMap.Data["config.yaml"]), false)
	if err != nil {
		return "", fmt.Errorf("failed decoding gardenlet configuration from ConfigMap: %w", err)
	}
	if gardenletConfig.SeedConfig == nil || gardenletConfig.SeedConfig.Name == "" {
		return "", fmt.Errorf("gardenlet configuration in ConfigMap %s has no seed name", client.ObjectKeyFromObject(configMap))
	}

	return gardenletConfig.SeedConfig.Name, nil
}

func gardenIsReady(virtualCluster cluster.Cluster, garden *operatorv1alpha1.Garden) bool {
	return virtualCluster != nil && operator.IsGardenSuccessfullyReconciled(garden)
}
