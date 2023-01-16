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

package access

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreversionedclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// CreateShootClientFromAdminKubeconfig requests an admin kubeconfig and creates a shoot client.
func CreateShootClientFromAdminKubeconfig(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (kubernetes.Interface, error) {
	kubeconfig, err := RequestAdminKubeconfigForShoot(ctx, gardenClient, shoot, pointer.Int64(7200))
	if err != nil {
		return nil, err
	}

	return kubernetes.NewClientFromBytes(
		kubeconfig,
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.ShootScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// RequestAdminKubeconfigForShoot requests an admin kubeconfig for the given shoot.
func RequestAdminKubeconfigForShoot(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, expirationSeconds *int64) ([]byte, error) {
	versionedClient, err := gardencoreversionedclientset.NewForConfig(gardenClient.RESTConfig())
	if err != nil {
		return nil, err
	}

	adminKubeconfigRequest := &authenticationv1alpha1.AdminKubeconfigRequest{
		Spec: authenticationv1alpha1.AdminKubeconfigRequestSpec{
			ExpirationSeconds: expirationSeconds,
		},
	}
	adminKubeconfig, err := versionedClient.CoreV1beta1().Shoots(shoot.GetNamespace()).CreateAdminKubeconfigRequest(ctx, shoot.GetName(), adminKubeconfigRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return adminKubeconfig.Status.Kubeconfig, nil
}
