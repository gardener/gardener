// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var extensionKindToObjectList = map[string]client.ObjectList{
	extensionsv1alpha1.BackupBucketResource: &extensionsv1alpha1.BackupBucketList{},
	extensionsv1alpha1.DNSRecordResource:    &extensionsv1alpha1.DNSRecordList{},
	extensionsv1alpha1.ExtensionResource:    &extensionsv1alpha1.ExtensionList{},
}

// RequeueDurationWhenGardenIsBeingDeleted is the duration after the request will be requeued when the Garden is being deleted.
// Exposed for testing.
var RequeueDurationWhenGardenIsBeingDeleted = 2 * time.Second

// Reconciler reconciles Extensions to determine their required state.
type Reconciler struct {
	Client client.Client
	Config operatorconfigv1alpha1.ExtensionRequiredRuntimeControllerConfiguration

	clock clock.Clock
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	extension := &operatorv1alpha1.Extension{}
	if err := r.Client.Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
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

	requiredExtensionKinds, err := r.calculateRequiredResourceKinds(ctx, log, r.Client, garden, extension)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateCondition(ctx, log, extension, requiredExtensionKinds); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update extension status: %w", err)
	}

	if len(requiredExtensionKinds) > 0 && garden.DeletionTimestamp != nil {
		return reconcile.Result{RequeueAfter: RequeueDurationWhenGardenIsBeingDeleted}, nil
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) calculateRequiredResourceKinds(ctx context.Context, log logr.Logger, c client.Client, garden *operatorv1alpha1.Garden, extension *operatorv1alpha1.Extension) (sets.Set[string], error) {
	var (
		requiredExtensionKinds = sets.New[string]()
		requiredExtensions     = gardenerutils.ComputeRequiredExtensionsForGarden(garden)
	)

	for _, kindType := range requiredExtensions.UnsortedList() {
		extensionKind, extensionType, err := gardenerutils.ExtensionKindAndTypeForID(kindType)
		if err != nil {
			return nil, err
		}

		if v1beta1helper.IsResourceSupported(extension.Spec.Resources, extensionKind, extensionType) {
			// The extension is not required anymore if the Garden is in deletion and resources are gone.
			if garden.DeletionTimestamp != nil {
				objList, ok := extensionKindToObjectList[extensionKind].DeepCopyObject().(client.ObjectList)
				if !ok {
					return nil, fmt.Errorf("extension kind %s unknown", extensionKind)
				}

				resourcesExist, err := kubernetesutils.ResourcesExist(ctx, c, objList, c.Scheme())
				if meta.IsNoMatchError(err) {
					continue
				} else if err != nil {
					return nil, err
				}

				if !resourcesExist {
					continue
				} else {
					log.Info("At least one extension is still present", "kind", extensionKind)
				}
			}
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
