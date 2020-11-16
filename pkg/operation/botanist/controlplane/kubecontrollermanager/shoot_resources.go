// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubecontrollermanager

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func (k *kubeControllerManager) deployShootManagedResource(ctx context.Context) error {
	if versionConstraintK8sGreaterEqual112.Check(k.version) && versionConstraintK8sSmaller114.Check(k.version) {
		data, err := k.computeShootResourcesData()
		if err != nil {
			return err
		}
		return common.DeployManagedResourceForShoot(ctx, k.seedClient, managedResourceName, k.namespace, false, data)
	}

	return kutil.DeleteObjects(
		ctx,
		k.seedClient,
		k.emptyManagedResource(),
		k.emptyManagedResourceSecret(),
	)
}

func (k *kubeControllerManager) computeShootResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		subjects = []rbacv1.Subject{{
			Kind: "User",
			Name: user.KubeControllerManager,
		}}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:controller:kube-controller-manager",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:auth-delegator",
			},
			Subjects: subjects,
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "system:controller:kube-controller-manager:auth-reader",
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     "extension-apiserver-authentication-reader",
			},
			Subjects: subjects,
		}
	)

	return registry.AddAllAndSerialize(
		clusterRoleBinding,
		roleBinding,
	)
}
