// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// MigrateSecrets exports the secrets generated with the fake client and imports them with the real client.
func (b *GardenadmBotanist) MigrateSecrets(ctx context.Context, fakeClient, realClient client.Client) error {
	secretList := &corev1.SecretList{}
	if err := fakeClient.List(ctx, secretList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return fmt.Errorf("failed listing secrets with fake client: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, secret := range secretList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return client.IgnoreAlreadyExists(realClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        secret.Name,
					Namespace:   secret.Namespace,
					Labels:      secret.Labels,
					Annotations: secret.Annotations,
				},
				Type:      secret.Type,
				Immutable: secret.Immutable,
				Data:      secret.Data,
			}))
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

const bootstrapShootStateFileName = "bootstrap-shootstate.yaml"

// PersistBootstrapSecrets exports all secrets from the fake client into a ShootState file in the config directory.
// On retry, `ReadManifests` picks up this file, the botanist enters the restore phase, and the existing
// `restoreSecretsFromShootState` restores the same secrets (including derived certs, not just CAs).
func (b *GardenadmBotanist) PersistBootstrapSecrets(ctx context.Context, configDir string) error {
	secretList := &corev1.SecretList{}
	if err := b.SeedClientSet.Client().List(ctx, secretList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return fmt.Errorf("failed listing secrets: %w", err)
	}

	var gardenerData []gardencorev1beta1.GardenerResourceData
	for _, secret := range secretList.Items {
		if len(secret.Data) == 0 {
			continue
		}

		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return fmt.Errorf("failed marshalling secret data for %s: %w", secret.Name, err)
		}

		gardenerData = append(gardenerData, gardencorev1beta1.GardenerResourceData{
			Name:   secret.Name,
			Labels: secret.Labels,
			Type:   v1beta1constants.DataTypeSecret,
			Data:   runtime.RawExtension{Raw: dataJSON},
		})
	}

	shoot := b.Shoot.GetInfo()
	shootState := &gardencorev1beta1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
		Spec: gardencorev1beta1.ShootStateSpec{
			Gardener: gardenerData,
		},
	}

	shootStateBytes, err := runtime.Encode(kubernetes.GardenCodec.EncoderForVersion(kubernetes.GardenSerializer, gardencorev1beta1.SchemeGroupVersion), shootState)
	if err != nil {
		return fmt.Errorf("failed encoding ShootState: %w", err)
	}

	return b.FS.WriteFile(filepath.Join(configDir, bootstrapShootStateFileName), shootStateBytes, 0600)
}

// CleanupBootstrapSecrets removes the bootstrap ShootState file from the config directory.
func (b *GardenadmBotanist) CleanupBootstrapSecrets(configDir string) error {
	if err := b.FS.Remove(filepath.Join(configDir, bootstrapShootStateFileName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed removing bootstrap ShootState file: %w", err)
	}
	return nil
}
