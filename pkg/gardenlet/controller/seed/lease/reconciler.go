// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/healthz"
)

// Reconciler reconciles Seed resources and updates the heartbeat Lease object in the garden cluster when the connection
// to the seed cluster succeeds.
type Reconciler struct {
	GardenClient   client.Client
	SeedRESTClient rest.Interface
	Config         gardenletconfigv1alpha1.SeedControllerConfiguration
	Clock          clock.Clock
	HealthManager  healthz.Manager
	LeaseNamespace string
	SeedName       string
}

// Reconcile reconciles Seed resources and updates the heartbeat Lease object in the garden cluster when the connection
// to the seed cluster succeeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, time.Duration(*r.Config.LeaseResyncSeconds)*time.Second)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := CheckSeedConnection(ctx, r.SeedRESTClient); err != nil {
		r.HealthManager.Set(false)
		return reconcile.Result{}, fmt.Errorf("cannot establish connection with Seed: %w", err)
	}

	if err := r.renewLeaseForSeed(ctx, seed); err != nil {
		r.HealthManager.Set(false)
		return reconcile.Result{}, err
	}

	r.HealthManager.Set(true)
	return reconcile.Result{RequeueAfter: time.Duration(*r.Config.LeaseResyncSeconds) * time.Second}, r.maintainGardenletReadyCondition(ctx, seed)
}

// CheckSeedConnection is a function which checks the connection to the seed.
// Exposed for testing.
var CheckSeedConnection = func(ctx context.Context, client rest.Interface) error {
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

func (r *Reconciler) renewLeaseForSeed(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      seed.Name,
			Namespace: r.LeaseNamespace,
		},
	}

	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, r.GardenClient, lease, func() error {
		lease.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Seed",
			Name:       seed.GetName(),
			UID:        seed.GetUID(),
		}}
		lease.Spec.HolderIdentity = ptr.To(seed.Name)
		lease.Spec.LeaseDurationSeconds = r.Config.LeaseResyncSeconds
		lease.Spec.RenewTime = &metav1.MicroTime{Time: r.Clock.Now()}
		return nil
	})
	return err
}

func (r *Reconciler) maintainGardenletReadyCondition(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	bldr, err := helper.NewConditionBuilder(gardencorev1beta1.SeedGardenletReady)
	if err != nil {
		return err
	}

	if oldCondition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady); oldCondition != nil {
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

	patch := client.StrategicMergeFrom(seed.DeepCopy())
	seed.Status.Conditions = helper.MergeConditions(seed.Status.Conditions, newCondition)
	return r.GardenClient.Status().Patch(ctx, seed, patch)
}
