// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func computeGardenerData(
	ctx context.Context,
	seedClient client.Client,
	seedNamespace string,
) (
	[]gardencorev1beta1.GardenerResourceData,
	error,
) {
	secretList := &corev1.SecretList{}
	if err := seedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
		secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
		secretsmanager.LabelKeyPersist:   secretsmanager.LabelValueTrue,
	}); err != nil {
		return nil, fmt.Errorf("failed listing all secrets that must be persisted: %w", err)
	}

	dataList := make([]gardencorev1beta1.GardenerResourceData, 0, len(secretList.Items))

	for _, secret := range secretList.Items {
		dataJSON, err := json.Marshal(secret.Data)
		if err != nil {
			return nil, fmt.Errorf("failed marshalling secret data to JSON for secret %s: %w", client.ObjectKeyFromObject(&secret), err)
		}

		dataList = append(dataList, gardencorev1beta1.GardenerResourceData{
			Name:   secret.Name,
			Labels: secret.Labels,
			Type:   "secret",
			Data:   runtime.RawExtension{Raw: dataJSON},
		})
	}

	return dataList, nil
}
