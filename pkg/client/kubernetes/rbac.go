// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ListRoleBindings returns a list of rolebindings in a given <namespace>.
// The selection can be restricted by passsing an <selector>.
func (c *Clientset) ListRoleBindings(namespace string, selector metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	return c.kubernetes.RbacV1().RoleBindings(namespace).List(selector)
}

// CreateOrPatchRoleBinding either creates the object or patches the existing one with the strategic merge patch type.
func (c *Clientset) CreateOrPatchRoleBinding(meta metav1.ObjectMeta, transform func(*rbacv1.RoleBinding) *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	rolebinding, err := c.kubernetes.RbacV1().RoleBindings(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.kubernetes.RbacV1().RoleBindings(meta.Namespace).Create(transform(&rbacv1.RoleBinding{
				TypeMeta: metav1.TypeMeta{
					APIVersion: rbacv1.SchemeGroupVersion.String(),
					Kind:       "RoleBinding",
				},
				ObjectMeta: meta,
			}))
		}
		return nil, err
	}
	return c.patchRoleBinding(rolebinding, transform(rolebinding.DeepCopy()))
}

func (c *Clientset) patchRoleBinding(oldObj, newObj *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
	patch, err := kutil.CreateTwoWayMergePatch(oldObj, newObj)
	if err != nil {
		return nil, err
	}

	if kutil.IsEmptyPatch(patch) {
		return oldObj, nil
	}

	return c.kubernetes.RbacV1().RoleBindings(oldObj.Namespace).Patch(oldObj.Name, types.StrategicMergePatchType, patch)
}

// DeleteClusterRole deletes a ClusterRole object.
func (c *Clientset) DeleteClusterRole(name string) error {
	return c.Kubernetes().RbacV1().ClusterRoles().Delete(name, &defaultDeleteOptions)
}

// DeleteClusterRoleBinding deletes a ClusterRoleBinding object.
func (c *Clientset) DeleteClusterRoleBinding(name string) error {
	return c.Kubernetes().RbacV1().ClusterRoleBindings().Delete(name, &defaultDeleteOptions)
}

// DeleteRoleBinding deletes a RoleBindung object.
func (c *Clientset) DeleteRoleBinding(namespace, name string) error {
	return c.Kubernetes().RbacV1().RoleBindings(namespace).Delete(name, &defaultDeleteOptions)
}
