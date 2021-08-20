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

package coredns

import (
	"context"
	"strconv"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/api/resources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-coredns"
	// PortServiceServer is the target port used for the DNS server.
	PortServiceServer = 53
	// PortServer is the service port used for the DNS server.
	PortServer = 8053

	deploymentName = "coredns"
	containerName  = "coredns"
	serviceName    = "kube-dns" // this is due to legacy reasons

	portNameMetrics = "metrics"
	portMetrics     = 9153
)

// Interface contains functions for a CoreDNS deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

type Values struct {
	// ClusterDomain is the domain used for cluster-wide DNS records handled by CoreDNS.
	ClusterDomain string
	// Image is the container image used for CoreDNS.
	Image string
}

// New creates a new instance of DeployWaiter for coredns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &coreDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type coreDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (c *coreDNS) Deploy(ctx context.Context) error {
	data, err := c.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, ManagedResourceName, false, data)
}

func (c *coreDNS) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, c.client, c.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *coreDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *coreDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, ManagedResourceName)
}

func (c *coreDNS) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:coredns",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "services", "pods", "namespaces"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "system:coredns",
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		// We don't need to make this ConfigMap immutable since CoreDNS provides the "reload" plugins which does an
		// auto-reload if the config changes.
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				"Corefile": `.:` + strconv.Itoa(PortServer) + ` {
  errors
  log . {
      class error
  }
  health
  ready
  kubernetes ` + c.values.ClusterDomain + ` in-addr.arpa ip6.arpa {
      pods insecure
      fallthrough in-addr.arpa ip6.arpa
      ttl 30
  }
  prometheus 0.0.0.0:` + strconv.Itoa(portMetrics) + `
  forward . /etc/resolv.conf
  cache 30
  loop
  reload
  loadbalance round_robin
  import custom/*.override
}
import custom/*.server
`,
			},
		}

		configMapCustom = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "coredns-custom",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Ignore: "true"},
			},
			Data: map[string]string{
				"changeme.server":   "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md",
				"changeme.override": "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md",
			},
		}
	)

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		configMap,
		configMapCustom,
	)
}
