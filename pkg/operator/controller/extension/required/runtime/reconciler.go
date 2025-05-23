// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

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
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

// RequeueExtensionKindNotCalculated is the time after which an extension will be requeued if the extension kind has not been processed yet. Exposed for testing.
var RequeueExtensionKindNotCalculated = 2 * time.Second

// Reconciler reconciles Extensions to determine their required state.
type Reconciler struct {
	Client              client.Client
	Config              operatorconfigv1alpha1.ExtensionRequiredRuntimeControllerConfiguration
	Lock                *sync.RWMutex
	KindToRequiredTypes map[string]sets.Set[string]

	clock clock.Clock
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

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

	gardenList := &operatorv1alpha1.GardenList{}
	if err := r.Client.List(ctx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving Garden: %w", err)
	}

	garden := &operatorv1alpha1.Garden{}
	if len(gardenList.Items) > 0 {
		garden = &gardenList.Items[0]
	} else {
		log.Info("No Garden found")
	}

	requiredExtensionKindsBySpec, err := r.calculateRequiredResourceKindsBySpec(ctx, garden, extension)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateCondition(ctx, log, extension, requiredExtensionKinds.Union(requiredExtensionKindsBySpec)); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update extension status: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) calculateRequiredResourceKindsBySpec(ctx context.Context, garden *operatorv1alpha1.Garden, extension *operatorv1alpha1.Extension) (sets.Set[string], error) {
	// Extensions are not required anymore if the Garden is in deletion.
	if garden.DeletionTimestamp != nil {
		return nil, nil
	}

	extensionList := &operatorv1alpha1.ExtensionList{}
	if err := r.Client.List(ctx, extensionList); err != nil {
		return nil, fmt.Errorf("failed to retrieve extensions: %w", err)
	}

	var (
		requiredExtensionKinds = sets.New[string]()
		requiredExtensions     = operator.ComputeRequiredExtensionsForGarden(garden, extensionList)
	)

	for _, kindType := range requiredExtensions.UnsortedList() {
		extensionKind, extensionType, err := gardenerutils.ExtensionKindAndTypeForID(kindType)
		if err != nil {
			return nil, err
		}

		if v1beta1helper.IsResourceSupported(extension.Spec.Resources, extensionKind, extensionType) {
			requiredExtensionKinds.Insert(extensionKind)
		}
	}
	return requiredExtensionKinds, nil
}

func (r *Reconciler) updateCondition(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension, kinds sets.Set[string]) error {
	requiredRuntimeCondition := v1beta1helper.GetOrInitConditionWithClock(r.clock, extension.Status.Conditions, operatorv1alpha1.ExtensionRequiredRuntime)

	if len(kinds) > 0 {
		sortedKinds := slices.Sorted(slices.Values(kinds.UnsortedList()))
		requiredRuntimeCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredRuntimeCondition, gardencorev1beta1.ConditionTrue, "ExtensionRequired", fmt.Sprintf("Extension required for kinds %s", sortedKinds))
		log.Info("Extension required for garden runtime cluster", "kinds", sortedKinds)
	} else {
		requiredRuntimeCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredRuntimeCondition, gardencorev1beta1.ConditionFalse, "ExtensionNotRequired", "Extension not required for any kind")
		log.Info("Extension not required for garden runtime cluster")
	}

	patch := client.MergeFromWithOptions(extension.DeepCopy(), client.MergeFromWithOptimisticLock{})
	newConditions := v1beta1helper.MergeConditions(extension.Status.Conditions, requiredRuntimeCondition)
	if !v1beta1helper.ConditionsNeedUpdate(extension.Status.Conditions, newConditions) {
		return nil
	}

	extension.Status.Conditions = newConditions
	return r.Client.Status().Patch(ctx, extension, patch)
}
