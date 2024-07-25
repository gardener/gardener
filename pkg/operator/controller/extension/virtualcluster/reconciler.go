// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtualcluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	RuntimeClient   client.Client
	Config          config.OperatorConfiguration
	Clock           clock.Clock
	Recorder        record.EventRecorder
	GardenNamespace string
	// GardenClientMap is the ClientMap used to communicate with the virtual garden cluster. It should be set by AddToManager function but the field is still public for use in tests.
	GardenClientMap clientmap.ClientMap
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	gardenList := &operatorv1alpha1.GardenList{}
	// We limit one result because we expect only a single Garden object to be there.
	if err := r.RuntimeClient.List(ctx, gardenList, client.Limit(1)); err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving Garden object: %w", err)
	}
	if len(gardenList.Items) == 0 {
		// in case a garden resource does not exist or is deleted, update the condition, remove the finalizers and exit early.
		log.Info("No Garden found")
		conditions := NewVirtualClusterConditions(r.Clock, extension.Status)
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionNoGardenFound, "no garden found")
		if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
			log.Error(err, "Failed to update Extension status")
		}
		return reconcile.Result{}, r.removeFinalizer(ctx, log, extension)
	}

	garden := &gardenList.Items[0]
	// check Garden's last operation status. If the last operation is a successful reconciliation, we can proceed to install the extensions to the virtual garden cluster.
	switch lastOperation := garden.Status.LastOperation; {
	case lastOperation == nil || (lastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded):
		return reconcile.Result{}, &reconciler.RequeueAfterError{
			RequeueAfter: requeueGardenResourceNotReady,
		}
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete:
		// if the last operation is a delete, then do nothing. Once the Garden resource is deleted, we will reconcile and remove the finalizers from the Extension.
		return reconcile.Result{}, nil
	}

	virtualClusterClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving virtual cluster client set: %w", err)
	}

	if extension.DeletionTimestamp != nil {
		return reconcile.Result{}, r.delete(ctx, log, virtualClusterClientSet.Client(), extension)
	}
	return reconcile.Result{}, r.reconcile(ctx, log, virtualClusterClientSet.Client(), extension)
}

func (r *Reconciler) updateExtensionStatus(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension, updatedConditions VirtualClusterConditions) error {
	currentConditions := NewVirtualClusterConditions(r.Clock, extension.Status)
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
	if err := r.RuntimeClient.Status().Patch(ctx, extension, patch); err != nil {
		return fmt.Errorf("could not update Extension status: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	conditions := NewVirtualClusterConditions(r.Clock, extension.Status)
	if !controllerutil.ContainsFinalizer(extension, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClient, extension, operatorv1alpha1.FinalizerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	log.Info("Reconciling extension virtual resources")
	if err := r.reconcileVirtualClusterResources(ctx, log, virtualClusterClient, extension); err != nil {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionTrue, ConditionReconcileSuccess, fmt.Sprintf("Extension %q has been reconciled successfully", extension.Name))
	return r.updateExtensionStatus(ctx, log, extension, conditions)
}

func (r *Reconciler) reconcileVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	// return early if we do not have to make a deployment
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		return r.deleteVirtualClusterResources(ctx, log, virtualClusterClient, extension)
	}

	if err := r.reconcileControllerDeployment(ctx, virtualClusterClient, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerDeployment: %w", err)
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerDeployment applied successfully")

	if err := r.reconcileControllerRegistration(ctx, virtualClusterClient, extension); err != nil {
		return fmt.Errorf("failed to reconcile ControllerRegistration: %w", err)
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration applied successfully")
	return nil
}

func (r *Reconciler) reconcileControllerDeployment(ctx context.Context, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	controllerDeployment := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerDeployment,
		func() error {
			controllerDeployment.Helm = &gardencorev1.HelmControllerDeployment{
				Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
				OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
			}
			return nil
		})
	return err
}

func (r *Reconciler) reconcileControllerRegistration(ctx context.Context, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	controllerRegistration := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, virtualClusterClient, controllerRegistration,
		func() error {
			// handle well known annotations
			if v, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
				metav1.SetMetaDataAnnotation(&controllerRegistration.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, v)
			} else {
				delete(controllerRegistration.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
			}

			controllerRegistration.Spec = gardencorev1beta1.ControllerRegistrationSpec{
				Resources: extension.Spec.Resources,
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy:       extension.Spec.Deployment.ExtensionDeployment.Policy,
					SeedSelector: extension.Spec.Deployment.ExtensionDeployment.SeedSelector,
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{
							Name: extension.Name,
						},
					},
				},
			}
			return nil
		})
	return err
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	conditions := NewVirtualClusterConditions(r.Clock, extension.Status)

	if err := r.deleteVirtualClusterResources(ctx, log, virtualClusterClient, extension); err != nil {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteSuccessful, "successfully deleted virtual cluster resources")
	if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
		log.Error(err, "Failed to update extension status")
	}

	return r.removeFinalizer(ctx, log, extension)
}

func (r *Reconciler) deleteVirtualClusterResources(ctx context.Context, log logr.Logger, virtualClusterClient client.Client, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting extension virtual resources")
	var (
		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			}}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
		}
	)

	log.Info("Deleting ControllerRegistration and ControllerDeployment")
	if err := kubernetesutils.DeleteObjects(ctx, virtualClusterClient, controllerDeployment, controllerRegistration); err != nil {
		return err
	}

	log.Info("Waiting until ControllerRegistration is gone")
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, virtualClusterClient, controllerRegistration, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted ControllerRegistration")

	log.Info("Waiting until ControllerDeployment is gone")
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, virtualClusterClient, controllerDeployment, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted ControllerDeployment")
	return nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	log.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClient, extension, operatorv1alpha1.FinalizerName); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}

// VirtualClusterConditions contains all conditions of the extension status subresource.
type VirtualClusterConditions struct {
	virtualClusterReconciled gardencorev1beta1.Condition
}

// ConvertToSlice returns the garden conditions as a slice.
func (vc VirtualClusterConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		vc.virtualClusterReconciled,
	}
}

// ConditionTypes returns all garden condition types.
func (vc VirtualClusterConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		vc.virtualClusterReconciled.Type,
	}
}

// NewVirtualClusterConditions returns a new instance of VirtualClusterConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewVirtualClusterConditions(clock clock.Clock, status operatorv1alpha1.ExtensionStatus) VirtualClusterConditions {
	return VirtualClusterConditions{
		virtualClusterReconciled: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.VirtualClusterExtensionReconciled),
	}
}
