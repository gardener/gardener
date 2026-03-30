// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/cmd/internal/migration"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	shootstate "github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	seedClient := g.mgr.GetClient()

	if features.DefaultFeatureGate.Enabled(features.VPAInPlaceUpdates) {
		if err := migration.MigrateVPAEmptyPatch(ctx, seedClient, log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAEmptyPatch' migration: %w", err)
		}
	} else {
		if err := migration.MigrateVPAUpdateModeToRecreate(ctx, seedClient, log); err != nil {
			return fmt.Errorf("failed to migrate VerticalPodAutoscaler with 'MigrateVPAUpdateModeToRecreate' migration: %w", err)
		}
	}

	if err := migrateShootStateSecretFormat(ctx, gardenClient, seedClient, log); err != nil {
		return fmt.Errorf("failed to migrate ShootState secret format: %w", err)
	}

	return nil
}

// TODO(tobschli): Remove this migration in the following release after the one that introduces it.
func migrateShootStateSecretFormat(ctx context.Context, gardenClient client.Client, seedClient client.Client, log logr.Logger) error {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return fmt.Errorf("failed listing Shoots: %w", err)
	}

	var tasks []flow.TaskFn

	for _, shoot := range shootList.Items {
		tasks = append(tasks, func(ctx context.Context) error {
			ss := &gardencorev1beta1.ShootState{}
			if err := gardenClient.Get(ctx, client.ObjectKey{Name: shoot.Name, Namespace: shoot.Namespace}, ss); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("failed getting ShootState for shoot %s: %w", client.ObjectKeyFromObject(&shoot), err)
			}

			patch := client.MergeFrom(ss.DeepCopy())
			changed := false

			for i, entry := range ss.Spec.Gardener {
				if entry.Type != v1beta1constants.DataTypeSecret {
					continue
				}

				var newFormat shootstate.SecretState
				if err := json.Unmarshal(entry.Data.Raw, &newFormat); err == nil && newFormat.Data != nil {
					continue
				}

				var oldFormat map[string][]byte
				if err := json.Unmarshal(entry.Data.Raw, &oldFormat); err != nil {
					log.Error(err, "Failed to unmarshal secret data, skipping", "shootState", client.ObjectKeyFromObject(ss), "secret", entry.Name)
					continue
				}

				newFormat = shootstate.SecretState{
					Data: oldFormat,
					Type: corev1.SecretTypeOpaque,
				}

				secret := &corev1.Secret{}
				err := seedClient.Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: entry.Name}, secret)
				if err != nil {
					return fmt.Errorf("failed getting secret %s for ShootState %s: %w", entry.Name, client.ObjectKeyFromObject(ss), err)
				}

				if err == nil {
					newFormat.Type = secret.Type
					newFormat.Immutable = secret.Immutable
				}

				newRaw, err := json.Marshal(newFormat)
				if err != nil {
					return fmt.Errorf("failed marshalling secret %s in ShootState %s: %w", entry.Name, client.ObjectKeyFromObject(ss), err)
				}

				ss.Spec.Gardener[i].Data.Raw = newRaw
				changed = true
			}

			if !changed {
				return nil
			}

			if err := gardenClient.Patch(ctx, ss, patch); err != nil {
				return fmt.Errorf("failed patching ShootState %s: %w", client.ObjectKeyFromObject(ss), err)
			}

			log.Info("Successfully migrated ShootState secret format", "shootState", client.ObjectKeyFromObject(ss))
			return nil
		})
	}

	return flow.Parallel(tasks...)(ctx)
}
