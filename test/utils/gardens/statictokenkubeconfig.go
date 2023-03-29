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

package access

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// CreateVirtualClusterClientFromStaticTokenKubeconfig retrieves the static token kubeconfig secret and creates a client
// for the virtual cluster.
func CreateVirtualClusterClientFromStaticTokenKubeconfig(ctx context.Context, runtimeClient client.Client, namespace string) (kubernetes.Interface, error) {
	secretList := &corev1.SecretList{}
	if err := runtimeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{secretsmanager.LabelKeyName: "user-kubeconfig"}); err != nil {
		return nil, err
	}

	if length := len(secretList.Items); length != 1 {
		return nil, fmt.Errorf("expected exactly 1 secret but found %d", length)
	}

	return kubernetes.NewClientFromSecret(ctx, runtimeClient, secretList.Items[0].Namespace, secretList.Items[0].Name, kubernetes.WithDisabledCachedClient())
}
