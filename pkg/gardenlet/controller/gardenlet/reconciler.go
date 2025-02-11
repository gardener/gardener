// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// Reconciler reconciles the Gardenlet.
type Reconciler struct {
	GardenClient     client.Client
	GardenRESTConfig *rest.Config
	SeedClientSet    kubernetes.Interface
	Config           gardenletconfigv1alpha1.GardenletConfiguration
	Recorder         record.EventRecorder
	Clock            clock.Clock
	GardenNamespace  string
	HelmRegistry     oci.Interface
	ValuesHelper     gardenletdeployer.ValuesHelper
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.Gardenlet.SyncPeriod.Duration)
	defer cancel()

	gardenlet := &seedmanagementv1alpha1.Gardenlet{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, gardenlet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.V(1).Info("Reconciling")
	status := gardenlet.Status.DeepCopy()
	status.ObservedGeneration = gardenlet.Generation

	_, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(gardenlet.Name, &gardenlet.Spec.Config)
	if err != nil {
		r.Recorder.Eventf(gardenlet, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		updateCondition(r.Clock, status, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
			log.Error(updateErr, "Could not update status")
		}
		return reconcile.Result{}, fmt.Errorf("error extracting gardenlet configuration: %w", err)
	}

	seed, err := gardenletdeployer.GetSeed(ctx, r.GardenClient, gardenlet.Name)
	if err != nil {
		r.Recorder.Eventf(gardenlet, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		updateCondition(r.Clock, status, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
			log.Error(updateErr, "Could not update status")
		}
		return reconcile.Result{}, fmt.Errorf("error getting seed: %w", err)
	}

	log.Info("Deploying gardenlet")
	r.Recorder.Eventf(gardenlet, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet")
	if err := r.deployGardenlet(ctx, log, gardenlet, seed, gardenletConfig); err != nil {
		r.Recorder.Eventf(gardenlet, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		updateCondition(r.Clock, status, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
			log.Error(updateErr, "Could not update status")
		}
		return reconcile.Result{}, fmt.Errorf("error deploying gardenlet: %w", err)
	}

	log.V(1).Info("Reconciliation finished")
	r.Recorder.Eventf(gardenlet, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, "Gardenlet has been deployed")
	updateCondition(r.Clock, status, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled, "Gardenlet with chart from "+gardenlet.Spec.Deployment.Helm.OCIRepository.GetURL()+" has been deployed")
	if updateErr := r.updateStatus(ctx, gardenlet, status); updateErr != nil {
		log.Error(updateErr, "Could not update status")
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Gardenlet.SyncPeriod.Duration}, nil
}

func (r *Reconciler) deployGardenlet(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration,
) error {
	values, err := r.prepareGardenletChartValues(ctx, log, gardenlet, seed, gardenletConfig)
	if err != nil {
		return fmt.Errorf("failed preparing gardenlet chart values: %w", err)
	}

	subCtx := context.WithValue(ctx, oci.ContextKeyPullSecretNamespace, gardenerutils.ComputeGardenNamespace(seed.Name))
	archive, err := r.HelmRegistry.Pull(subCtx, &gardenlet.Spec.Deployment.Helm.OCIRepository)
	if err != nil {
		return fmt.Errorf("failed pulling Helm chart from OCI repository: %w", err)
	}

	if err := r.SeedClientSet.ChartApplier().ApplyFromArchive(ctx, archive, r.GardenNamespace, "gardenlet", kubernetes.Values(values)); err != nil {
		return fmt.Errorf("failed applying gardenlet chart from archive: %w", err)
	}

	// remove renew-kubeconfig annotation, if it exists
	if gardenlet.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		patch := client.MergeFrom(gardenlet.DeepCopy())
		delete(gardenlet.Annotations, v1beta1constants.GardenerOperation)
		if err := r.GardenClient.Patch(ctx, gardenlet, patch); err != nil {
			return fmt.Errorf("failed patching gardenlet annotations: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) prepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	gardenlet *seedmanagementv1alpha1.Gardenlet,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration,
) (
	map[string]interface{},
	error,
) {
	values, err := gardenletdeployer.PrepareGardenletChartValues(
		ctx,
		log,
		r.GardenClient,
		r.GardenRESTConfig,
		r.SeedClientSet.Client(),
		r.Recorder,
		gardenlet,
		seed,
		r.ValuesHelper,
		seedmanagementv1alpha1.BootstrapToken,
		&gardenlet.Spec.Deployment.GardenletDeployment,
		gardenletConfig,
		r.GardenNamespace,
	)
	if err != nil {
		return nil, fmt.Errorf("failed preparing chart values: %w", err)
	}

	if imageVector := gardenlet.Spec.Deployment.ImageVectorOverwrite; imageVector != nil {
		values["imageVectorOverwrite"] = *imageVector
	}

	if imageVector := gardenlet.Spec.Deployment.ComponentImageVectorOverwrite; imageVector != nil {
		values["componentImageVectorOverwrites"] = *imageVector
	}

	return values, nil
}

func updateCondition(clock clock.Clock, status *seedmanagementv1alpha1.GardenletStatus, conditionStatus gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, seedmanagementv1alpha1.GardenletReconciled)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, conditionStatus, reason, message)
	status.Conditions = v1beta1helper.MergeConditions(status.Conditions, condition)
}

func (r *Reconciler) updateStatus(ctx context.Context, gardenlet *seedmanagementv1alpha1.Gardenlet, status *seedmanagementv1alpha1.GardenletStatus) error {
	patch := client.StrategicMergeFrom(gardenlet.DeepCopy())
	gardenlet.Status = *status
	return r.GardenClient.Status().Patch(ctx, gardenlet, patch)
}
