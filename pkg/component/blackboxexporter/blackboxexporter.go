// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package blackboxexporter

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-blackbox-exporter"

// Interface contains functions for a blackbox-exporter deployer.
type Interface interface {
	component.DeployWaiter
}

// Values is a set of configuration values for the blackbox-exporter.
type Values struct{}

// New creates a new instance of DeployWaiter for blackbox-exporter.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &blackboxExporter{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type blackboxExporter struct {
	client    client.Client
	namespace string
	values    Values
}

func (b *blackboxExporter) Deploy(ctx context.Context) error {
	data, err := b.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, b.client, b.namespace, ManagedResourceName, managedresources.LabelValueGardener, false, data)
}

func (b *blackboxExporter) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, b.client, b.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *blackboxExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, ManagedResourceName)
}

func (b *blackboxExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, ManagedResourceName)
}

func (b *blackboxExporter) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
					"component":                 "blackbox-exporter",
					"origin":                    "gardener",
				},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "blackbox-exporter-config",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.LabelApp:  "prometheus",
					v1beta1constants.LabelRole: v1beta1constants.GardenRoleMonitoring,
				},
			},
			Data: map[string]string{
				`blackbox.yaml`: `modules:
  http_kubernetes_service:
    prober: http
    timeout: 10s
    http:
      headers:
        Accept: "*/*"
        Accept-Language: "en-US"
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      preferred_ip_protocol: "ip4"
`,
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return registry.AddAllAndSerialize(
		serviceAccount,
		configMap,
	)
}
