// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/utils/ptr"
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

// Reconciler patches HA-relevant Deployments and StatefulSets when the number of nodes or the number of distinct zones
// with schedulable system-component nodes in the cluster crosses the respective single-domain threshold. This
// re-triggers the HA config webhook which adjusts the topology spread constraints and node affinity based on the current
// node and zone count.
type Reconciler struct {
	TargetClient client.Client
	// ZoneCapNodeSelector, when non-nil, enables zone-set-change tracking. Must match the corresponding field on the
	// HA config webhook Handler — when the webhook does not cap zones, the controller must not re-patch on zone changes.
	ZoneCapNodeSelector  client.MatchingLabels
	lastHasMultipleNodes *bool
	lastHasMultipleZones *bool
}

// Reconcile checks the current node and zone count and, if either changed across its single-domain threshold, patches
// HA-relevant Deployments and StatefulSets whose topology spread constraints require adaptation to re-trigger the HA
// config webhook.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	hasMultipleNodes, err := kubernetesutils.HasMoreThanOneNode(ctx, r.TargetClient)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check whether cluster has more than one node: %w", err)
	}

	var hasMultipleZones bool
	if r.ZoneCapNodeSelector != nil {
		availableZones, err := kubernetesutils.AvailableZones(ctx, r.TargetClient, r.ZoneCapNodeSelector)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to determine available zones: %w", err)
		}
		hasMultipleZones = availableZones.Len() > 1
	}

	if ptr.Deref(r.lastHasMultipleNodes, false) == hasMultipleNodes && ptr.Deref(r.lastHasMultipleZones, false) == hasMultipleZones {
		log.V(1).Info("Neither node nor zone count crossed their threshold, nothing to do",
			"hasMultipleNodes", hasMultipleNodes,
			"hasMultipleZones", hasMultipleZones,
		)
		return reconcile.Result{}, nil
	}
	r.lastHasMultipleNodes, r.lastHasMultipleZones = &hasMultipleNodes, &hasMultipleZones

	log.Info("Re-triggering HA webhook for objects requiring adaptation",
		"multipleNodesAvailable", hasMultipleNodes,
		"multipleZonesAvailable", hasMultipleZones,
	)

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
				if constraints := topologySpreadConstraintsForObject(o); !needsHostSpreadUpdate(constraints, hasMultipleNodes) && !needsZoneSpreadUpdate(constraints, hasMultipleZones) {
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

// needsZoneSpreadUpdate returns true if the presence of a zone topology spread constraint doesn't match what the HA
// webhook would set given the current zone count. Only objects that already carry a hostname constraint are considered
// as those are the ones managed by the webhook. The webhook caps the zones to the available ones when less than two are
// available, so a zone constraint must exist if and only if multiple zones are available.
func needsZoneSpreadUpdate(constraints []corev1.TopologySpreadConstraint, hasMultipleZones bool) bool {
	var hasHostConstraint, hasZoneConstraint bool
	for _, constraint := range constraints {
		switch constraint.TopologyKey {
		case corev1.LabelHostname:
			hasHostConstraint = true
		case corev1.LabelTopologyZone:
			hasZoneConstraint = true
		}
	}

	if !hasHostConstraint {
		return false
	}
	return hasZoneConstraint != hasMultipleZones
}
