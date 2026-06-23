// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ControllerName is the name of this controller.
const ControllerName = "self-hosted-shoot-exposure"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.ShootClient == nil {
		r.ShootClient = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	// NOTE: ControllerRegistrations are intentionally not watched. A self-hosted gardenlet may not List/Watch them
	// (RBAC, hence endpointUpdatesEnabled fetches them by name, indirectly via ControllerInstallations).
	return builder.
		ControllerManagedBy(mgr).
		// Only one Shoot is managed, so there is nothing to parallelize.
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Named(ControllerName).
		WatchesRawSource(source.Kind[client.Object](mgr.GetCache(),
			&corev1.Node{},
			&handler.EnqueueRequestForObject{},
			r.NodePredicate(),
		)).
		// Watch the Shoot so switching the exposure mechanism (or omitting exposure) reconciles without a Node event.
		WatchesRawSource(source.Kind[client.Object](gardenCluster.GetCache(),
			&gardencorev1beta1.Shoot{},
			&handler.EnqueueRequestForObject{},
			r.ShootExposureChangePredicate(),
		)).
		// Watch the SelfHostedShootExposure for ingress changes by the exposure extension (e.g. a rotated LB IP).
		WatchesRawSource(source.Kind[client.Object](mgr.GetCache(),
			&extensionsv1alpha1.SelfHostedShootExposure{},
			&handler.EnqueueRequestForObject{},
			r.ExposureIngressChangePredicate(),
		)).
		Complete(r)
}

// NodePredicate triggers on control-plane Node events relevant to exposure: create/delete of a control-plane Node,
// gaining/losing the control-plane label (after registration), and address or health-verdict changes on one.
func (r *Reconciler) NodePredicate() predicate.Predicate {
	isControlPlane := func(obj client.Object) bool {
		_, ok := obj.GetLabels()[v1beta1constants.LabelNodeRoleControlPlane]
		return ok
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isControlPlane(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return isControlPlane(e.Object) },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, ok1 := e.ObjectOld.(*corev1.Node)
			newNode, ok2 := e.ObjectNew.(*corev1.Node)
			if !ok1 || !ok2 {
				return false
			}
			// React only if the node is (or was) a control-plane node. A label-membership change always matters;
			// otherwise only address or health-verdict changes do.
			oldCP, newCP := isControlPlane(oldNode), isControlPlane(newNode)
			if !oldCP && !newCP {
				return false
			}
			if oldCP != newCP || !slices.Equal(oldNode.Status.Addresses, newNode.Status.Addresses) {
				return true
			}
			return (health.CheckNode(oldNode) == nil) != (health.CheckNode(newNode) == nil)
		},
	}
}

// ExposureIngressChangePredicate triggers only when the ingress reported by the exposure extension changes.
func (r *Reconciler) ExposureIngressChangePredicate() predicate.Predicate {
	ingressOf := func(obj client.Object) []corev1.LoadBalancerIngress {
		if exposure, ok := obj.(*extensionsv1alpha1.SelfHostedShootExposure); ok {
			return exposure.Status.Ingress
		}
		return nil
	}
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !apiequality.Semantic.DeepEqual(ingressOf(e.ObjectOld), ingressOf(e.ObjectNew))
		},
	}
}

// ShootExposureChangePredicate triggers only when the control-plane exposure configuration changes (or the Shoot first
// appears), so an unrelated Shoot update does not enqueue a reconcile.
func (r *Reconciler) ShootExposureChangePredicate() predicate.Predicate {
	exposureOf := func(obj client.Object) *gardencorev1beta1.Exposure {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil
		}
		if pool := v1beta1helper.ControlPlaneWorkerPoolForShoot(shoot.Spec.Provider.Workers); pool != nil && pool.ControlPlane != nil {
			return pool.ControlPlane.Exposure
		}
		return nil
	}
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !apiequality.Semantic.DeepEqual(exposureOf(e.ObjectOld), exposureOf(e.ObjectNew))
		},
	}
}
