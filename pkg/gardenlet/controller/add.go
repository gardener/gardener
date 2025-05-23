// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controller/tokenrequestor"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
	"github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation"
	"github.com/gardener/gardener/pkg/gardenlet/controller/gardenlet"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	"github.com/gardener/gardener/pkg/gardenlet/controller/tokenrequestor/workloadidentity"
	"github.com/gardener/gardener/pkg/gardenlet/controller/vpaevictionrequirements"
	"github.com/gardener/gardener/pkg/healthz"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AddToManager adds all gardenlet controllers to the given manager.
func AddToManager(
	ctx context.Context,
	mgr manager.Manager,
	gardenletCancel context.CancelFunc,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
	shootClientMap clientmap.ClientMap,
	cfg *gardenletconfigv1alpha1.GardenletConfiguration,
	healthManager healthz.Manager,
) error {
	identity, err := gardenerutils.DetermineIdentity()
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{}
	if err := gardenCluster.GetClient().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: v1beta1constants.ClusterIdentity}, configMap); err != nil {
		return fmt.Errorf("failed getting cluster-identity ConfigMap in garden cluster: %w", err)
	}
	gardenClusterIdentity, ok := configMap.Data[v1beta1constants.ClusterIdentity]
	if !ok {
		return fmt.Errorf("cluster-identity ConfigMap data does not have %q key", v1beta1constants.ClusterIdentity)
	}

	seedClientSet, err := kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(seedCluster.GetConfig()),
		kubernetes.WithRuntimeAPIReader(seedCluster.GetAPIReader()),
		kubernetes.WithRuntimeClient(seedCluster.GetClient()),
		kubernetes.WithRuntimeCache(seedCluster.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("failed creating seed clientset: %w", err)
	}

	if err := (&backupbucket.Reconciler{
		Config:   *cfg.Controllers.BackupBucket,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding BackupBucket controller: %w", err)
	}

	if err := (&backupentry.Reconciler{
		Config:   *cfg.Controllers.BackupEntry,
		SeedName: cfg.SeedConfig.Name,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding BackupEntry controller: %w", err)
	}

	if err := (&bastion.Reconciler{
		Config: *cfg.Controllers.Bastion,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding Bastion controller: %w", err)
	}

	if err := controllerinstallation.AddToManager(ctx, mgr, gardenCluster, seedCluster, seedClientSet, *cfg, identity, gardenClusterIdentity); err != nil {
		return fmt.Errorf("failed adding ControllerInstallation controller: %w", err)
	}

	if err := (&gardenlet.Reconciler{
		Config: *cfg,
	}).AddToManager(mgr, gardenCluster, seedClientSet); err != nil {
		return fmt.Errorf("failed adding Gardenlet controller: %w", err)
	}

	if err := (&managedseed.Reconciler{
		Config:         *cfg,
		ShootClientMap: shootClientMap,
	}).AddToManager(mgr, gardenCluster, seedCluster); err != nil {
		return fmt.Errorf("failed adding ManagedSeed controller: %w", err)
	}

	if err := networkpolicy.AddToManager(ctx, mgr, gardenletCancel, seedCluster, *cfg.Controllers.NetworkPolicy, cfg.SeedConfig.Spec.Networks, nil); err != nil {
		return fmt.Errorf("failed adding NetworkPolicy controller: %w", err)
	}

	if err := seed.AddToManager(mgr, gardenCluster, seedCluster, seedClientSet, *cfg, identity, healthManager); err != nil {
		return fmt.Errorf("failed adding Seed controller: %w", err)
	}

	if err := shoot.AddToManager(ctx, mgr, gardenCluster, seedCluster, seedClientSet, shootClientMap, *cfg, identity, gardenClusterIdentity); err != nil {
		return fmt.Errorf("failed adding Shoot controller: %w", err)
	}

	if err := vpaevictionrequirements.AddToManager(ctx, mgr, gardenletCancel, *cfg.Controllers.VPAEvictionRequirements, seedCluster); err != nil {
		return fmt.Errorf("failed adding VPAEvictionRequirements controller: %w", err)
	}

	if err := (&tokenrequestor.Reconciler{
		ConcurrentSyncs: ptr.Deref(cfg.Controllers.TokenRequestorServiceAccount.ConcurrentSyncs, 0),
		Class:           ptr.To(resourcesv1alpha1.ResourceManagerClassGarden),
		TargetNamespace: gardenerutils.ComputeGardenNamespace(cfg.SeedConfig.Name),
	}).AddToManager(mgr, seedCluster, gardenCluster); err != nil {
		return fmt.Errorf("failed adding TokenRequestorServiceAccount controller: %w", err)
	}

	if err := (&workloadidentity.Reconciler{
		ConcurrentSyncs: ptr.Deref(cfg.Controllers.TokenRequestorWorkloadIdentity.ConcurrentSyncs, 0),
	}).AddToManager(mgr, seedCluster, gardenCluster); err != nil {
		return fmt.Errorf("failed adding TokenRequestorWorkloadIdentity controller: %w", err)
	}

	return nil
}
