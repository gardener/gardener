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

package tokenrequest

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// RenewAccessSecrets drops the serviceaccount.resources.gardener.cloud/token-renew-timestamp annotation from all
// access secrets. This will make the TokenRequestor controller part of gardener-resource-manager issuing new
// tokens immediately.
func RenewAccessSecrets(ctx context.Context, c client.Client, namespace string) error {
	secretList := &corev1.SecretList{}
	if err := c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
		resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
	}); err != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, obj := range secretList.Items {
		secret := obj

		fns = append(fns, func(ctx context.Context) error {
			patch := client.MergeFrom(secret.DeepCopy())
			delete(secret.Annotations, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp)
			return c.Patch(ctx, &secret, patch)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// IsTokenPopulated checks if a `kubeconfig` secret already contains a token.
func IsTokenPopulated(secret *corev1.Secret) (bool, error) {
	kubeconfig := &clientcmdv1.Config{}
	if _, _, err := clientcmdlatest.Codec.Decode(secret.Data[kubernetes.KubeConfig], nil, kubeconfig); err != nil {
		return false, err
	}

	var userName string
	for _, namedContext := range kubeconfig.Contexts {
		if namedContext.Name == kubeconfig.CurrentContext {
			userName = namedContext.Context.AuthInfo
			break
		}
	}

	for _, users := range kubeconfig.AuthInfos {
		if users.Name == userName {
			if len(users.AuthInfo.Token) > 0 {
				return true, nil
			}
			return false, nil
		}
	}

	return false, nil
}
