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

package kuberbacproxy

import (
	"context"
	"errors"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-node-logging"

	kubeRBACProxyName = "kube-rbac-proxy"
	clusterRoleName   = "gardener.cloud:logging:kube-rbac-proxy"

	valitailName     = "gardener-valitail"
	valitailRBACName = "gardener.cloud:logging:valitail"
	// ValitailTokenSecretName is the name of a secret in the kube-system namespace in the target cluster containing
	// valitail's token for communication with the kube-apiserver.
	ValitailTokenSecretName = valitailName
)

// New creates a new instance of kubeRBACProxy for the kube-rbac-proxy.
func New(client client.Client, namespace string) (component.Deployer, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	return &kubeRBACProxy{client: client, namespace: namespace}, nil
}

type kubeRBACProxy struct {
	// client to create resources with.
	client client.Client
	// namespace in the seed cluster.
	namespace string
}

func (k *kubeRBACProxy) Deploy(ctx context.Context) error {
	kubeRBACProxyShootAccessSecret := k.newKubeRBACProxyShootAccessSecret()
	if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, k.client); err != nil {
		return err
	}

	valitailShootAccessSecret := k.newValitailShootAccessSecret()
	if err := valitailShootAccessSecret.Reconcile(ctx, k.client); err != nil {
		return err
	}

	var (
		kubeRBACProxyClusterRolebinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterRoleName,
				Labels: getKubeRBACProxyLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:auth-delegator",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      kubeRBACProxyShootAccessSecret.ServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		valitailClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   valitailRBACName,
				Labels: getValitailLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						"",
					},
					Resources: []string{
						"nodes",
						"nodes/proxy",
						"services",
						"endpoints",
						"pods",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
				{
					NonResourceURLs: []string{
						"/vali/api/v1/push",
					},
					Verbs: []string{
						"create",
					},
				},
			},
		}

		valitailClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   valitailRBACName,
				Labels: getValitailLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     valitailClusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      valitailShootAccessSecret.ServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	resources, err := registry.AddAllAndSerialize(
		kubeRBACProxyClusterRolebinding,
		valitailClusterRole,
		valitailClusterRoleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, k.client, k.namespace, managedResourceName, managedresources.LabelValueGardener, false, resources)
}

func (k *kubeRBACProxy) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, k.client, k.namespace, managedResourceName); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, k.client,
		k.newKubeRBACProxyShootAccessSecret().Secret,
		k.newValitailShootAccessSecret().Secret,
	)
}

func (k *kubeRBACProxy) newKubeRBACProxyShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(kubeRBACProxyName, k.namespace)
}

func (k *kubeRBACProxy) newValitailShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret("valitail", k.namespace).
		WithServiceAccountName(valitailName).
		WithTargetSecret(ValitailTokenSecretName, metav1.NamespaceSystem)
}

func getKubeRBACProxyLabels() map[string]string {
	return map[string]string{
		"app": kubeRBACProxyName,
	}
}

func getValitailLabels() map[string]string {
	return map[string]string{
		"app": valitailName,
	}
}
