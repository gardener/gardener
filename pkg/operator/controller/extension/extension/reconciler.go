// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/admission"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/controllerregistration"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/runtime"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener/operator"
	"github.com/gardener/gardener/pkg/utils/oci"
)

const (
	// ReasonReconcileFailed indicates the reconciliation failed.
	ReasonReconcileFailed = "ReconcileFailed"
	// ReasonReconcileSuccess indicates the reconciliation succeeded.
	ReasonReconcileSuccess = "ReconcileSuccessful"
	// ReasonDeleteFailed indicates the deletion failed.
	ReasonDeleteFailed = "DeleteFailed"
	// ReasonDeleteSuccessful indicates the deletion failed.
	ReasonDeleteSuccessful = "DeleteSuccessful"
	// ReasonNoGardenFound indicates no Garden resource exists.
	ReasonNoGardenFound = "NoGardenFound"
	// ReasonInstalledInRuntime indicates the extension is installed in the garden runtime cluster.
	ReasonInstalledInRuntime = "InstalledInRuntime"
)

// Reconciler reconciles Extensions.
type Reconciler struct {
	RuntimeClientSet kubernetes.Interface
	GardenClientMap  clientmap.ClientMap
	Config           operatorconfigv1alpha1.OperatorConfiguration
	GardenNamespace  string
	HelmRegistry     oci.Interface
	Clock            clock.Clock
	Recorder         record.EventRecorder

	admission              admission.Interface
	controllerRegistration controllerregistration.Interface
	runtime                runtime.Interface
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	gardenList := &operatorv1alpha1.GardenList{}
	// We limit one result because we expect only a single Garden object to be there.
	if err := r.RuntimeClientSet.Client().List(ctx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving Garden object: %w", err)
	}

	var gardenObj *operatorv1alpha1.Garden
	if len(gardenList.Items) != 0 {
		gardenObj = &gardenList.Items[0]
	}

	garden := newGardenInfo(gardenObj)

	if extension.DeletionTimestamp != nil || garden.deleting {
		return r.delete(ctx, log, extension)
	}

	return r.reconcile(ctx, log, garden, extension)
}

func (r *Reconciler) updateExtensionStatus(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension, updatedConditions Conditions) error {
	currentConditions := NewConditions(r.Clock, extension.Status)
	if extension.Generation == extension.Status.ObservedGeneration && !v1beta1helper.ConditionsNeedUpdate(currentConditions.ConvertToSlice(), updatedConditions.ConvertToSlice()) {
		return nil
	}

	patch := client.MergeFromWithOptions(extension.DeepCopy(), client.MergeFromWithOptimisticLock{})
	// Rebuild extension currentConditions to ensure that only the currentConditions with the correct types will be updated, and any other
	// currentConditions will remain intact
	extension.Status.Conditions = v1beta1helper.BuildConditions(extension.Status.Conditions, updatedConditions.ConvertToSlice(), currentConditions.ConditionTypes())
	extension.Status.ObservedGeneration = extension.Generation

	// prevent sending empty patches
	if data, err := patch.Data(extension); err != nil {
		return fmt.Errorf("failed getting patch data for Extension: %w", err)
	} else if string(data) == "{}" {
		return nil
	}

	log.V(1).Info("Updating Extension status")
	if err := r.RuntimeClientSet.Client().Status().Patch(ctx, extension, patch); err != nil {
		return fmt.Errorf("could not update Extension status: %w", err)
	}

	return nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}

// Conditions contains all conditions of the extension status subresource.
type Conditions struct {
	installed gardencorev1beta1.Condition
}

// ConvertToSlice returns the garden conditions as a slice.
func (c Conditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		c.installed,
	}
}

// ConditionTypes returns all garden condition types.
func (c Conditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		c.installed.Type,
	}
}

// NewConditions returns a new instance of Conditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewConditions(clock clock.Clock, status operatorv1alpha1.ExtensionStatus) Conditions {
	return Conditions{
		installed: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.ExtensionInstalled),
	}
}

type gardenInfo struct {
	garden *operatorv1alpha1.Garden

	reconciled                       bool
	deleting                         bool
	genericTokenKubeconfigSecretName *string
}

func newGardenInfo(garden *operatorv1alpha1.Garden) *gardenInfo {
	if garden == nil {
		return &gardenInfo{
			reconciled: false,
			deleting:   false,
		}
	}

	var kubeconfigSecretName *string
	if name, ok := garden.Annotations[v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName]; ok {
		kubeconfigSecretName = &name
	}

	return &gardenInfo{
		garden:                           garden,
		reconciled:                       gardenerutils.IsGardenSuccessfullyReconciled(garden),
		deleting:                         gardenInDeletion(garden),
		genericTokenKubeconfigSecretName: kubeconfigSecretName,
	}
}

func gardenInDeletion(garden *operatorv1alpha1.Garden) bool {
	return garden.DeletionTimestamp != nil
}
