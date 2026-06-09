// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/utils/flow"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	seedClient := g.mgr.GetClient()

	// The below migration is preserved in order to cover the migration when upgrading from
	// VPAInPlaceUpdates feature gate disabled to VPAInPlaceUpdates enabled unconditionally.
	//
	// TODO(ialidzhikov): Clean up the migration below when cleaning up the VPAInPlaceUpdates feature gate.
	if err := migration.MigrateVPAEmptyPatch(ctx, seedClient, log); err != nil {
		return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
	}

	// TODO(shafeeqes): Remove this function in gardener v1.147
	log.Info("Cleaning up OSC hash versioning secrets in shoot namespaces")
	if err := CleanupHashVersioningSecrets(ctx, seedClient); err != nil {
		return fmt.Errorf("failed to clean up OSC hash versioning secrets: %w", err)
	}

	return nil
}

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
