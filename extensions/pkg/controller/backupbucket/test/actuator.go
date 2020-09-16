// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/controller/test"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(mgr manager.Manager, ignoreOperationAnnotation bool) error {
	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator:                  &actuator{},
		ControllerOptions:         controller.Options{},
		Predicates:                backupbucket.DefaultPredicates(ignoreOperationAnnotation),
		Type:                      test.Type,
		IgnoreOperationAnnotation: ignoreOperationAnnotation,
	})
}

type actuator struct {
	client client.Client
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// Reconcile updates the time-out annotation on the `BackupBucket` with the value of the `time-in` annotation. This is
// to enable integration tests to ensure that the `Reconcile` function of the actuator was called.
func (a *actuator) Reconcile(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) error {
	if bb.Annotations[test.AnnotationKeyDesiredOperationState] == test.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", test.AnnotationKeyDesiredOperationState, test.AnnotationValueDesiredOperationStateError)
	}

	metav1.SetMetaDataAnnotation(&bb.ObjectMeta, test.AnnotationKeyTimeOut, bb.Annotations[test.AnnotationKeyTimeIn])
	return a.client.Update(ctx, bb)
}

// Delete updates some annotation on the namespace of the referenced secret. This is to enable integration tests to
// ensure that the `Delete` function of the actuator was called. The backupbucket controller is removing the finalizer
// from the `BackupBucket` resource right after the `Delete` function returns nil, hence, we can't put the annotation
// directly to the `BackupBucket` resource because tests wouldn't be able to read it (the object would have already been
// deleted).
func (a *actuator) Delete(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) error {
	if bb.Annotations[test.AnnotationKeyDesiredOperationState] == test.AnnotationValueDesiredOperationStateError {
		return fmt.Errorf("error as requested by %s=%s annotation", test.AnnotationKeyDesiredOperationState, test.AnnotationValueDesiredOperationStateError)
	}

	namespace := &corev1.Namespace{}
	if err := a.client.Get(ctx, kutil.Key(bb.Spec.SecretRef.Namespace), namespace); err != nil {
		return err
	}

	metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, test.AnnotationKeyDesiredOperation, test.AnnotationValueOperationDelete)
	return a.client.Update(ctx, namespace)
}
