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
)

// Reconciler reconciles Extensions.
type Reconciler struct {
	RuntimeClient   client.Client
	Config          config.OperatorConfiguration
	Clock           clock.Clock
	Recorder        record.EventRecorder
	GardenClientMap clientmap.ClientMap
	GardenNamespace string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

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
		return reconcile.Result{}, fmt.Errorf("error retrieving garden object: %w", err)
	}
	if len(gardenList.Items) == 0 {
		// in case a garden resource does not exist or is deleted, update the condition, remove the finalizers and exit early.
		log.Info("No garden found")
		conditions := NewVirtualClusterConditions(r.Clock, extension.Status)
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionNoGardenFound, "no garden found")
		if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
			return reconcile.Result{}, fmt.Errorf("error updating extension status: %w", err)
		}
		return reconcile.Result{}, r.removeFinalizers(ctx, log, extension)
	}

	garden := &gardenList.Items[0]
	gardenClientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("error retrieving garden client object: %w", err)
	}

	if extension.DeletionTimestamp != nil {
		return reconcile.Result{}, r.delete(ctx, log, gardenClientSet.Client(), extension)
	}
	return reconcile.Result{}, r.reconcile(ctx, log, gardenClientSet.Client(), extension)
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
		return fmt.Errorf("failed getting patch data for Extension %s: %w", extension.Name, err)
	} else if string(data) == "{}" {
		return nil
	}

	log.V(1).Info("Updating extension status currentConditions")
	if err := r.RuntimeClient.Status().Patch(ctx, extension, patch); err != nil {
		log.Error(err, "Could not update extension status")
		return err
	}

	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	conditions := NewVirtualClusterConditions(r.Clock, extension.Status)
	if !controllerutil.ContainsFinalizer(extension, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.RuntimeClient, extension, operatorv1alpha1.FinalizerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	log.Info("Reconciling extension virtual resources", "name", extension.Name)
	err := r.reconcileVirtualClusterResources(ctx, log, gardenClient, extension)
	if err != nil {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionReconcileFailed, err.Error())
	} else {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionTrue, ConditionReconcileSuccess, fmt.Sprintf("Extension %q has been reconciled successfully", extension.Name))
	}

	return errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
}

func (r *Reconciler) reconcileVirtualClusterResources(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	// return early if we do not have to make a deployment
	if extension.Spec.Deployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment == nil ||
		extension.Spec.Deployment.ExtensionDeployment.Helm == nil {
		return r.deleteVirtualClusterResources(ctx, log, gardenClient, extension)
	}

	if err := r.reconcileControllerDeployment(ctx, gardenClient, extension); err != nil {
		err := fmt.Errorf("failed to reconciler controller deployment: %w", err)
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerDeployment applied successfully")

	if err := r.reconcileControllerRegistration(ctx, gardenClient, extension); err != nil {
		err := fmt.Errorf("failed to reconciler controller registration: %w", err)
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Reconciliation", "ControllerRegistration applied successfully")
	return nil
}

func (r *Reconciler) reconcileControllerDeployment(ctx context.Context, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	ctrlDeploy := &gardencorev1.ControllerDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	deployMutateFn := func() error {
		ctrlDeploy.Helm = &gardencorev1.HelmControllerDeployment{
			Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
			OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
		}
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlDeploy, deployMutateFn); err != nil {
		return fmt.Errorf("failed to create or update ControllerDeployment: %w", err)
	}
	return nil
}

func (r *Reconciler) reconcileControllerRegistration(ctx context.Context, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	ctrlReg := &gardencorev1beta1.ControllerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: extension.Name,
		},
	}

	regMutateFn := func() error {
		// handle well known annotations
		if v, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataAnnotation(&ctrlReg.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, v)
		} else {
			delete(ctrlReg.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
		}

		ctrlReg.Spec = gardencorev1beta1.ControllerRegistrationSpec{
			Resources: extension.Spec.Resources,
			Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
				Policy: extension.Spec.Deployment.ExtensionDeployment.Policy,
				DeploymentRefs: []gardencorev1beta1.DeploymentRef{
					{
						Name: extension.Name,
					},
				},
			},
		}
		return nil
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, gardenClient, ctrlReg, regMutateFn); err != nil {
		return fmt.Errorf("failed to create or update ControllerRegistration: %w", err)
	}
	return nil
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	conditions := NewVirtualClusterConditions(r.Clock, extension.Status)

	err := r.deleteVirtualClusterResources(ctx, log, gardenClient, extension)
	if err != nil {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
	} else {
		conditions.virtualClusterReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.virtualClusterReconciled, gardencorev1beta1.ConditionFalse, ConditionDeleteSuccessful, "successfully deleted virtual cluster resources")
	}

	if err := errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions)); err != nil {
		return err
	}

	return r.removeFinalizers(ctx, log, extension)
}

func (r *Reconciler) deleteVirtualClusterResources(ctx context.Context, log logr.Logger, gardenClient client.Client, extension *operatorv1alpha1.Extension) error {
	log.Info("Deleting extension virtual resources", "name", extension.Name)
	var (
		ctrlDeploy = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			}}

		ctrlReg = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
		}
	)

	// deleting the controller deployment first to set the termination timestamp on the object. The deletion will be complete once the
	// controllerRegistration has been deleted.
	log.Info("Deleting controller deployment for extension", "extension", extension.Name)
	if err := kubernetesutils.DeleteObject(ctx, gardenClient, ctrlReg); err != nil {
		return err
	}

	log.Info("Deleting controller registration for extension", "extension", extension.Name)
	if err := kubernetesutils.DeleteObject(ctx, gardenClient, ctrlDeploy); err != nil {
		return err
	}

	log.Info("Waiting until controller registration is gone", "extension", extension.Name)
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, gardenClient, ctrlReg, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted controller registration")

	log.Info("Waiting until controller deployment is gone", "extension", extension.Name)
	if err := kubernetesutils.WaitUntilResourceDeleted(ctx, gardenClient, ctrlDeploy, 5*time.Second); err != nil {
		return err
	}
	r.Recorder.Event(extension, corev1.EventTypeNormal, "Deletion", "Successfully deleted controller deployment")
	return nil
}

func (r *Reconciler) removeFinalizers(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
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
