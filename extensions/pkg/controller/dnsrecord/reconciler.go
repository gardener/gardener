// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdaterCustom
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// dnsrecord resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.DNSRecord{} },
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

	dns := &extensionsv1alpha1.DNSRecord{}
	if err := r.client.Get(ctx, request.NamespacedName, dns); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var cluster *extensions.Cluster
	if gardenerutils.IsShootNamespace(dns.Namespace) {
		var err error
		cluster, err = extensionscontroller.GetCluster(ctx, r.client, dns.Namespace)
		if err != nil {
			return reconcile.Result{}, err
		}

		if extensionscontroller.IsFailed(cluster) {
			log.Info("Skipping the reconciliation of DNSRecord of failed shoot")
			return reconcile.Result{}, nil
		}
	}

	operationType := v1beta1helper.ComputeOperationType(dns.ObjectMeta, dns.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, dns):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, dns, cluster)
	case dns.DeletionTimestamp != nil:
		return r.delete(ctx, log, dns, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, dns, cluster)
	default:
		return r.reconcile(ctx, log, dns, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	dns *extensionsv1alpha1.DNSRecord,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(dns, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, dns, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.ProcessingCustom(ctx, log, dns, operationType, "Reconciling the DNSRecord", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of DNSRecord")
	if err := r.actuator.Reconcile(ctx, log, dns, cluster); err != nil {
		_ = r.statusUpdater.ErrorCustom(ctx, log, dns, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling DNSRecord", addCreatedConditionFalse)
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, dns, operationType, "Successfully reconciled DNSRecord", addCreatedConditionTrue); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	dns *extensionsv1alpha1.DNSRecord,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(dns, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, dns, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.ProcessingCustom(ctx, log, dns, gardencorev1beta1.LastOperationTypeRestore, "Restoring the DNSRecord", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the restoration of DNSRecord")
	if err := r.actuator.Restore(ctx, log, dns, cluster); err != nil {
		_ = r.statusUpdater.ErrorCustom(ctx, log, dns, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring DNSRecord", addCreatedConditionFalse)
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, dns, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored DNSRecord", addCreatedConditionTrue); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, dns, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from DNSRecord: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	dns *extensionsv1alpha1.DNSRecord,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if err := r.statusUpdater.ProcessingCustom(ctx, log, dns, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the DNSRecord", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the migration of DNSRecord")
	if err := r.actuator.Migrate(ctx, log, dns, cluster); err != nil {
		_ = r.statusUpdater.ErrorCustom(ctx, log, dns, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating DNSRecord", nil)
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, dns, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated DNSRecord", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, dns); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, dns, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from DNSRecord: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	dns *extensionsv1alpha1.DNSRecord,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(dns, FinalizerName) {
		log.Info("Deleting DNSRecord causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	switch getCreatedConditionStatus(dns.GetExtensionStatus()) {
	case gardencorev1beta1.ConditionTrue, gardencorev1beta1.ConditionUnknown:
		operationType := v1beta1helper.ComputeOperationType(dns.ObjectMeta, dns.Status.LastOperation)
		if err := r.statusUpdater.ProcessingCustom(ctx, log, dns, operationType, "Deleting the DNSRecord", nil); err != nil {
			return reconcile.Result{}, err
		}

		log.Info("Starting the deletion of DNSRecord")
		var err error
		if cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot) {
			err = r.actuator.ForceDelete(ctx, log, dns, cluster)
		} else {
			err = r.actuator.Delete(ctx, log, dns, cluster)
		}
		if err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, dns, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting DNSRecord", nil)
			return reconcilerutils.ReconcileErr(err)
		}

		if err := r.statusUpdater.SuccessCustom(ctx, log, dns, operationType, "Successfully deleted DNSRecord", nil); err != nil {
			return reconcile.Result{}, err
		}
	case gardencorev1beta1.ConditionFalse:
		log.Info("Deleting DNSRecord is no-op as not created")
	}

	if controllerutil.ContainsFinalizer(dns, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, dns, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func updateCreatedCondition(status extensionsv1alpha1.Status, conditionStatus gardencorev1beta1.ConditionStatus, reason, message string, updateIfExisting bool) error {
	conditions := status.GetConditions()
	c := v1beta1helper.GetCondition(conditions, extensionsv1alpha1.ConditionTypeCreated)
	if c != nil && !updateIfExisting {
		return nil
	}
	if c != nil && c.Status == conditionStatus {
		return nil
	}

	builder, err := v1beta1helper.NewConditionBuilder(extensionsv1alpha1.ConditionTypeCreated)
	if err != nil {
		return err
	}
	if c != nil {
		builder = builder.WithOldCondition(*c)
	}

	newCondition, _ := builder.WithStatus(conditionStatus).WithReason(reason).WithMessage(message).Build()
	status.SetConditions(v1beta1helper.MergeConditions(conditions, newCondition))
	return nil
}

func getCreatedConditionStatus(status extensionsv1alpha1.Status) gardencorev1beta1.ConditionStatus {
	for _, c := range status.GetConditions() {
		if c.Type == extensionsv1alpha1.ConditionTypeCreated {
			return c.Status
		}
	}
	return gardencorev1beta1.ConditionUnknown
}

func addCreatedConditionFalse(status extensionsv1alpha1.Status) error {
	message := "Error on initial record creation in infrastructure"
	return updateCreatedCondition(status, gardencorev1beta1.ConditionFalse, "Error", message, false)
}

func addCreatedConditionTrue(status extensionsv1alpha1.Status) error {
	message := "Record was created successfully in infrastructure at least once"
	return updateCreatedCondition(status, gardencorev1beta1.ConditionTrue, "Success", message, true)
}
