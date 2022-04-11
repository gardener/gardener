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

package logging

import (
	"context"
	"errors"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	managedResourceName = "shoot-node-logging"

	kubeRBACProxyName            = "kube-rbac-proxy"
	kubeRBACProxyClusterRoleName = "gardener.cloud:logging:kube-rbac-proxy"

	promtailName     = "gardener-promtail"
	promtailRBACName = "gardener.cloud:logging:promtail"
	// PromtailTokenSecretName is the name of a secret in the kube-system namespace in the target cluster containing
	// promtail's token for communication with the kube-apiserver.
	PromtailTokenSecretName = promtailName
)

// Values are the values for the kube-rbac-proxy.
type Values struct {
	// Client to create resources with.
	Client client.Client
	// Namespace in the seed cluster.
	Namespace string
}

// NewKubeRBACProxy creates a new instance of kubeRBACProxy for the kube-rbac-proxy.
func NewKubeRBACProxy(so *Values) (component.Deployer, error) {
	if so == nil {
		return nil, errors.New("options cannot be nil")
	}

	if so.Client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if len(so.Namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	return &kubeRBACProxy{Values: so}, nil
}

type kubeRBACProxy struct {
	*Values
}

func (k *kubeRBACProxy) Deploy(ctx context.Context) error {
	kubeRBACProxyShootAccessSecret := k.newKubeRBACProxyShootAccessSecret()
	if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, k.Client); err != nil {
		return err
	}

	promtailShootAccessSecret := k.newPromtailShootAccessSecret()
	if err := promtailShootAccessSecret.Reconcile(ctx, k.Client); err != nil {
		return err
	}

	var (
		kubeRBACProxyClusterRolebinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   kubeRBACProxyClusterRoleName,
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

		promtailClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   promtailRBACName,
				Labels: getPromtailLabels(),
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
						"watch",
						"list",
					},
				},
				{
					NonResourceURLs: []string{
						"/loki/api/v1/push",
					},
					Verbs: []string{
						"create",
					},
				},
			},
		}

		promtailClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   promtailRBACName,
				Labels: getPromtailLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     promtailClusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      promtailShootAccessSecret.ServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	resources, err := registry.AddAllAndSerialize(
		kubeRBACProxyClusterRolebinding,
		promtailClusterRole,
		promtailClusterRoleBinding,
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForShoot(ctx, k.Client, k.Namespace, managedResourceName, false, resources); err != nil {
		return err
	}

	return nil
}

func (k *kubeRBACProxy) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, k.Client, k.Namespace, managedResourceName); err != nil {
		return err
	}

	return kutil.DeleteObjects(ctx, k.Client,
		k.newKubeRBACProxyShootAccessSecret().Secret,
		k.newPromtailShootAccessSecret().Secret,
	)
}

func (k *kubeRBACProxy) newKubeRBACProxyShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret(kubeRBACProxyName, k.Values.Namespace)
}

func (k *kubeRBACProxy) newPromtailShootAccessSecret() *gutil.ShootAccessSecret {
	return gutil.NewShootAccessSecret("promtail", k.Values.Namespace).
		WithServiceAccountName(promtailName).
		WithTargetSecret(PromtailTokenSecretName, metav1.NamespaceSystem)
}

func getKubeRBACProxyLabels() map[string]string {
	return map[string]string{
		"app": kubeRBACProxyName,
	}
}

func getPromtailLabels() map[string]string {
	return map[string]string{
		"app": promtailName,
	}
}
