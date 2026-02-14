// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	extensionsintegrationtest "github.com/gardener/gardener/test/integration/extensions/controller"
)

// annotationValueModifySecretDuringDeletion is a constant for a value of an annotation on the BackupEntry
// describing that the referenced secret should be modified during deletion.
const annotationValueModifySecretDuringDeletion = "ModifySecretDuringDeletion"

// addTestControllerToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func addTestControllerToManagerWithOptions(mgr manager.Manager, opts ...func(args *backupentry.AddArgs)) error {
	addArgs := backupentry.AddArgs{
		Actuator:          &actuator{client: mgr.GetClient()},
		ControllerOptions: controller.Options{
			// Use custom rate limiter to slow down re-enqueuing in case of errors.
			// Some tests rely on reading an error state which is removed too quickly by subsequent reconciliations.
			// RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](50*time.Millisecond, 1000*time.Second),
		},
		Type:                      extensionsintegrationtest.Type,
		Predicates:                backupentry.DefaultPredicates(false),
		IgnoreOperationAnnotation: false,
	}

	for _, opt := range opts {
		opt(&addArgs)
	}

	return backupentry.Add(mgr, addArgs)
}

func ignoreOperationAnnotationOption(ignoreOperationAnnotation bool) func(args *backupentry.AddArgs) {
	return func(args *backupentry.AddArgs) {
		args.Predicates = backupentry.DefaultPredicates(ignoreOperationAnnotation)
		args.IgnoreOperationAnnotation = ignoreOperationAnnotation
	}
}

type actuator struct {
	client client.Client
}

// Reconcile updates the time-out annotation on the `BackupEntry` with the value of the `time-in` annotation. This is
// to enable integration tests to ensure that the `Reconcile` function of the actuator was called.
func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeOut, be.Annotations[extensionsintegrationtest.AnnotationKeyTimeIn])
	return a.client.Update(ctx, be)
}

// Delete updates some annotation on the namespace of the referenced secret. This is to enable integration tests to
// ensure that the `Delete` function of the actuator was called. The backupentry controller is removing the finalizer
// from the `BackupEntry` resource right after the `Delete` function returns nil, hence, we can't put the annotation
// directly to the `BackupEntry` resource because tests wouldn't be able to read it (the object would have already been
// deleted).
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperation] == annotationValueModifySecretDuringDeletion {
		// When this annotation is used, the referenced secret is modified to simulate a modification done by an external
		// controller. In a real case, the `core.BackupEntry` reconciler can modify the referenced secret during deletion
		// while the `extensions.BackupEntry` is being deleted by this actuator.
		// See https://github.com/gardener/gardener/issues/12612 for more information.
		secretMetadata, err := kubernetesutils.GetSecretMetadataByReference(ctx, a.client, &be.Spec.SecretRef)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(secretMetadata.DeepCopy())
		metav1.SetMetaDataAnnotation(&secretMetadata.ObjectMeta, "time", time.Now().String())
		if err := a.client.Patch(ctx, secretMetadata, patch); err != nil {
			return err
		}
	}

	namespace := &corev1.Namespace{}
	if err := a.client.Get(ctx, client.ObjectKey{Name: be.Spec.SecretRef.Namespace}, namespace); err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperation, extensionsintegrationtest.AnnotationValueOperationDelete)
	return a.client.Update(ctx, namespace)
}

// Restore adds the `desired-operation: restore` annotation to the `BackupEntry` so that integration tests
// can check that the `Restore` function of the actuator was called. It then calls the `Reconcile` function.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationRestore)
	return a.Reconcile(ctx, log, be)
}

// Migrate adds the `desired-operation: migrate` annotation to the `BackupEntry` so that integration tests
// can check that the `Migrate` function of the actuator was called. It also updates the time-out annotation
// on the `BackupEntry` with the value of the `time-in` annotation.
func (a *actuator) Migrate(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperation, v1beta1constants.GardenerOperationMigrate)
	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeOut, be.Annotations[extensionsintegrationtest.AnnotationKeyTimeIn])
	return a.client.Update(ctx, be)
}
