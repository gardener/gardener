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
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"
	"github.com/gardener/gardener/pkg/utils/flow"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (k *kubeAPIServer) deployRBAC(ctx context.Context) error {
	var (
		role        = k.emptyRole()
		roleBinding = k.emptyRoleBinding()
	)

	createOrUpdateRoleFunc := func(ctx context.Context) error {
		return createOrUpdateRole(ctx, k.seedClient.Client(), role)
	}
	createOrUpdateRoleBindingFunc := func(ctx context.Context) error {
		return createOrUpdateRoleBinding(ctx, k.seedClient.Client(), role, roleBinding)
	}

	fns := []flow.TaskFn{createOrUpdateRoleFunc, createOrUpdateRoleBindingFunc}
	return flow.Parallel(fns...)(ctx)
}

func createOrUpdateRole(ctx context.Context, client client.Client, role *rbacv1.Role) error {
	if _, err := controllerutil.CreateOrUpdate(ctx, client, role, func() error {
		role.Labels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole:  labelRole,
		}

		role.Rules = []rbacv1.PolicyRule{{
			Verbs:         []string{"get", "list", "watch"},
			APIGroups:     []string{appsv1.SchemeGroupVersion.Group},
			Resources:     []string{"deployments"},
			ResourceNames: []string{v1beta1constants.DeploymentNameKubeAPIServer},
		},
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func createOrUpdateRoleBinding(ctx context.Context, client client.Client, role *rbacv1.Role, roleBinding *rbacv1.RoleBinding) error {
	if _, err := controllerutil.CreateOrUpdate(ctx, client, roleBinding, func() error {
		roleBinding.Labels = map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
			v1beta1constants.LabelRole:  labelRole,
		}

		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}

		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: roleBinding.Namespace,
		}}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) emptyRole() *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: konnectivity.ServerName, Namespace: k.seedNamespace}}
}

func (k *kubeAPIServer) emptyRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: konnectivity.ServerName, Namespace: k.seedNamespace}}
}
