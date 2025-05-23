// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsintegrationtest "github.com/gardener/gardener/test/integration/extensions/controller"
)

// addTestControllerToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func addTestControllerToManagerWithOptions(mgr manager.Manager, ignoreOperationAnnotation bool) error {
	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator: &actuator{client: mgr.GetClient()},
		ControllerOptions: controller.Options{
			// Use custom rate limiter to slow down re-enqueuing in case of errors.
			// Some tests rely on reading an error state which is removed too quickly by subsequent reconciliations.
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](50*time.Millisecond, 1000*time.Second),
		},
		Predicates:                backupbucket.DefaultPredicates(ignoreOperationAnnotation),
		Type:                      extensionsintegrationtest.Type,
		IgnoreOperationAnnotation: ignoreOperationAnnotation,
	})
}

type actuator struct {
	client client.Client
}

// Reconcile updates the time-out annotation on the `BackupBucket` with the value of the `time-in` annotation. This is
// to enable integration tests to ensure that the `Reconcile` function of the actuator was called.
func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	if bb.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&bb.ObjectMeta, extensionsintegrationtest.AnnotationKeyTimeOut, bb.Annotations[extensionsintegrationtest.AnnotationKeyTimeIn])
	return a.client.Update(ctx, bb)
}

// Delete updates some annotation on the namespace of the referenced secret. This is to enable integration tests to
// ensure that the `Delete` function of the actuator was called. The backupbucket controller is removing the finalizer
// from the `BackupBucket` resource right after the `Delete` function returns nil, hence, we can't put the annotation
// directly to the `BackupBucket` resource because tests wouldn't be able to read it (the object would have already been
// deleted).
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	if bb.Annotations[extensionsintegrationtest.AnnotationKeyDesiredOperationState] == extensionsintegrationtest.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", extensionsintegrationtest.AnnotationKeyDesiredOperationState, extensionsintegrationtest.AnnotationValueDesiredOperationStateError)
	}

	namespace := &corev1.Namespace{}
	if err := a.client.Get(ctx, client.ObjectKey{Name: bb.Spec.SecretRef.Namespace}, namespace); err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, extensionsintegrationtest.AnnotationKeyDesiredOperation, extensionsintegrationtest.AnnotationValueOperationDelete)
	return a.client.Update(ctx, namespace)
}
