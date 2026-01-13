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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	extensionsintegrationtest "github.com/gardener/gardener/test/integration/extensions/controller"
)

func IgnoreOperationAnnotationOption(ignoreOperationAnnotation bool) func(args *backupentry.AddArgs) {
	return func(args *backupentry.AddArgs) {
		args.Predicates = backupentry.DefaultPredicates(ignoreOperationAnnotation)
		args.IgnoreOperationAnnotation = ignoreOperationAnnotation
	}
}

func WithActuator(a *actuator) func(args *backupentry.AddArgs) {
	return func(args *backupentry.AddArgs) {
		args.Actuator = a
	}
}

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

type actuator struct {
	client                                client.Client
	modifyBackupEntrySecretDuringDeletion bool
}

// Reconcile reconciles the BackupEntry.
func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeOut, be.Annotations[extensionsintegrationtest.AnnotationKeyTimeIn])
	return a.client.Update(ctx, be)
}

// Delete deletes the BackupEntry.
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	if a.modifyBackupEntrySecretDuringDeletion {
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

// Restore restores the BackupEntry.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyRestored, "true")
	return a.Reconcile(ctx, log, be)
}

// Migrate migrates the BackupEntry.
func (a *actuator) Migrate(ctx context.Context, _ logr.Logger, be *extensionsv1alpha1.BackupEntry) error {
	if be.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyMigrated, "true")
	metav1.SetMetaDataAnnotation(&be.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeOut, be.Annotations[extensionsintegrationtest.AnnotationKeyTimeIn])
	return a.client.Update(ctx, be)
}
