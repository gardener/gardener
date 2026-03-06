// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// selfhostedshootexposure resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.SelfHostedShootExposure{} },
		&reconciler{
			actuator:      actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	selfHostedShootExposure := &extensionsv1alpha1.SelfHostedShootExposure{}
	if err := r.client.Get(ctx, request.NamespacedName, selfHostedShootExposure); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, selfHostedShootExposure.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	operationType := v1beta1helper.ComputeOperationType(selfHostedShootExposure.ObjectMeta, selfHostedShootExposure.Status.LastOperation)

	switch {
	case selfHostedShootExposure.DeletionTimestamp != nil:
		return r.delete(ctx, log, selfHostedShootExposure, cluster)
	default:
		return r.reconcile(ctx, log, selfHostedShootExposure, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	selfHostedShootExposure *extensionsv1alpha1.SelfHostedShootExposure,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(selfHostedShootExposure, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, selfHostedShootExposure, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, selfHostedShootExposure, operationType, "Reconciling the SelfHostedShootExposure"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of SelfHostedShootExposure")
	loadBalancerIngresses, err := r.actuator.Reconcile(ctx, log, selfHostedShootExposure, cluster)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, selfHostedShootExposure, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling SelfHostedShootExposure")
		return reconcilerutils.ReconcileErr(err)
	}

	if len(loadBalancerIngresses) > 0 {
		patch := client.MergeFrom(selfHostedShootExposure.DeepCopy())
		selfHostedShootExposure.Status.Ingress = loadBalancerIngresses
		if err := r.client.Status().Patch(ctx, selfHostedShootExposure, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status ingress: %w", err)
		}
	}

	if err := r.statusUpdater.Success(ctx, log, selfHostedShootExposure, operationType, "Successfully reconciled SelfHostedShootExposure"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	selfHostedShootExposure *extensionsv1alpha1.SelfHostedShootExposure,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(selfHostedShootExposure, FinalizerName) {
		log.Info("Deleting SelfHostedShootExposure causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(selfHostedShootExposure.ObjectMeta, selfHostedShootExposure.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, selfHostedShootExposure, operationType, "Deleting the SelfHostedShootExposure"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of SelfHostedShootExposure")
	var err error
	if kubernetesutils.HasMetaDataAnnotation(&selfHostedShootExposure.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true") {
		err = r.actuator.ForceDelete(ctx, log, selfHostedShootExposure, cluster)
	} else {
		err = r.actuator.Delete(ctx, log, selfHostedShootExposure, cluster)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, selfHostedShootExposure, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting SelfHostedShootExposure")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, selfHostedShootExposure, operationType, "Successfully reconciled SelfHostedShootExposure"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(selfHostedShootExposure, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, selfHostedShootExposure, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
