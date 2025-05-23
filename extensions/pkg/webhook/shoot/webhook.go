// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
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
	managedResourceName string,
	shootWebhookConfigs webhook.Configs,
	cluster *controller.Cluster,
	createIfNotExists bool,
) error {
	if cluster.Shoot == nil {
		return fmt.Errorf("no shoot found in cluster resource")
	}

	data, err := managedresources.
		NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
		AddAllAndSerialize(shootWebhookConfigs.GetWebhookConfigs()...)
	if err != nil {
		return err
	}

	f := managedresources.Create
	if !createIfNotExists {
		f = managedresources.Update
	}

	if err := f(ctx, c, shootNamespace, managedResourceName, nil, false, "", data, nil, nil, nil); err != nil {
		return fmt.Errorf("failed reconciling managed resource '%s/%s' containing shoot webhooks: %w", shootNamespace, managedResourceName, err)
	}
	return nil
}

// ReconcileWebhooksForAllNamespaces reconciles the shoot webhooks in all shoot namespaces of the given
// provider type. This is necessary in case the webhook port is changed (otherwise, the network policy would only be
// updated again as part of the ControlPlane reconciliation which might only happen in the next 24h).
func ReconcileWebhooksForAllNamespaces(
	ctx context.Context,
	c client.Client,
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

			// Ignore not found errors since the managed resource can be deleted in parallel during shoot deletion.
			if err := ReconcileWebhookConfig(ctx, c, namespaceName, managedResourceName, *shootWebhookConfigs.DeepCopy(), cluster, false); client.IgnoreNotFound(err) != nil {
				return err
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}
