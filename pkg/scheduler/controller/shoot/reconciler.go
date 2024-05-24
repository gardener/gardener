// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
)

// Reconciler schedules shoots to seeds.
type Reconciler struct {
	Client          client.Client
	Config          *config.ShootSchedulerConfiguration
	GardenNamespace string
	Recorder        record.EventRecorder
}

// Reconcile schedules shoots to seeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if shoot.Spec.SeedName != nil {
		log.Info("Shoot already scheduled onto seed, nothing left to do", "seed", *shoot.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	if shoot.DeletionTimestamp != nil {
		log.Info("Ignoring shoot because it has been marked for deletion")
		return reconcile.Result{}, nil
	}

	// If no Seed is referenced, we try to determine an adequate one.
	seed, err := (&SeedDeterminer{Client: r.Client, GardenNamespace: r.GardenNamespace, Strategy: r.Config.Strategy}).Determine(ctx, log, shoot)
	if err != nil {
		r.reportFailedScheduling(ctx, log, shoot, err)
		return reconcile.Result{}, fmt.Errorf("failed to determine seed for shoot: %w", err)
	}

	shoot.Spec.SeedName = &seed.Name
	if err = r.Client.SubResource("binding").Update(ctx, shoot); err != nil {
		r.reportFailedScheduling(ctx, log, shoot, err)
		return reconcile.Result{}, fmt.Errorf("failed to bind shoot to seed: %w", err)
	}

	log.Info(
		"Shoot successfully scheduled to seed",
		"cloudprofile", shoot.Spec.CloudProfileName,
		"region", shoot.Spec.Region,
		"seed", seed.Name,
		"strategy", r.Config.Strategy,
	)

	r.reportEvent(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventSchedulingSuccessful, "Scheduled to seed '%s'", seed.Name)
	return reconcile.Result{}, nil
}

func (r *Reconciler) reportFailedScheduling(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot, err error) {
	description := "Failed to schedule Shoot: " + err.Error()
	r.reportEvent(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootEventSchedulingFailed, description)

	patch := client.MergeFrom(shoot.DeepCopy())
	if shoot.Status.LastOperation == nil {
		shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
	shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStatePending
	shoot.Status.LastOperation.LastUpdateTime = metav1.Now()
	shoot.Status.LastOperation.Description = description
	if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
		log.Error(err, "Failed to report scheduling failure to shoot status")
	}
}

func (r *Reconciler) reportEvent(shoot *gardencorev1beta1.Shoot, eventType string, eventReason, messageFmt string, args ...any) {
	r.Recorder.Eventf(shoot, eventType, eventReason, messageFmt, args...)
}
