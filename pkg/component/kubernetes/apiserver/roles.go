// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const roleNameHAVPN = "kube-apiserver-vpn-client-init"

func (k *kubeAPIServer) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: k.namespace,
		},
	}
}

func (k *kubeAPIServer) emptyRoleHAVPN() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleNameHAVPN,
			Namespace: k.namespace,
		},
	}
}

func (k *kubeAPIServer) emptyRoleBindingHAVPN() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleNameHAVPN,
			Namespace: k.namespace,
		},
	}
}

func (k *kubeAPIServer) reconcileServiceAccount(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = ptr.To(false)
		return nil
	})
	return err
}

func (k *kubeAPIServer) reconcileRoleHAVPN(ctx context.Context) error {
	role := k.emptyRoleHAVPN()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch", "patch", "update"},
			},
		}
		return nil
	})
	return err
}

func (k *kubeAPIServer) reconcileRoleBindingHAVPN(ctx context.Context, serviceAccount *corev1.ServiceAccount) error {
	roleBinding := k.emptyRoleBindingHAVPN()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleNameHAVPN,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		}
		return nil
	})
	return err
}
