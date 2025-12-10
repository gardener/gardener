// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
)

func (m *manager) Cleanup(ctx context.Context) error {
	secretList, err := m.listSecrets(ctx)
	if err != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, secret := range secretList.Items {
		name := secret.Labels[LabelKeyName]
		if v, ok := secret.Labels[LabelKeyBundleFor]; ok {
			name = v
		}

		if secrets, found := m.getFromStore(name); found &&
			(secrets.current.obj.Name == secret.Name ||
				(secrets.old != nil && secrets.old.obj.Name == secret.Name) ||
				(secrets.bundle != nil && secrets.bundle.obj.Name == secret.Name)) {
			continue
		}

		fns = append(fns, func(ctx context.Context) error {
			m.logger.Info("Deleting stale secret", "secret", client.ObjectKeyFromObject(&secret))
			return client.IgnoreNotFound(m.client.Delete(ctx, &secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}
