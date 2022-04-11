// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"

	"github.com/gardener/gardener/pkg/utils/flow"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *manager) Cleanup(ctx context.Context) error {
	secretList, err := m.listSecrets(ctx)
	if err != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, s := range secretList.Items {
		secret := s

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
			m.logger.Info("Deleting stale secret", "namespace", secret.Namespace, "name", secret.Name)
			return client.IgnoreNotFound(m.client.Delete(ctx, &secret))
		})
	}

	return flow.Parallel(fns...)(ctx)
}
