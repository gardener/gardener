// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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

var notSkippedSelector labels.Selector

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

// Reconcile checks the current node count and, if it changed across the >1 threshold, patches HA-relevant Deployments
// and StatefulSets whose hostname topology spread constraint requires adaptation to re-trigger the HA config webhook.
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

	log.Info("Re-triggering HA webhook for objects requiring adaptation", "multipleNodesAvailable", hasMultipleNodes)

	namespaceList := &metav1.PartialObjectMetadataList{}
	namespaceList.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "NamespaceList"})
	if err := r.TargetClient.List(ctx, namespaceList, client.MatchingLabels{resourcesv1alpha1.HighAvailabilityConfigConsider: "true"}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list HA-relevant namespaces: %w", err)
	}

	for _, namespace := range namespaceList.Items {
		for _, objectList := range []client.ObjectList{
			&appsv1.DeploymentList{},
			&appsv1.StatefulSetList{},
		} {
			if err := r.TargetClient.List(ctx, objectList, client.MatchingLabelsSelector{Selector: notSkippedSelector}, client.InNamespace(namespace.Name)); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to list %T: %w", objectList, err)
			}

			if err := meta.EachListItem(objectList, func(o runtime.Object) error {
				if !needsHostSpreadUpdate(topologySpreadConstraintsForObject(o), hasMultipleNodes) {
					return nil
				}

				obj := o.(client.Object)
				log.V(1).Info("Sending empty patch to trigger HA webhook", "objectKey", client.ObjectKeyFromObject(obj), "kind", fmt.Sprintf("%T", obj))
				return r.TargetClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte("{}")))
			}); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to handle %T: %w", objectList, err)
			}
		}
	}

	return reconcile.Result{}, nil
}

func topologySpreadConstraintsForObject(obj runtime.Object) []corev1.TopologySpreadConstraint {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return o.Spec.Template.Spec.TopologySpreadConstraints
	case *appsv1.StatefulSet:
		return o.Spec.Template.Spec.TopologySpreadConstraints
	default:
		return nil
	}
}

// needsHostSpreadUpdate returns true if the hostname topology spread constraint has a WhenUnsatisfiable value that
// doesn't match what the HA webhook would set given the current node count.
func needsHostSpreadUpdate(constraints []corev1.TopologySpreadConstraint, hasMultipleNodes bool) bool {
	for _, constraint := range constraints {
		if constraint.TopologyKey != corev1.LabelHostname {
			continue
		}
		if hasMultipleNodes {
			return constraint.WhenUnsatisfiable == corev1.ScheduleAnyway
		}
		return constraint.WhenUnsatisfiable == corev1.DoNotSchedule
	}
	return false
}
