// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the ManagedSeed.
type Reconciler struct {
	GardenConfig          *rest.Config
	GardenAPIReader       client.Reader
	GardenClient          client.Client
	SeedClient            client.Client
	Config                gardenletconfigv1alpha1.GardenletConfiguration
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	ShootClientMap        clientmap.ClientMap
	GardenNamespaceGarden string
	GardenNamespaceShoot  string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.ManagedSeed.SyncPeriod.Duration)
	defer cancel()

	ms := &seedmanagementv1alpha1.ManagedSeed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, ms); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: ms.Spec.Shoot.Name, Namespace: ms.Namespace}}
	if err := r.GardenAPIReader.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	log = log.WithValues("shootName", shoot.Name)
	actuator := r.newActuator(shoot)

	if ms.DeletionTimestamp != nil {
		return r.delete(ctx, log, ms, actuator)
	}
	return r.reconcile(ctx, log, ms, actuator, shoot)
}

// Actuator is exposed for testing.
var Actuator gardenletdeployer.Interface

func (r *Reconciler) newActuator(shoot *gardencorev1beta1.Shoot) gardenletdeployer.Interface {
	if Actuator != nil {
		return Actuator
	}

	return &gardenletdeployer.Actuator{
		GardenConfig: r.GardenConfig,
		GardenClient: r.GardenClient,
		GetTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
			return r.ShootClientMap.GetClient(ctx, keys.ForShoot(shoot))
		},
		CheckIfVPAAlreadyExists: func(ctx context.Context) (bool, error) {
			if err := r.SeedClient.Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "vpa-admission-controller"}, &appsv1.Deployment{}); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return true, nil
		},
		GetInfrastructureSecret: func(ctx context.Context) (*corev1.Secret, error) {
			if shoot.Spec.SecretBindingName == nil && shoot.Spec.CredentialsBindingName == nil {
				return nil, fmt.Errorf("both secretBindingName and credentialsBindingName are nil for the Shoot: %s/%s", shoot.Namespace, shoot.Name)
			}

			if shoot.Spec.SecretBindingName != nil {
				shootSecretBinding := &gardencorev1beta1.SecretBinding{}
				if err := r.GardenClient.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.SecretBindingName}, shootSecretBinding); err != nil {
					return nil, err
				}
				return kubernetesutils.GetSecretByReference(ctx, r.GardenClient, &shootSecretBinding.SecretRef)
			}

			shootCredentialsBinding := &securityv1alpha1.CredentialsBinding{}
			if err := r.GardenClient.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.CredentialsBindingName}, shootCredentialsBinding); err != nil {
				return nil, err
			}

			// TODO(dimityrmirchev): only handle credentials of kind secret as this function will be eventually removed
			if shootCredentialsBinding.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() &&
				shootCredentialsBinding.CredentialsRef.Kind == "Secret" {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootCredentialsBinding.CredentialsRef.Name,
						Namespace: shootCredentialsBinding.CredentialsRef.Namespace,
					},
				}

				return secret, r.GardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
			}

			return nil, nil
		},
		GetTargetDomain: func() string {
			if shoot.Spec.DNS == nil {
				return ""
			}
			return ptr.Deref(shoot.Spec.DNS.Domain, "")
		},
		ApplyGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error {
			return targetChartApplier.ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, r.GardenNamespaceShoot, "gardenlet", kubernetes.Values(values))
		},
		DeleteGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error {
			return targetChartApplier.DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, r.GardenNamespaceShoot, "gardenlet", kubernetes.Values(values))
		},
		Clock:                 r.Clock,
		ValuesHelper:          gardenletdeployer.NewValuesHelper(&r.Config),
		Recorder:              r.Recorder,
		GardenNamespaceTarget: r.GardenNamespaceShoot,
	}
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
	actuator gardenletdeployer.Interface,
	shoot *gardencorev1beta1.Shoot,
) (
	result reconcile.Result,
	err error,
) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// Initialize status
	status := ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	// Check if shoot is reconciled and update ShootReconciled condition
	if !shootReconciled(shoot) {
		log.Info("Waiting for shoot to be reconciled")

		msg := fmt.Sprintf("Waiting for shoot %q to be reconciled", client.ObjectKeyFromObject(shoot).String())
		r.Recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, msg)
		updateCondition(r.Clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconciling, msg)

		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, r.updateStatus(ctx, ms, status)
	}
	updateCondition(r.Clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Shoot %q has been reconciled", client.ObjectKeyFromObject(shoot).String()))

	// Reconcile creation or update
	log.V(1).Info("Reconciling")
	status.Conditions, err = actuator.Reconcile(ctx, log, ms, status.Conditions, ms.Spec.Gardenlet.Deployment, &ms.Spec.Gardenlet.Config, helper.GetBootstrap(ms.Spec.Gardenlet.Bootstrap), ptr.Deref(ms.Spec.Gardenlet.MergeWithParent, false))
	if err != nil {
		if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil {
			log.Error(updateErr, "Could not update status", "status", status)
		}
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s creation or update: %w", client.ObjectKeyFromObject(ms), err)
	}

	log.V(1).Info("Reconciliation finished")
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, r.updateStatus(ctx, ms, status)
}

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
	actuator gardenletdeployer.Interface,
) (
	result reconcile.Result,
	err error,
) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping deletion as object does not have a finalizer")
		return reconcile.Result{}, nil
	}

	// Initialize status
	status := ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	// Reconcile deletion
	log.V(1).Info("Deletion")
	var wait, removeFinalizer bool
	status.Conditions, wait, removeFinalizer, err = actuator.Delete(ctx, log, ms, ms.Status.Conditions, ms.Spec.Gardenlet.Deployment, &ms.Spec.Gardenlet.Config, helper.GetBootstrap(ms.Spec.Gardenlet.Bootstrap), ptr.Deref(ms.Spec.Gardenlet.MergeWithParent, false))
	if err != nil {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil {
				log.Error(updateErr, "Could not update status", "status", status)
			}
		}
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s deletion: %w", client.ObjectKeyFromObject(ms), err)
	}

	log.V(1).Info("Deletion finished")
	// Only update status if the finalizer is not removed to prevent errors if the object is already gone
	if !removeFinalizer {
		if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil {
			log.Error(updateErr, "Could not update status", "status", status)
		}
	}

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func shootReconciled(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && lastOp != nil && lastOp.State == gardencorev1beta1.LastOperationStateSucceeded
}

func updateCondition(clock clock.Clock, status *seedmanagementv1alpha1.ManagedSeedStatus, ct gardencorev1beta1.ConditionType, cs gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, ct)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	status.Conditions = v1beta1helper.MergeConditions(status.Conditions, condition)
}

func (r *Reconciler) updateStatus(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed, status *seedmanagementv1alpha1.ManagedSeedStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(ms.DeepCopy())
	ms.Status = *status
	return r.GardenClient.Status().Patch(ctx, ms, patch)
}
