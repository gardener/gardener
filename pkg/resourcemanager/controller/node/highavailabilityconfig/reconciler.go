// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	notSkippedSelector labels.Selector

	// objectGVKsToRetrigger is the list of object kinds that the controller will empty-patch to re-trigger the HA
	// config webhook when the node count changes.
	objectGVKsToRetrigger = []schema.GroupVersionKind{
		appsv1.SchemeGroupVersion.WithKind("Deployment"),
		appsv1.SchemeGroupVersion.WithKind("StatefulSet"),
	}
)

func init() {
	notSkipped, err := labels.NewRequirement(resourcesv1alpha1.HighAvailabilityConfigSkip, selection.DoesNotExist, nil)
	utilruntime.Must(err)
	notSkippedSelector = labels.NewSelector().Add(*notSkipped)
}

// Reconciler patches HA-relevant Deployments and StatefulSets when the number of nodes in the cluster crosses the
// single-node threshold. This re-triggers the HA config webhook which adjusts the topology spread constraints based on
// the current node count.
type Reconciler struct {
	TargetClient         client.Client
	lastHasMultipleNodes *bool
}

// Reconcile checks the current node count and, if it changed across the >1 threshold, patches all HA-relevant
// Deployments and StatefulSets to re-trigger the HA config webhook.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	hasMultipleNodes, err := kubernetesutils.HasMoreThanOneNode(ctx, r.TargetClient)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check whether cluster has more than one node: %w", err)
	}

	if r.lastHasMultipleNodes != nil && *r.lastHasMultipleNodes == hasMultipleNodes {
		log.V(1).Info("Node count did not cross the single-node threshold, nothing to do", "hasMultipleNodes", hasMultipleNodes)
		return reconcile.Result{}, nil
	}
	r.lastHasMultipleNodes = &hasMultipleNodes

	log.Info("Re-triggering HA webhook because node count changed", "multipleNodesAvailable", hasMultipleNodes)

	namespaceList := &metav1.PartialObjectMetadataList{}
	namespaceList.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "NamespaceList"})
	if err := r.TargetClient.List(ctx, namespaceList, client.MatchingLabels{resourcesv1alpha1.HighAvailabilityConfigConsider: "true"}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list HA-relevant namespaces: %w", err)
	}

	emptyPatch := client.RawPatch(types.MergePatchType, []byte("{}"))

	for _, namespace := range namespaceList.Items {
		for _, gvk := range objectGVKsToRetrigger {
			objectList := &metav1.PartialObjectMetadataList{}
			objectList.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			if err := r.TargetClient.List(ctx, objectList, client.InNamespace(namespace.Name), client.MatchingLabelsSelector{Selector: notSkippedSelector}); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to list %ss in namespace %q: %w", gvk.Kind, namespace.Name, err)
			}

			for i := range objectList.Items {
				log.V(1).Info("Patching object", "kind", gvk.Kind, "namespace", objectList.Items[i].Namespace, "name", objectList.Items[i].Name)
				if err := r.TargetClient.Patch(ctx, &objectList.Items[i], emptyPatch); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to patch %s %s/%s: %w", gvk.Kind, objectList.Items[i].Namespace, objectList.Items[i].Name, err)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
