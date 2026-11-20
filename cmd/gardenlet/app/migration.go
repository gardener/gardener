// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	if !gardenlet.IsResponsibleForSelfHostedShoot() && features.DefaultFeatureGate.Enabled(features.RemoveHTTPProxyLegacyPort) {
		if err := VerifyRemoveHTTPProxyLegacyPortMigration(ctx, gardenClient, g.config.SeedConfig.Name); err != nil {
			return fmt.Errorf("failed to verify migration for RemoveHTTPProxyLegacyPort feature gate: %w", err)
		}
	}

	// TODO(DobromirNPeev): Remove this migration after Gardener v1.158 has been released.
	if err := migration.RemoveVPAInPlaceUpdatesMutatedLabel(ctx, g.mgr.GetClient(), log); err != nil {
		return err
	}

	// TODO(gagan16k): Remove this migration after Gardener v1.159 has been released.
	log.Info("Migrating RBAC resources for cluster-autoscaler")
	if err := migrateCARBAC(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	return nil
}

// Replaces the cluster-autoscaler's old cluster-wide ClusterRoleBinding with a namespaced Role/RoleBinding,
// and removes the seed-wide ClusterRole managed resource that used to back it.
// TODO(gagan16k): Remove this function after Gardener v1.159 has been released.
func migrateCARBAC(ctx context.Context, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return fmt.Errorf("failed listing namespaces with '%s: %s' label: %w", v1beta1constants.GardenRole, v1beta1constants.GardenRoleShoot, err)
	}

	var tasks []flow.TaskFn

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}
		tasks = append(tasks, func(ctx context.Context) error {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := seedClient.Get(ctx, client.ObjectKey{Name: "cluster-autoscaler-" + namespace.Name}, clusterRoleBinding); err != nil {
				// If the cluster-autoscaler ClusterRoleBinding does not exist, there is nothing to do.
				return client.IgnoreNotFound(err)
			}

			return clusterautoscaler.New(seedClient, namespace.Name, nil, "", 0, nil, nil, nil, nil).MigrateRBAC(ctx)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return err
	}

	// Remove the seed-wide ClusterRole managed resource that used to be deployed in the garden namespace by the
	// cluster-autoscaler bootstrapper.
	if err := managedresources.DeleteForSeed(ctx, seedClient, v1beta1constants.GardenNamespace, "cluster-autoscaler"); err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
	}

	return nil
}

// VerifyRemoveHTTPProxyLegacyPortMigration returns an error if any shoot on this seed has not yet been reconfigured to
// use the unified `http-proxy` port, i.e. if removing the legacy `tls-tunnel` port would break it.
// TODO(jamand): Remove when feature gate RemoveHTTPProxyLegacyPort is removed.
func VerifyRemoveHTTPProxyLegacyPortMigration(ctx context.Context, gardenClient client.Client, seedName string) error {
	// List all (eligible) Shoot resources managed by this seed.
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return err
	}

	for _, shoot := range shootList.Items {
		if specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(&shoot); gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) != seedName {
			continue
		}

		// Skip workerless Shoots (UsesUnifiedHTTPProxyPort is never set for workerless Shoots).
		if helper.IsWorkerless(&shoot) {
			continue
		}

		// Skip hibernated Shoots (UsesUnifiedHTTPProxyPort is not set, vpn-shoot will be deployed on wake-up Reconciliation).
		if helper.IsShootInHibernation(&shoot) {
			continue
		}

		// Skip if not picked up yet or Creating/Deleting.
		if shoot.Status.LastOperation == nil || ((shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate || shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete) && shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) {
			continue
		}

		if cond := helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
			return errors.New("the `tls-tunnel` port on the istio ingress gateway cannot be removed until the api server proxy and vpn client in all shoots on this seed have been reconfigured to use the unified `http-proxy` port instead, i.e., the `RemoveHTTPProxyLegacyPort` feature gate can only be enabled once all shoots have the `UsesUnifiedHTTPProxyPort` constraint with status `true`")
		}
	}

	return nil
}
