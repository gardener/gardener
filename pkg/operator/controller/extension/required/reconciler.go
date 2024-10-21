// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/apis/config"
)

// RequeueExtensionKindNotCalculated is the time after which an extension will be requeued if the extension kind has not been processed yet. Exposed for testing.
var RequeueExtensionKindNotCalculated = 2 * time.Second

// Reconciler reconciles Extensions to determine their required state.
type Reconciler struct {
	Client client.Client
	Config *config.OperatorConfiguration

	Lock                *sync.RWMutex
	KindToRequiredTypes map[string]sets.Set[string]

	clock clock.Clock
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	for _, ext := range runtimeClusterExtensions {
		r.Lock.RLock()
		_, kindProcessed := r.KindToRequiredTypes[ext.objectKind]
		r.Lock.RUnlock()
		if !kindProcessed {
			// The object kind in question has not yet been processed.
			// Hence, it's not possible to determine if the extension is required or not.
			log.Info("Kind is not yet calculated. Request is re-queued", "kind", ext.objectKind)
			return reconcile.Result{RequeueAfter: RequeueExtensionKindNotCalculated}, nil
		}
	}

	extension := &operatorv1alpha1.Extension{}
	if err := r.Client.Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	requiredExtensionKinds := sets.New[string]()
	for _, resource := range extension.Spec.Resources {
		r.Lock.RLock()
		requiredTypes, ok := r.KindToRequiredTypes[resource.Kind]
		r.Lock.RUnlock()
		if !ok {
			continue
		}

		if requiredTypes.Has(resource.Type) {
			requiredExtensionKinds.Insert(resource.Kind)
		}
	}

	requiredRuntimeCondition := r.getRuntimeCondition(log, extension, requiredExtensionKinds)
	if err := r.updateConditions(ctx, extension, requiredRuntimeCondition); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update extension status: %w", err)
	}

	return reconcile.Result{}, nil
}

const (
	// ExtensionRequiredReason is the reason to indicate that the extension is required.
	ExtensionRequiredReason = "ExtensionRequired"
	// ExtensionNotRequiredReason is the reason to indicate that the extension is not required.
	ExtensionNotRequiredReason = "ExtensionNotRequired"
)

func (r *Reconciler) getRuntimeCondition(log logr.Logger, extension *operatorv1alpha1.Extension, kinds sets.Set[string]) gardencorev1beta1.Condition {
	requiredRuntimeCondition := v1beta1helper.GetOrInitConditionWithClock(r.clock, extension.Status.Conditions, operatorv1alpha1.ExtensionRequiredRuntime)

	if len(kinds) > 0 {
		sortedKinds := slices.Sorted(slices.Values(kinds.UnsortedList()))
		requiredRuntimeCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredRuntimeCondition, gardencorev1beta1.ConditionTrue, ExtensionRequiredReason, fmt.Sprintf("Extension required for kinds %s", sortedKinds))
		log.Info("Extension required for garden runtime cluster", "kinds", sortedKinds)
	} else {
		requiredRuntimeCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredRuntimeCondition, gardencorev1beta1.ConditionFalse, ExtensionNotRequiredReason, "Extension not required for any kind")
		log.Info("Extension not required for garden runtime cluster")
	}

	return requiredRuntimeCondition
}

func (r *Reconciler) updateConditions(ctx context.Context, extension *operatorv1alpha1.Extension, conditions ...gardencorev1beta1.Condition) error {
	patch := client.MergeFromWithOptions(extension.DeepCopy(), client.MergeFromWithOptimisticLock{})
	newConditions := v1beta1helper.MergeConditions(extension.Status.Conditions, conditions...)
	if !v1beta1helper.ConditionsNeedUpdate(extension.Status.Conditions, newConditions) {
		return nil
	}

	extension.Status.Conditions = newConditions
	return r.Client.Status().Patch(ctx, extension, patch)
}
