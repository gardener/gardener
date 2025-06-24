// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// Reconciler is a reconciler that patches every ManagedResource object to a healthy status.
// This is useful for integration testing against an envtest control plane, which doesn't run the gardener-resource-manager.
type Reconciler struct {
	Client client.Client
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.ControllerManagedBy(mgr).
		Named("managedresource").
		For(&resourcesv1alpha1.ManagedResource{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			RecoverPanic:            ptr.To(true),
		}).
		Complete(r)
}

// Reconcile makes sure that every ManagedResource object has a healthy status.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.Client.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	patch := client.MergeFrom(mr.DeepCopy())
	mr.Status.ObservedGeneration = mr.Generation
	mr.Status.Conditions = []gardencorev1beta1.Condition{
		{
			Type:               "ResourcesHealthy",
			Status:             "True",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
		{
			Type:               "ResourcesApplied",
			Status:             "True",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
		{
			Type:               "ResourcesProgressing",
			Status:             "False",
			LastUpdateTime:     metav1.NewTime(time.Unix(0, 0)),
			LastTransitionTime: metav1.NewTime(time.Unix(0, 0)),
		},
	}

	if err := r.Client.Status().Patch(ctx, mr, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to patch status of managed resource %s: %w", client.ObjectKeyFromObject(mr), err)
	}

	return reconcile.Result{}, nil
}
