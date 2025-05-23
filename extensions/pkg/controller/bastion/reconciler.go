// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type reconciler struct {
	actuator        Actuator
	configValidator ConfigValidator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// bastion resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator, configValidator ConfigValidator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.Bastion{} },
		&reconciler{
			actuator:        actuator,
			configValidator: configValidator,
			client:          mgr.GetClient(),
			reader:          mgr.GetAPIReader(),
			statusUpdater:   extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	bastion := &extensionsv1alpha1.Bastion{}
	if err := r.client.Get(ctx, request.NamespacedName, bastion); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, bastion.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	operationType := v1beta1helper.ComputeOperationType(bastion.ObjectMeta, bastion.Status.LastOperation)

	switch {
	case bastion.DeletionTimestamp != nil:
		return r.delete(ctx, log, bastion, cluster)
	default:
		return r.reconcile(ctx, log, bastion, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	bastion *extensionsv1alpha1.Bastion,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(bastion, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, bastion, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, bastion, operationType, "Reconciling the Bastion"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.validateConfig(ctx, bastion, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bastion, err, operationType, "Error checking bastion config")
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of Bastion")
	if err := r.actuator.Reconcile(ctx, log, bastion, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bastion, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling Bastion")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bastion, operationType, "Successfully reconciled Bastion"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	bastion *extensionsv1alpha1.Bastion,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(bastion, FinalizerName) {
		log.Info("Deleting Bastion causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(bastion.ObjectMeta, bastion.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, bastion, operationType, "Deleting the Bastion"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of Bastion")
	var err error
	if kubernetesutils.HasMetaDataAnnotation(&bastion.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true") {
		err = r.actuator.ForceDelete(ctx, log, bastion, cluster)
	} else {
		err = r.actuator.Delete(ctx, log, bastion, cluster)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, bastion, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting Bastion")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bastion, operationType, "Successfully reconciled Bastion"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(bastion, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, bastion, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) validateConfig(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *extensions.Cluster) error {
	if r.configValidator == nil {
		return nil
	}

	if allErrs := r.configValidator.Validate(ctx, bastion, cluster); len(allErrs) > 0 {
		if filteredErrs := allErrs.Filter(field.NewErrorTypeMatcher(field.ErrorTypeInternal)); len(filteredErrs) < len(allErrs) {
			return allErrs.ToAggregate()
		}

		return v1beta1helper.NewErrorWithCodes(allErrs.ToAggregate(), gardencorev1beta1.ErrorConfigurationProblem)
	}

	return nil
}
