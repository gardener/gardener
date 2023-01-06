// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nginxingress

import (
	"context"
	"time"

	"github.com/Masterminds/semver"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the nginx-ingress addon managed resource.
	ManagedResourceName = "shoot-addon-nginx-ingress"

	labelAppValue = "nginx-ingress"

	clusterRoleName = "addons-nginx-ingress"
)

// Values is a set of configuration values for the nginx-ingress component.
type Values struct {
	// ImageController is the container image used for nginx-ingress controller.
	ImageController string
	// ImageDefaultBackend is the container image used for default ingress backend.
	ImageDefaultBackend string
	// KubernetesVersion is the version of kubernetes for the shoot cluster.
	KubernetesVersion *semver.Version
	// ConfigData contains the configuration details for the nginx-ingress controller
	ConfigData map[string]string
	// KubeAPIServerHost is the host of the kube-apiserver.
	KubeAPIServerHost string
}

// New creates a new instance of DeployWaiter for nginx-ingress
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &nginxIngress{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nginxIngress struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nginxIngress) Deploy(ctx context.Context) error {
	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nginxIngress) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nginxIngress) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nginxIngress) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, ManagedResourceName)
}

func (n *nginxIngress) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleName,
				Labels: map[string]string{
					v1beta1constants.LabelApp: labelAppValue,
					"release":                 "addons",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "nodes", "pods", "secrets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"services", "configmaps"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"extensions", "networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{"extensions", "networking.k8s.io"},
					Resources: []string{"ingresses/status"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingressclasses"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"list", "watch"},
				},
			},
		}
	)

	return registry.AddAllAndSerialize(
		clusterRole,
	)
}
