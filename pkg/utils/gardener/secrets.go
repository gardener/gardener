// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	// NoControlPlaneSecretsReq is a label selector requirement to select non-control plane secrets.
	NoControlPlaneSecretsReq = utils.MustNewRequirement(constants.GardenRole, selection.NotIn, constants.ControlPlaneSecretRoles...)
	// UncontrolledSecretSelector is a selector for objects which are managed by operators/users and not created by
	// Gardener controllers.
	UncontrolledSecretSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(NoControlPlaneSecretsReq)}
)

// FetchKubeconfigFromSecret tries to retrieve the kubeconfig bytes in given secret.
func FetchKubeconfigFromSecret(ctx context.Context, c client.Client, key client.ObjectKey) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok || len(kubeconfig) == 0 {
		return nil, errors.New("the secret's field 'kubeconfig' is either not present or empty")
	}

	return kubeconfig, nil
}
