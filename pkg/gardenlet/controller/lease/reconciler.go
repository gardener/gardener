// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"context"
	"fmt"
	"net/http"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/healthz"
)

// Reconciler reconciles resources and updates a corresponding heartbeat Lease object in the garden cluster when the
// connection to the runtime cluster succeeds.
type Reconciler struct {
	GardenClient      client.Client
	RuntimeRESTClient rest.Interface

	NewObjectFunc       func() client.Object
	GetObjectConditions func(client.Object) []gardencorev1beta1.Condition
	SetObjectConditions func(client.Object, []gardencorev1beta1.Condition)

	LeaseResyncSeconds int32
	LeaseNamePrefix    string
	LeaseNamespace     *string
	Clock              clock.Clock
	HealthManager      healthz.Manager
}

// Reconcile reconciles resources and updates a corresponding heartbeat Lease object in the garden cluster when the
// // connection to the runtime cluster succeeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := r.NewObjectFunc()
	if err := r.GardenClient.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := CheckConnection(ctx, r.RuntimeRESTClient); err != nil {
		r.HealthManager.Set(false)
		return reconcile.Result{}, fmt.Errorf("cannot establish connection with runtime cluster: %w", err)
	}

	if err := r.renewLease(ctx, obj); err != nil {
		r.HealthManager.Set(false)
		return reconcile.Result{}, err
	}

	r.HealthManager.Set(true)
	return reconcile.Result{RequeueAfter: time.Duration(r.LeaseResyncSeconds) * time.Second}, r.maintainGardenletReadyCondition(ctx, obj)
}

// CheckConnection is a function which checks the connection to the runtime cluster.
// Exposed for testing.
var CheckConnection = func(ctx context.Context, client rest.Interface) error {
	result := client.Get().AbsPath("/healthz").Do(ctx)
	if result.Error() != nil {
		return fmt.Errorf("failed to execute call to Kubernetes API Server: %v", result.Error())
	}

	var statusCode int
	result.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		return fmt.Errorf("API Server returned unexpected status code: %d", statusCode)
	}

	return nil
}

func (r *Reconciler) renewLease(ctx context.Context, obj client.Object) error {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.LeaseNamePrefix + obj.GetName(),
			Namespace: ptr.Deref(r.LeaseNamespace, obj.GetNamespace()),
		},
	}

	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, r.GardenClient, lease, func() error {
		lease.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
			Name:       obj.GetName(),
			UID:        obj.GetUID(),
		}}
		lease.Spec.HolderIdentity = ptr.To(obj.GetName())
		lease.Spec.LeaseDurationSeconds = &r.LeaseResyncSeconds
		lease.Spec.RenewTime = &metav1.MicroTime{Time: r.Clock.Now()}
		return nil
	})
	return err
}

func (r *Reconciler) maintainGardenletReadyCondition(ctx context.Context, obj client.Object) error {
	bldr, err := helper.NewConditionBuilder(gardencorev1beta1.GardenletReady)
	if err != nil {
		return err
	}

	if oldCondition := helper.GetCondition(r.GetObjectConditions(obj), gardencorev1beta1.GardenletReady); oldCondition != nil {
		bldr.WithOldCondition(*oldCondition)
	}
	bldr.WithStatus(gardencorev1beta1.ConditionTrue)
	bldr.WithReason("GardenletReady")
	bldr.WithMessage("Gardenlet is posting ready status.")
	bldr.WithClock(r.Clock)

	newCondition, needsUpdate := bldr.Build()
	if !needsUpdate {
		return nil
	}

	patch := client.StrategicMergeFrom(obj.DeepCopyObject().(client.Object))
	r.SetObjectConditions(obj, helper.MergeConditions(r.GetObjectConditions(obj), newCondition))
	return r.GardenClient.Status().Patch(ctx, obj, patch)
}
