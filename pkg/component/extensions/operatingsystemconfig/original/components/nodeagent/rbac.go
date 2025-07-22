// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// RBACResourcesData returns a map of serialized Kubernetes resources providing permissions to allow initiating the
// Kubernetes TLS bootstrapping process.
func RBACResourcesData() (map[string][]byte, error) {
	return managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(GetCertificateSigningRequestClusterRoleBindings()...)
}

// GetCertificateSigningRequestClusterRoleBindings returns the ClusterRoleBindings that allows bootstrap tokens to
// create CertificateSigningRequests.
func GetCertificateSigningRequestClusterRoleBindings() []client.Object {
	return []client.Object{
		&rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:node-bootstrapper",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:node-bootstrapper",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		},
		&rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     bootstraptokenapi.BootstrapDefaultGroup,
			}},
		},
		&rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     rbacv1.GroupKind,
				Name:     user.NodesGroup,
			}},
		},
	}
}
