// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// ReconcileWebhookConfig deploys the shoot webhook configuration, i.e., a network policy to allow the
// kube-apiserver to talk to the extension, and a managed resource that contains the MutatingWebhookConfiguration.
func ReconcileWebhookConfig(
	ctx context.Context,
	c client.Client,
	shootNamespace string,
	extensionNamespace string,
	extensionName string,
	managedResourceName string,
	shootWebhookConfigs webhook.Configs,
	cluster *controller.Cluster,
) error {
	if cluster.Shoot == nil {
		return fmt.Errorf("no shoot found in cluster resource")
	}

	// TODO(rfranzke): Remove this after Gardener v1.86 has been released.
	{
		if err := c.Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: shootNamespace, Name: "gardener-extension-" + extensionName}}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete old egress network policy for shoot webhooks in namespace '%s': %w", shootNamespace, err)
		}
		if err := c.Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: extensionNamespace, Name: "ingress-from-all-shoots-kube-apiserver"}}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not delete old ingress network policy for shoot webhooks in namespace '%s': %w", extensionNamespace, err)
		}
	}

	data, err := managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(shootWebhookConfigs.GetWebhookConfigs()...)
	if err != nil {
		return err
	}

	if err := managedresources.Create(ctx, c, shootNamespace, managedResourceName, nil, false, "", data, nil, nil, nil); err != nil {
		return fmt.Errorf("could not create or update managed resource '%s/%s' containing shoot webhooks: %w", shootNamespace, managedResourceName, err)
	}

	return nil
}

// ReconcileWebhooksForAllNamespaces reconciles the shoot webhooks in all shoot namespaces of the given
// provider type. This is necessary in case the webhook port is changed (otherwise, the network policy would only be
// updated again as part of the ControlPlane reconciliation which might only happen in the next 24h).
func ReconcileWebhooksForAllNamespaces(
	ctx context.Context,
	c client.Client,
	extensionNamespace string,
	extensionName string,
	managedResourceName string,
	shootNamespaceSelector map[string]string,
	shootWebhookConfigs webhook.Configs,
) error {
	namespaceList := &corev1.NamespaceList{}
	if err := c.List(ctx, namespaceList, client.MatchingLabels(utils.MergeStringMaps(map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	}, shootNamespaceSelector))); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(namespaceList.Items)+1)

	for _, namespace := range namespaceList.Items {
		namespaceName := namespace.Name

		fns = append(fns, func(ctx context.Context) error {
			managedResource := &metav1.PartialObjectMetadata{}
			managedResource.SetGroupVersionKind(resourcesv1alpha1.SchemeGroupVersion.WithKind("ManagedResource"))
			if err := c.Get(ctx, client.ObjectKey{Name: managedResourceName, Namespace: namespaceName}, managedResource); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			cluster, err := extensions.GetCluster(ctx, c, namespaceName)
			if err != nil {
				return err
			}

			return ReconcileWebhookConfig(ctx, c, namespaceName, extensionNamespace, extensionName, managedResourceName, *shootWebhookConfigs.DeepCopy(), cluster)
		})
	}

	return flow.Parallel(fns...)(ctx)
}
