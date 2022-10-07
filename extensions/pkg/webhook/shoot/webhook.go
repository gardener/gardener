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

package shoot

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileWebhookConfig deploys the shoot webhook configuration, i.e., a network policy to allow the
// kube-apiserver to talk to the extension, and a managed resource that contains the MutatingWebhookConfiguration.
func ReconcileWebhookConfig(
	ctx context.Context,
	c client.Client,
	namespace string,
	extensionName string,
	managedResourceName string,
	serverPort int,
	shootWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration,
	cluster *controller.Cluster,
) error {
	if err := EnsureNetworkPolicy(ctx, c, namespace, extensionName, serverPort); err != nil {
		return fmt.Errorf("could not create or update network policy for shoot webhooks in namespace '%s': %w", namespace, err)
	}

	if cluster.Shoot == nil {
		return fmt.Errorf("no shoot found in cluster resource")
	}

	data, err := managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(shootWebhookConfig)
	if err != nil {
		return err
	}

	if err := managedresources.Create(ctx, c, namespace, managedResourceName, nil, false, "", data, nil, nil, nil); err != nil {
		return fmt.Errorf("could not create or update managed resource '%s/%s' containing shoot webhooks: %w", namespace, managedResourceName, err)
	}

	return nil
}

// ReconcileWebhooksForAllNamespaces reconciles the shoot webhooks in all shoot namespaces of the given
// provider type. This is necessary in case the webhook port is changed (otherwise, the network policy would only be
// updated again as part of the ControlPlane reconciliation which might only happen in the next 24h).
func ReconcileWebhooksForAllNamespaces(
	ctx context.Context,
	c client.Client,
	extensionName string,
	managedResourceName string,
	shootNamespaceSelector map[string]string,
	port int,
	shootWebhookConfig *admissionregistrationv1.MutatingWebhookConfiguration,
) error {
	namespaceList := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaceList, client.MatchingLabels(utils.MergeStringMaps(map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	}, shootNamespaceSelector))); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(namespaceList.Items))

	for _, namespace := range namespaceList.Items {
		var (
			networkPolicy     = GetNetworkPolicyMeta(namespace.Name, extensionName)
			namespaceName     = namespace.Name
			networkPolicyName = networkPolicy.Name
		)

		fns = append(fns, func(ctx context.Context) error {
			if err := c.Get(ctx, kutil.Key(namespaceName, networkPolicyName), &networkingv1.NetworkPolicy{}); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
				return nil
			}

			cluster, err := extensions.GetCluster(ctx, c, namespaceName)
			if err != nil {
				return err
			}

			return ReconcileWebhookConfig(ctx, c, namespaceName, extensionName, managedResourceName, port, shootWebhookConfig.DeepCopy(), cluster)
		})
	}

	return flow.Parallel(fns...)(ctx)
}
