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
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LokiKubeRBACProxyName is the name of managed resources associated with Loki's kube-rbac-proxy and Promtail's RBAC.
	ShootNodeLoggingManagedResourceName = "shoot-node-logging"
	// KubeRBACProxyImageName is the name of the kube-rbac-proxy image.
	KubeRBACProxyImageName = common.LokiKubeRBACProxyName
	KubeRBACProxyUserName  = "gardener.cloud:logging:kube-rbac-proxy"
)

// KubeRBACProxyOptions are the options for the kube-rbac-proxy.
type KubeRBACProxyOptions struct {
	// Client to create resources with.
	Client client.Client
	// Namespace in the seed cluster.
	Namespace string
	// IsShootNodeLoggingActivated flag enables or disables the shoot node logging
	IsShootNodeLoggingActivated bool
}

// NewKubeRBACProxy creates a new instance of kubeRBACProxy for the kube-rbac-proxy.
func NewKubeRBACProxy(so *KubeRBACProxyOptions) (component.DeployWaiter, error) {
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

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)
	if !k.IsShootNodeLoggingActivated {
		return common.DeleteManagedResourceForShoot(ctx, k.Client, ShootNodeLoggingManagedResourceName, k.Namespace)
	}
	resources, err := registry.AddAllAndSerialize(kubeRBACProxyClusterRolebinding)
	if err != nil {
		return err
	}

	return common.DeployManagedResourceForShoot(ctx, k.Client, ShootNodeLoggingManagedResourceName, k.Namespace, false, resources)
}

func (k *kubeRBACProxy) Destroy(ctx context.Context) error {
	return common.DeleteManagedResourceForShoot(ctx, k.Client, ShootNodeLoggingManagedResourceName, k.Namespace)
}

func (k *kubeRBACProxy) Wait(ctx context.Context) error {
	return nil
}

func (k *kubeRBACProxy) WaitCleanup(ctx context.Context) error {
	return nil
}

func getLabels() map[string]string {
	return map[string]string{
		"app": common.LokiKubeRBACProxyName,
	}
}
