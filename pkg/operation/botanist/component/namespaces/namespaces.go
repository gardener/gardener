// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package namespaces

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
const ManagedResourceName = "shoot-core-namespaces"

// New creates a new instance of DeployWaiter for the namespaces.
func New(
	client client.Client,
	namespace string,
) component.DeployWaiter {
	return &namespaces{
		client:    client,
		namespace: namespace,
	}
}

type namespaces struct {
	client    client.Client
	namespace string
}

func (n *namespaces) Deploy(ctx context.Context) error {
	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, ManagedResourceName, true, data)
}

func (n *namespaces) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, ManagedResourceName)
}

func (n *namespaces) Wait(_ context.Context) error        { return nil }
func (n *namespaces) WaitCleanup(_ context.Context) error { return nil }

func (n *namespaces) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		kubeSystemNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   metav1.NamespaceSystem,
				Labels: getLabels(),
			},
		}
	)

	return registry.AddAllAndSerialize(kubeSystemNamespace)
}

func getLabels() map[string]string {
	return map[string]string{v1beta1constants.GardenerPurpose: metav1.NamespaceSystem}
}
