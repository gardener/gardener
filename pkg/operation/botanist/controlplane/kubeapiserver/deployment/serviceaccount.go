// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package deployment

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const serviceAccountName = "kube-apiserver"

func (k *kubeAPIServer) deployServiceAccount(ctx context.Context) error {
	var serviceAccount = k.emptyServiceAccount()

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole:  labelRole,
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: k.seedNamespace}}
}
