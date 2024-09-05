// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/operator/controller/extension/admission"
	"github.com/gardener/gardener/pkg/operator/controller/extension/controllerregistration"
	"github.com/gardener/gardener/pkg/utils/oci"
)

const (
	// ConditionReconcileFailed is the condition type for when the virtual cluster resources fail to be reconciled.
	ConditionReconcileFailed = "ReconcileFailed"
	// ConditionDeleteFailed is the condition type for when the virtual cluster resources fail to be deleted.
	ConditionDeleteFailed = "DeleteFailed"
	// ConditionNoGardenFound is the condition type for when no Garden resource exists.
	ConditionNoGardenFound = "NoGardenFound"
	// ConditionReconcileSuccess is the condition type for when the virtual cluster resources successfully reconcile.
	ConditionReconcileSuccess = "ReconcileSuccessful"
	// ConditionDeleteSuccessful is the condition type for when the virtual cluster resources successfully delete.
	ConditionDeleteSuccessful = "DeleteSuccessful"
	// requeueGardenResourceNotReady is the time after which an extension will be requeued, if the Garden resource was not ready during its reconciliation.
	requeueGardenResourceNotReady = 10 * time.Second
)

// Reconciler reconciles Extensions.
type Reconciler struct {
	// GardenClientMap is the ClientMap used to communicate with the virtual garden cluster. It should be set by AddToManager function but the field is still public for usage in tests.
	GardenClientMap  clientmap.ClientMap
	RuntimeClientSet kubernetes.Interface

	Config          config.OperatorConfiguration
	Clock           clock.Clock
	Recorder        record.EventRecorder
	GardenNamespace string
	HelmRegistry    oci.Interface

	admission              admission.Interface
	controllerRegistration controllerregistration.Interface
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
	if len(gardenList.Items) == 0 {
		// in case a garden resource does not exist or is deleted, update the condition, remove the finalizers and exit early.
		log.Info("No Garden found")
		conditions := NewConditions(r.Clock, extension.Status)
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionNoGardenFound, "No garden found")
		if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
			log.Error(err, "Failed to update Extension status")
		}
		return reconcile.Result{}, r.removeFinalizer(ctx, log, extension)
	}

	garden := &gardenList.Items[0]
	// check Garden's last operation status. If the last operation is a successful reconciliation, we can proceed to install the extensions to the virtual garden cluster.
	switch lastOperation := garden.Status.LastOperation; {
	case lastOperation == nil || (lastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded):
		log.Info("Garden is not yet in 'Reconcile Succeeded' state, requeueing", "requeueAfter", requeueGardenResourceNotReady)
		return reconcile.Result{RequeueAfter: requeueGardenResourceNotReady}, nil
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete:
		// If the last operation is a delete, then do nothing. Once the Garden resource is deleted, we will reconcile and remove the finalizers from the Extension.
		// TODO(timuthy): Drop this handling and implement a proper removal procedure when the garden is deleted. Planned for release v1.103 or v1.104.
		return reconcile.Result{}, nil
	}

	virtualClusterClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving virtual cluster client set: %w", err)
	}

	if extension.DeletionTimestamp != nil {
		return reconcile.Result{}, r.delete(ctx, log, virtualClusterClientSet.Client(), extension)
	}

	genericTokenKubeconfigSecretName := garden.Annotations[v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName]
	if genericTokenKubeconfigSecretName == "" {
		return reconcile.Result{}, fmt.Errorf("error retrieving generic kubeconfig secret name from %q annotation of Garden", v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName)
	}

	return reconcile.Result{}, r.reconcile(ctx, log, virtualClusterClientSet, genericTokenKubeconfigSecretName, extension)
}

func (r *Reconciler) updateExtensionStatus(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension, updatedConditions Conditions) error {
	currentConditions := NewConditions(r.Clock, extension.Status)
	if extension.Generation == extension.Status.ObservedGeneration && !v1beta1helper.ConditionsNeedUpdate(currentConditions.ConvertToSlice(), updatedConditions.ConvertToSlice()) {
		return nil
	}

	patch := client.MergeFrom(extension.DeepCopy())
	// Rebuild garden currentConditions to ensure that only the currentConditions with the correct types will be updated, and any other
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

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, genericTokenKubeconfigSecretName string, extension *operatorv1alpha1.Extension) error {
	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	conditions := NewConditions(r.Clock, extension.Status)
	if !controllerutil.ContainsFinalizer(extension, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(reconcileCtx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.controllerRegistration.Reconcile(reconcileCtx, log, virtualClusterClientSet.Client(), extension); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	if err := r.admission.Reconcile(reconcileCtx, log, virtualClusterClientSet, genericTokenKubeconfigSecretName, extension); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionTrue, ConditionReconcileSuccess, fmt.Sprintf("Extension %q has been reconciled successfully", extension.Name))
	return r.updateExtensionStatus(ctx, log, extension, conditions)
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	deleteCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	conditions := NewConditions(r.Clock, extension.Status)

	if err := r.controllerRegistration.Delete(deleteCtx, log, virtualClusterClient, extension); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	if err := r.admission.Delete(deleteCtx, log, extension); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionDeleteSuccessful, "Successfully deleted runtime cluster resources")
	if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
		log.Error(err, "Failed to update extension status")
	}

	return r.removeFinalizer(ctx, log, extension)
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
