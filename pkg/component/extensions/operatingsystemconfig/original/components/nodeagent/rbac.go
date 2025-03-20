// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent

import (
	coordinationv1 "k8s.io/api/coordination/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// RBACResourcesData returns a map of serialized Kubernetes resources that allow the gardener-node-agent to
// access the list of given secrets. Additionally, serialized resources providing permissions to allow initiating the
// Kubernetes TLS bootstrapping process will be returned.
func RBACResourcesData(secretNames []string) (map[string][]byte, error) {
	var (
		// TODO(oliver-goetz): Remove when NodeAgentAuthorizer feature gate is going to be removed
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-node-agent",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "nodes/status"},
					Verbs:     []string{"get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list", "watch", "patch", "update", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"get", "list", "watch", "create", "patch", "update"},
				},
			},
		}

		// TODO(oliver-goetz): Remove when NodeAgentAuthorizer feature gate is going to be removed
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener-node-agent",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      nodeagentconfigv1alpha1.AccessSecretName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		// TODO(oliver-goetz): Remove when NodeAgentAuthorizer feature gate is going to be removed
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: append([]string{nodeagentconfigv1alpha1.AccessSecretName, valiconstants.ValitailTokenSecretName}, secretNames...),
					Verbs:         []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch", "create", "update"},
				},
			},
		}

		// TODO(oliver-goetz): Remove when NodeAgentAuthorizer feature gate is going to be removed
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-node-agent",
				Namespace: metav1.NamespaceSystem,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.SchemeGroupVersion.Group,
				Kind:     "Role",
				Name:     role.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: rbacv1.GroupKind,
					Name: bootstraptokenapi.BootstrapDefaultGroup,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      nodeagentconfigv1alpha1.AccessSecretName,
					Namespace: metav1.NamespaceSystem,
				},
			},
		}
	)

	if features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer) {
		// When the feature gate is enabled, gardener-node-agent service account needs permissions to create an CSR on
		// existing nodes for migration.
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"certificates.k8s.io"},
			Resources: []string{"certificatesigningrequests"},
			Verbs:     []string{"create", "get"},
		})
	} else {
		// For the case that NodeAgentAuthorizer feature gate is disabled again node-agents group has to be added to
		// (cluster) role binding so that node-agents which are already migrated do not lose access.
		clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, rbacv1.Subject{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     rbacv1.GroupKind,
			Name:     v1beta1constants.NodeAgentsGroup,
		})
		roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     rbacv1.GroupKind,
			Name:     v1beta1constants.NodeAgentsGroup,
		})
	}

	return managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(append([]client.Object{
			clusterRole,
			clusterRoleBinding,
			role,
			roleBinding,
		}, GetCertificateSigningRequestClusterRoleBindings()...)...)
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
