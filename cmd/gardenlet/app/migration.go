// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	seedClient := g.mgr.GetClient()

	// The below migration is preserved in order to cover the migration when upgrading from
	// VPAInPlaceUpdates feature gate disabled to VPAInPlaceUpdates enabled unconditionally.
	//
	// TODO(ialidzhikov): Clean up the migration below when cleaning up the VPAInPlaceUpdates feature gate.
	if err := migration.MigrateVPAEmptyPatch(ctx, seedClient, log); err != nil {
		return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
	}

	// TODO(shafeeqes): Remove this function in gardener v1.148
	log.Info("Cleaning up OSC hash versioning secrets in shoot namespaces")
	if err := CleanupHashVersioningSecrets(ctx, seedClient); err != nil {
		return fmt.Errorf("failed to clean up OSC hash versioning secrets: %w", err)
	}

	if features.DefaultFeatureGate.Enabled(features.RemoveHTTPProxyLegacyPort) {
		if err := verifyRemoveHTTPProxyLegacyPortMigration(ctx, gardenClient, g.config.SeedConfig.Name); err != nil {
			return fmt.Errorf("failed to verify migration for RemoveHTTPProxyLegacyPort feature gate: %w", err)
		}
	}

	return nil
}

// TODO(jamand): Remove when feature gate RemoveHTTPProxyLegacyPort is removed.
func verifyRemoveHTTPProxyLegacyPortMigration(ctx context.Context, gardenClient client.Client, seedName string) error {
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

// CleanupHashVersioningSecrets removes the OSC hash versioning secrets in shoot namespaces.
func CleanupHashVersioningSecrets(ctx context.Context, seedClient client.Client) error {
	shootNamespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, ns := range shootNamespaceList.Items {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns.Name, Name: operatingsystemconfig.WorkerPoolHashesSecretName},
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			if err := client.IgnoreNotFound(seedClient.Delete(ctx, secret)); err != nil {
				return fmt.Errorf("failed deleting secret %s in namespace %s: %w", secret.Name, secret.Namespace, err)
			}

			return nil
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}
