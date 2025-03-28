// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
)

// MigrateSecrets exports the secrets generated with the fake client and imports them with the real client.
func (b *AutonomousBotanist) MigrateSecrets(ctx context.Context, fakeClient, realClient client.Client) error {
	secretList := &corev1.SecretList{}
	if err := fakeClient.List(ctx, secretList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return fmt.Errorf("failed listing secrets with fake client: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, secret := range secretList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return realClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        secret.Name,
					Namespace:   secret.Namespace,
					Labels:      secret.Labels,
					Annotations: secret.Annotations,
				},
				Type:      secret.Type,
				Immutable: secret.Immutable,
				Data:      secret.Data,
			})
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}
