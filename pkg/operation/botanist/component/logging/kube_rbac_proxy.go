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
	"github.com/gardener/gardener/pkg/utils/managedresources"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ShootNodeLoggingManagedResourceName is the name of managed resources associated with Loki's kube-rbac-proxy and Promtail's RBAC.
	ShootNodeLoggingManagedResourceName = "shoot-node-logging"
	// LokiKubeRBACProxyName  is the name of the kube-rbac-proxy image.
	LokiKubeRBACProxyName = "kube-rbac-proxy"
	// SecretNameLokiKubeRBACProxyKubeconfig is the name of the Loki's kube-rbac-proxy kubeconfig secret.
	SecretNameLokiKubeRBACProxyKubeconfig = LokiKubeRBACProxyName + "-kubeconfig"
	// KubeRBACProxyUserName is the name of the user used by Kube-RBAC-Proxy to make delegating authorization decisions.
	KubeRBACProxyUserName = "gardener.cloud:logging:kube-rbac-proxy"
	// PromtailRBACName is the name of the user used by promtail to auth gains Kube-RBAC-Proxy
	PromtailRBACName = "gardener.cloud:logging:promtail"
	// PromtailName is the name used for labeling the Kubernetes resources associated with Promtail installed on the shoot nodes.
	PromtailName = "gardener-promtail"
)

// KubeRBACProxyOptions are the options for the kube-rbac-proxy.
type KubeRBACProxyOptions struct {
	// Client to create resources with.
	Client client.Client
	// Namespace in the seed cluster.
	Namespace string
	// IsShootNodeLoggingEnabled flag enables or disables the shoot node logging
	IsShootNodeLoggingEnabled bool
}

// NewKubeRBACProxy creates a new instance of kubeRBACProxy for the kube-rbac-proxy.
func NewKubeRBACProxy(so *KubeRBACProxyOptions) (component.Deployer, error) {
	if so == nil {
		return nil, errors.New("options cannot be nil")
	}

	if so.Client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if len(so.Namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	return &kubeRBACProxy{KubeRBACProxyOptions: so}, nil
}

type kubeRBACProxy struct {
	*KubeRBACProxyOptions
}

func (k *kubeRBACProxy) Deploy(ctx context.Context) error {
	var (
		kubeRBACProxyClusterRolebinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   KubeRBACProxyUserName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:auth-delegator",
			},
			Subjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: KubeRBACProxyUserName,
			}},
		}

		promtailClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   PromtailRBACName,
				Labels: getPromtailLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     PromtailRBACName,
			},
			Subjects: []rbacv1.Subject{{
				Kind: rbacv1.UserKind,
				Name: PromtailRBACName,
			}},
		}

		promtailClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   PromtailRBACName,
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

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	if !k.IsShootNodeLoggingEnabled {
		return managedresources.DeleteForShoot(ctx, k.Client, k.Namespace, ShootNodeLoggingManagedResourceName)

	}

	resources, err := registry.AddAllAndSerialize(kubeRBACProxyClusterRolebinding, promtailClusterRole, promtailClusterRoleBinding)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, k.Client, k.Namespace, ShootNodeLoggingManagedResourceName, false, resources)
}

func (k *kubeRBACProxy) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, k.Client, k.Namespace, ShootNodeLoggingManagedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		"app": LokiKubeRBACProxyName,
	}
}

func getPromtailLabels() map[string]string {
	return map[string]string{
		"app": PromtailName,
	}
}
