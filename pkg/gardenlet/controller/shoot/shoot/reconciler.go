// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerextensions "github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

const taskID = "initializeOperation"

// Reconciler implements the main shoot reconciliation logic, i.e., creation, hibernation, migration and deletion.
type Reconciler struct {
	GardenClient          client.Client
	SeedClientSet         kubernetes.Interface
	ShootClientMap        clientmap.ClientMap
	Config                config.GardenletConfiguration
	Recorder              record.EventRecorder
	ImageVector           imagevector.ImageVector
	Identity              *gardencorev1beta1.Gardener
	GardenClusterIdentity string
	Clock                 clock.Clock
}

// Reconcile implements the main shoot reconciliation logic, i.e., creation, hibernation, migration and deletion.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if responsibleSeedName := gardenerutils.GetResponsibleSeedName(shoot.Spec.SeedName, shoot.Status.SeedName); responsibleSeedName != r.Config.SeedConfig.Name {
		log.Info("Skipping because Shoot is not managed by this gardenlet", "seedName", responsibleSeedName)
		return reconcile.Result{}, nil
	}

	if shoot.DeletionTimestamp != nil {
		return r.deleteShoot(ctx, log, shoot)
	}

	if helper.ShouldPrepareShootForMigration(shoot) {
		return r.migrateShoot(ctx, log, shoot)
	}

	return r.reconcileShoot(ctx, log, shoot)
}

func (r *Reconciler) reconcileShoot(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) (reconcile.Result, error) {
	var (
		operationType = helper.ComputeOperationType(shoot)
		isRestoring   = operationType == gardencorev1beta1.LastOperationTypeRestore
	)
	log = log.WithValues("operation", strings.ToLower(string(operationType)))

	if !controllerutil.ContainsFinalizer(shoot, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, shoot, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	o, result, err := r.prepareOperation(ctx, log, shoot)
	if err != nil || o == nil {
		return result, err
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, fmt.Sprintf("%s Shoot cluster", utils.IifString(isRestoring, "Restoring", "Reconciling")))
	if flowErr := r.runReconcileShootFlow(ctx, o, operationType); flowErr != nil {
		r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, flowErr.Description)
		updateErr := r.patchShootStatusOperationError(ctx, shoot, flowErr.Description, operationType, flowErr.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, fmt.Sprintf("%s Shoot cluster", utils.IifString(isRestoring, "Restored", "Reconciled")))
	if err := r.patchShootStatusOperationSuccess(ctx, shoot, o.Shoot.SeedNamespace, &o.Seed.GetInfo().Name, operationType); err != nil {
		return reconcile.Result{}, err
	}

	if syncErr := r.syncClusterResourceToSeed(ctx, shoot, o.Garden.Project, o.Shoot.CloudProfile, o.Seed.GetInfo()); syncErr != nil {
		log.Error(syncErr, "Cluster resource sync to seed failed")
		updateErr := r.patchShootStatusOperationError(ctx, shoot, syncErr.Error(), operationType, shoot.Status.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(syncErr, updateErr)
	}

	// determine when the next shoot reconciliation is supposed to happen
	result = helper.CalculateControllerInfos(shoot, r.Clock, *r.Config.Controllers.Shoot).RequeueAfter
	nextReconciliation := r.Clock.Now().UTC().Add(result.RequeueAfter)

	log.Info("Shoot operation finished successfully, scheduling next reconciliation for Shoot", "requeueAfter", result.RequeueAfter, "nextReconciliation", nextReconciliation)
	return result, nil
}

func (r *Reconciler) migrateShoot(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) (reconcile.Result, error) {
	log = log.WithValues("operation", "migrate")

	destinationSeed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Name: *shoot.Spec.SeedName}, destinationSeed); err != nil {
		return reconcile.Result{}, err
	}

	if err := helper.IsSeedReadyForMigration(destinationSeed, r.Identity); err != nil {
		return reconcile.Result{}, fmt.Errorf("destination Seed is not available to host the control plane of Shoot %s: %w", shoot.GetName(), err)
	}

	hasBastions, err := r.shootHasBastions(ctx, shoot)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check for related Bastions: %w", err)
	}
	if hasBastions {
		hasBastionErr := errors.New("shoot has still Bastions")
		updateErr := r.patchShootStatusOperationError(ctx, shoot, hasBastionErr.Error(), gardencorev1beta1.LastOperationTypeMigrate, shoot.Status.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(hasBastionErr, updateErr)
	}

	o, result, err := r.prepareOperation(ctx, log, shoot)
	if err != nil || o == nil {
		return result, err
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventPrepareMigration, "Preparing Shoot cluster for migration")
	if flowErr := r.runMigrateShootFlow(ctx, o); flowErr != nil {
		r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventMigrationPreparationFailed, flowErr.Description)
		updateErr := r.patchShootStatusOperationError(ctx, shoot, flowErr.Description, gardencorev1beta1.LastOperationTypeMigrate, flowErr.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	return r.finalizeShootMigration(ctx, shoot, o)
}

func (r *Reconciler) deleteShoot(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(shoot, gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	log = log.WithValues("operation", "delete")

	// If the .status.uid field is empty, then we assume that there has never been any operation running for this Shoot
	// cluster. This implies that there can not be any resource which we have to delete.
	// We accept the deletion.
	if len(shoot.Status.UID) == 0 {
		log.Info("The `.status.uid` is empty, assuming Shoot cluster did never exist, deletion accepted")
		return r.finalizeShootDeletion(ctx, log, shoot)
	}

	operationType := v1beta1helper.ComputeOperationType(shoot.ObjectMeta, shoot.Status.LastOperation)

	hasBastions, err := r.shootHasBastions(ctx, shoot)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check for related Bastions: %w", err)
	}
	if hasBastions {
		hasBastionErr := errors.New("shoot has still Bastions")
		updateErr := r.patchShootStatusOperationError(ctx, shoot, hasBastionErr.Error(), operationType, shoot.Status.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(hasBastionErr, updateErr)
	}

	// If the .status.lastOperation already indicates that the deletion is successful then we finalize it immediately.
	if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete && shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
		log.Info("The `.status.lastOperation` indicates a successful deletion, deletion accepted")
		return r.finalizeShootDeletion(ctx, log, shoot)
	}

	o, result, err := r.prepareOperation(ctx, log, shoot)
	if err != nil || o == nil {
		return result, err
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting Shoot cluster")
	if flowErr := r.runDeleteShootFlow(ctx, o); flowErr != nil {
		r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, flowErr.Description)
		updateErr := r.patchShootStatusOperationError(ctx, shoot, flowErr.Description, operationType, flowErr.LastErrors...)
		return reconcile.Result{}, errorsutils.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, "Deleted Shoot cluster")
	return r.finalizeShootDeletion(ctx, log, shoot)
}

func (r *Reconciler) prepareOperation(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) (*operation.Operation, reconcile.Result, error) {
	// fetch related objects required for shoot operation
	project, _, err := gardenerutils.ProjectAndNamespaceFromReader(ctx, r.GardenClient, shoot.Namespace)
	if err != nil {
		return nil, reconcile.Result{}, err
	}
	if project == nil {
		return nil, reconcile.Result{}, fmt.Errorf("cannot find Project for namespace '%s'", shoot.Namespace)
	}

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.GardenClient.Get(ctx, kubernetesutils.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return nil, reconcile.Result{}, err
	}

	seed := &gardencorev1beta1.Seed{}
	// always fetch the seed that this gardenlet is responsible for (instead of using spec.seedName),
	// it is never acting on a foreign seed (e.g., during control plane migration)
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Name: r.Config.SeedConfig.Name}, seed); err != nil {
		return nil, reconcile.Result{}, err
	}

	i := helper.CalculateControllerInfos(shoot, r.Clock, *r.Config.Controllers.Shoot)
	log.V(1).Info("Calculated infos", "infos", i)

	if !i.ShouldReconcileNow {
		log.Info("Skipping shoot because it should not be reconciled right now")

		if i.ShouldOnlySyncClusterResource {
			if syncErr := r.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); syncErr != nil {
				log.Error(syncErr, "Failed syncing Cluster resource to Seed while Shoot should not be reconciled")
				updateErr := r.patchShootStatusOperationError(ctx, shoot, syncErr.Error(), i.OperationType, shoot.Status.LastErrors...)
				return nil, reconcile.Result{}, errorsutils.WithSuppressed(syncErr, updateErr)
			}
			return nil, reconcile.Result{}, nil
		}

		return nil, i.RequeueAfter, nil
	}

	shootNamespace := shootpkg.ComputeTechnicalID(project.Name, shoot)
	if err := r.updateShootStatusOperationStart(ctx, shoot, shootNamespace, i.OperationType); err != nil {
		return nil, reconcile.Result{}, err
	}

	o, operationErr := r.initializeOperation(ctx, log, shoot, project, cloudProfile, seed)
	if operationErr != nil {
		updateErr := r.patchShootStatusOperationError(ctx, shoot, fmt.Sprintf("Could not initialize a new operation for Shoot cluster: %s", operationErr.Error()), i.OperationType, lastErrorsOperationInitializationFailure(shoot.Status.LastErrors, operationErr)...)
		return nil, reconcile.Result{}, errorsutils.WithSuppressed(operationErr, updateErr)
	}

	if err := r.checkSeedAndSyncClusterResource(ctx, shoot, project, cloudProfile, seed); err != nil {
		log.Error(err, "Shoot cannot be synced with seed")

		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.LastOperation.Description = fmt.Sprintf("Shoot cannot be synced with Seed: %v", err)
		shoot.Status.LastOperation.LastUpdateTime = metav1.NewTime(r.Clock.Now().UTC())
		if patchErr := r.GardenClient.Status().Patch(ctx, shoot, patch); patchErr != nil {
			return nil, reconcile.Result{}, errorsutils.WithSuppressed(err, patchErr)
		}

		return nil, reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return o, reconcile.Result{}, nil
}

func (r *Reconciler) initializeOperation(
	ctx context.Context,
	log logr.Logger,
	shoot *gardencorev1beta1.Shoot,
	project *gardencorev1beta1.Project,
	cloudProfile *gardencorev1beta1.CloudProfile,
	seed *gardencorev1beta1.Seed,
) (
	*operation.Operation,
	error,
) {
	gardenSecrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(seed.Name), true)
	if err != nil {
		return nil, err
	}

	gardenObj, err := garden.
		NewBuilder().
		WithProject(project).
		WithInternalDomainFromSecrets(gardenSecrets).
		WithDefaultDomainsFromSecrets(gardenSecrets).
		Build(ctx)
	if err != nil {
		return nil, err
	}

	seedObj, err := seedpkg.
		NewBuilder().
		WithSeedObject(seed).
		Build(ctx)
	if err != nil {
		return nil, err
	}

	shootObj, err := shootpkg.
		NewBuilder().
		WithShootObject(shoot).
		WithCloudProfileObject(cloudProfile).
		WithShootSecretFrom(r.GardenClient).
		WithSeedObject(seed).
		WithProjectName(project.Name).
		WithInternalDomain(gardenObj.InternalDomain).
		WithDefaultDomains(gardenObj.DefaultDomains).
		Build(ctx, r.GardenClient)
	if err != nil {
		return nil, err
	}

	return operation.
		NewBuilder().
		WithLogger(log).
		WithConfig(&r.Config).
		WithGardenerInfo(r.Identity).
		WithGardenClusterIdentity(r.GardenClusterIdentity).
		WithSecrets(gardenSecrets).
		WithImageVector(r.ImageVector).
		WithGarden(gardenObj).
		WithSeed(seedObj).
		WithShoot(shootObj).
		Build(ctx, r.GardenClient, r.SeedClientSet, r.ShootClientMap)
}

func (r *Reconciler) syncClusterResourceToSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	clusterName := shootpkg.ComputeTechnicalID(project.Name, shoot)
	return gardenerextensions.SyncClusterResourceToSeed(ctx, r.SeedClientSet.Client(), clusterName, shoot, cloudProfile, seed)
}

func (r *Reconciler) checkSeedAndSyncClusterResource(ctx context.Context, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	// Don't wait for the Seed to be ready if it is already marked for deletion. In this case
	// it will never get ready because the bootstrap loop is never executed again.
	// Don't block the Shoot deletion flow in this case to allow proper cleanup.
	if seed.DeletionTimestamp == nil {
		if err := health.CheckSeed(seed, r.Identity); err != nil {
			return fmt.Errorf("seed is not yet ready: %w", err)
		}
	}

	if err := r.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); err != nil {
		return fmt.Errorf("could not sync cluster resource to seed: %w", err)
	}

	return nil
}

func (r *Reconciler) finalizeShootMigration(ctx context.Context, shoot *gardencorev1beta1.Shoot, o *operation.Operation) (reconcile.Result, error) {
	if len(shoot.Status.UID) > 0 {
		if err := o.DeleteClusterResourceFromSeed(ctx); err != nil {
			lastErr := v1beta1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", err))
			r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastErr.Description)
			updateErr := r.patchShootStatusOperationError(ctx, shoot, lastErr.Description, gardencorev1beta1.LastOperationTypeMigrate, *lastErr)
			return reconcile.Result{}, errorsutils.WithSuppressed(errors.New(lastErr.Description), updateErr)
		}
	}

	metaPatch := client.MergeFrom(shoot.DeepCopy())
	controllerutils.RemoveAllTasks(shoot.Annotations)
	if err := r.GardenClient.Patch(ctx, shoot, metaPatch); err != nil {
		return reconcile.Result{}, err
	}

	r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventMigrationPrepared, "Prepared Shoot cluster for migration")
	return reconcile.Result{}, r.patchShootStatusOperationSuccess(ctx, shoot, o.Shoot.SeedNamespace, nil, gardencorev1beta1.LastOperationTypeMigrate)
}

func (r *Reconciler) finalizeShootDeletion(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) (reconcile.Result, error) {
	if cleanErr := r.deleteClusterResourceFromSeed(ctx, shoot); cleanErr != nil {
		lastErr := v1beta1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", cleanErr))
		updateErr := r.patchShootStatusOperationError(ctx, shoot, lastErr.Description, gardencorev1beta1.LastOperationTypeDelete, *lastErr)
		r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastErr.Description)
		return reconcile.Result{}, errorsutils.WithSuppressed(errors.New(lastErr.Description), updateErr)
	}

	return reconcile.Result{}, r.removeFinalizerFromShoot(ctx, log, shoot)
}

// deleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (r *Reconciler) deleteClusterResourceFromSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	if shoot.Status.TechnicalID == "" {
		return nil
	}
	cluster := &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: shoot.Status.TechnicalID}}
	return client.IgnoreNotFound(r.SeedClientSet.Client().Delete(ctx, cluster))
}

func (r *Reconciler) removeFinalizerFromShoot(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	if err := r.patchShootStatusOperationSuccess(ctx, shoot, "", nil, gardencorev1beta1.LastOperationTypeDelete); err != nil {
		return err
	}

	if controllerutil.ContainsFinalizer(shoot, gardencorev1beta1.GardenerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, shoot, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	// Wait until the above modifications are reflected in the cache to prevent unwanted reconcile
	// operations (sometimes the cache is not synced fast enough).
	return retryutils.UntilTimeout(ctx, time.Second, 30*time.Second, func(context.Context) (bool, error) {
		err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
		if apierrors.IsNotFound(err) {
			return retryutils.Ok()
		}
		if err != nil {
			return retryutils.SevereError(err)
		}
		lastOperation := shoot.Status.LastOperation
		if !sets.New(shoot.Finalizers...).Has(gardencorev1beta1.GardenerName) && lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeDelete && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
			return retryutils.Ok()
		}
		return retryutils.MinorError(fmt.Errorf("shoot still has finalizer %s", gardencorev1beta1.GardenerName))
	})
}

func (r *Reconciler) newProgressReporter(reporterFn flow.ProgressReporterFn) flow.ProgressReporter {
	if r.Config.Controllers.Shoot != nil && r.Config.Controllers.Shoot.ProgressReportPeriod != nil {
		return flow.NewDelayingProgressReporter(clock.RealClock{}, reporterFn, r.Config.Controllers.Shoot.ProgressReportPeriod.Duration)
	}
	return flow.NewImmediateProgressReporter(reporterFn)
}

func (r *Reconciler) updateShootStatusOperationStart(
	ctx context.Context,
	shoot *gardencorev1beta1.Shoot,
	shootNamespace string,
	operationType gardencorev1beta1.LastOperationType,
) error {
	var (
		now                   = metav1.NewTime(r.Clock.Now().UTC())
		operationTypeSwitched bool
		description           string
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Shoot cluster initialized."
		operationTypeSwitched = false

	case gardencorev1beta1.LastOperationTypeRestore:
		description = "Restoration of Shoot cluster initialized."
		operationTypeSwitched = false

	case gardencorev1beta1.LastOperationTypeMigrate:
		description = "Preparation of Shoot cluster for migration initialized."
		operationTypeSwitched = false

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Shoot cluster in progress."
		operationTypeSwitched = shoot.Status.LastOperation != nil && shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeDelete
	}

	if shoot.Status.RetryCycleStartTime == nil ||
		shoot.Generation != shoot.Status.ObservedGeneration ||
		shoot.Status.Gardener.Version != version.Get().GitVersion ||
		operationTypeSwitched {

		shoot.Status.RetryCycleStartTime = &now
	}

	if len(shoot.Status.UID) == 0 {
		shoot.Status.UID = shoot.UID
	}

	if len(shoot.Status.TechnicalID) == 0 {
		shoot.Status.TechnicalID = shootNamespace
	}

	if !equality.Semantic.DeepEqual(shoot.Status.SeedName, shoot.Spec.SeedName) &&
		operationType != gardencorev1beta1.LastOperationTypeMigrate &&
		operationType != gardencorev1beta1.LastOperationTypeDelete {
		shoot.Status.SeedName = shoot.Spec.SeedName
	}

	shoot.Status.LastErrors = v1beta1helper.DeleteLastErrorByTaskID(shoot.Status.LastErrors, taskID)
	shoot.Status.Gardener = *r.Identity
	shoot.Status.ObservedGeneration = shoot.Generation
	shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: now,
	}

	var mustRemoveOperationAnnotation bool

	switch shoot.Annotations[v1beta1constants.GardenerOperation] {
	case v1beta1constants.OperationRotateCredentialsStart:
		mustRemoveOperationAnnotation = true
		startRotationCA(shoot, &now)
		startRotationServiceAccountKey(shoot, &now)
		startRotationKubeconfig(shoot, &now)
		if v1beta1helper.ShootEnablesSSHAccess(shoot) {
			startRotationSSHKeypair(shoot, &now)
		}
		startRotationObservability(shoot, &now)
		startRotationETCDEncryptionKey(shoot, &now)
	case v1beta1constants.OperationRotateCredentialsComplete:
		mustRemoveOperationAnnotation = true
		completeRotationCA(shoot, &now)
		completeRotationServiceAccountKey(shoot, &now)
		completeRotationETCDEncryptionKey(shoot, &now)

	case v1beta1constants.OperationRotateCAStart:
		mustRemoveOperationAnnotation = true
		startRotationCA(shoot, &now)
	case v1beta1constants.OperationRotateCAComplete:
		mustRemoveOperationAnnotation = true
		completeRotationCA(shoot, &now)

	case v1beta1constants.ShootOperationRotateKubeconfigCredentials:
		mustRemoveOperationAnnotation = true
		startRotationKubeconfig(shoot, &now)

	case v1beta1constants.ShootOperationRotateSSHKeypair:
		mustRemoveOperationAnnotation = true
		if v1beta1helper.ShootEnablesSSHAccess(shoot) {
			startRotationSSHKeypair(shoot, &now)
		}

	case v1beta1constants.ShootOperationRotateObservabilityCredentials:
		mustRemoveOperationAnnotation = true
		startRotationObservability(shoot, &now)

	case v1beta1constants.OperationRotateServiceAccountKeyStart:
		mustRemoveOperationAnnotation = true
		startRotationServiceAccountKey(shoot, &now)
	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		mustRemoveOperationAnnotation = true
		completeRotationServiceAccountKey(shoot, &now)

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		mustRemoveOperationAnnotation = true
		startRotationETCDEncryptionKey(shoot, &now)
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		mustRemoveOperationAnnotation = true
		completeRotationETCDEncryptionKey(shoot, &now)
	}

	if err := r.GardenClient.Status().Update(ctx, shoot); err != nil {
		return err
	}

	if mustRemoveOperationAnnotation {
		patch := client.MergeFrom(shoot.DeepCopy())
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
		return r.GardenClient.Patch(ctx, shoot, patch)
	}

	return nil
}

func (r *Reconciler) patchShootStatusOperationSuccess(
	ctx context.Context,
	shoot *gardencorev1beta1.Shoot,
	shootSeedNamespace string,
	seedName *string,
	operationType gardencorev1beta1.LastOperationType,
) error {
	var (
		now                        = metav1.NewTime(r.Clock.Now().UTC())
		description                string
		setConditionsToProgressing bool
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Shoot cluster has been successfully reconciled."
		setConditionsToProgressing = true

	case gardencorev1beta1.LastOperationTypeMigrate:
		description = "Shoot cluster has been successfully prepared for migration."
		setConditionsToProgressing = false

	case gardencorev1beta1.LastOperationTypeRestore:
		description = "Shoot cluster has been successfully restored."
		setConditionsToProgressing = true

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Shoot cluster has been successfully deleted."
		setConditionsToProgressing = false
	}

	patch := client.StrategicMergeFrom(shoot.DeepCopy())

	if len(shootSeedNamespace) > 0 && seedName != nil {
		isHibernated, err := r.isHibernationActive(ctx, shootSeedNamespace, seedName)
		if err != nil {
			return fmt.Errorf("error updating Shoot (%s/%s) after successful reconciliation when checking for active hibernation: %w", shoot.Namespace, shoot.Name, err)
		}
		shoot.Status.IsHibernated = isHibernated
	}

	if setConditionsToProgressing {
		for i, cond := range shoot.Status.Conditions {
			switch cond.Type {
			case gardencorev1beta1.ShootAPIServerAvailable,
				gardencorev1beta1.ShootControlPlaneHealthy,
				gardencorev1beta1.ShootObservabilityComponentsHealthy,
				gardencorev1beta1.ShootEveryNodeReady,
				gardencorev1beta1.ShootSystemComponentsHealthy:
				if cond.Status != gardencorev1beta1.ConditionFalse {
					shoot.Status.Conditions[i].Status = gardencorev1beta1.ConditionProgressing
					shoot.Status.Conditions[i].LastUpdateTime = metav1.Now()
				}
			}
		}
		for i, constr := range shoot.Status.Constraints {
			switch constr.Type {
			case gardencorev1beta1.ShootHibernationPossible,
				gardencorev1beta1.ShootMaintenancePreconditionsSatisfied:
				if constr.Status != gardencorev1beta1.ConditionFalse {
					shoot.Status.Constraints[i].Status = gardencorev1beta1.ConditionProgressing
					shoot.Status.Conditions[i].LastUpdateTime = metav1.Now()
				}
			}
		}
	}

	if operationType != gardencorev1beta1.LastOperationTypeDelete {
		shoot.Status.SeedName = shoot.Spec.SeedName
	}

	shoot.Status.RetryCycleStartTime = nil
	shoot.Status.LastErrors = nil
	shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    description,
		LastUpdateTime: now,
	}

	switch v1beta1helper.GetShootCARotationPhase(shoot.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		v1beta1helper.MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		v1beta1helper.MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	switch v1beta1helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		v1beta1helper.MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		v1beta1helper.MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	switch v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		v1beta1helper.MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		v1beta1helper.MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	if v1beta1helper.IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime(shoot.Status.Credentials) {
		v1beta1helper.MutateShootKubeconfigRotation(shoot, func(rotation *gardencorev1beta1.ShootKubeconfigRotation) {
			rotation.LastCompletionTime = &now
		})
	}

	if v1beta1helper.IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime(shoot.Status.Credentials) {
		v1beta1helper.MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
			rotation.LastCompletionTime = &now
		})
	}

	if v1beta1helper.IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime(shoot.Status.Credentials) {
		v1beta1helper.MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ShootObservabilityRotation) {
			rotation.LastCompletionTime = &now
		})
	}

	if shoot.Status.Credentials != nil && shoot.Status.Credentials.Rotation != nil {
		if pointer.BoolEqual(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, pointer.Bool(false)) {
			shoot.Status.Credentials.Rotation.Kubeconfig = nil
		}

		if !v1beta1helper.ShootEnablesSSHAccess(shoot) {
			shoot.Status.Credentials.Rotation.SSHKeypair = nil
		}
	}

	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}

func (r *Reconciler) patchShootStatusOperationError(
	ctx context.Context,
	shoot *gardencorev1beta1.Shoot,
	description string,
	operationType gardencorev1beta1.LastOperationType,
	lastErrors ...gardencorev1beta1.LastError,
) error {
	var (
		now          = metav1.NewTime(r.Clock.Now().UTC())
		state        = gardencorev1beta1.LastOperationStateError
		willNotRetry = v1beta1helper.HasNonRetryableErrorCode(lastErrors...) || utils.TimeElapsed(shoot.Status.RetryCycleStartTime, r.Config.Controllers.Shoot.RetryDuration.Duration)
	)

	statusPatch := client.StrategicMergeFrom(shoot.DeepCopy())

	if willNotRetry {
		state = gardencorev1beta1.LastOperationStateFailed
		shoot.Status.RetryCycleStartTime = nil
	} else {
		description += " Operation will be retried."
	}

	shoot.Status.Gardener = *r.Identity
	shoot.Status.LastErrors = lastErrors

	if shoot.Status.LastOperation == nil {
		shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	shoot.Status.LastOperation.Type = operationType
	shoot.Status.LastOperation.State = state
	shoot.Status.LastOperation.Description = description
	shoot.Status.LastOperation.LastUpdateTime = now

	return r.GardenClient.Status().Patch(ctx, shoot, statusPatch)
}

func (r *Reconciler) shootHasBastions(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, r.GardenClient, operationsv1alpha1.SchemeGroupVersion.WithKind("BastionList"), client.MatchingFields{operations.BastionShootName: shoot.Name})
}

// isHibernationActive uses the Cluster resource in the Seed to determine whether the Shoot is hibernated
// The Cluster contains the actual or "active" spec of the Shoot resource for this reconciliation
// as the Shoot resources field `spec.hibernation.enabled` might have changed during the reconciliation
func (r *Reconciler) isHibernationActive(ctx context.Context, shootSeedNamespace string, seedName *string) (bool, error) {
	if seedName == nil {
		return false, nil
	}

	cluster, err := gardenerextensions.GetCluster(ctx, r.SeedClientSet.Client(), shootSeedNamespace)
	if err != nil {
		return false, err
	}

	shoot := cluster.Shoot
	if shoot == nil {
		return false, fmt.Errorf("shoot is missing in cluster resource: %w", err)
	}

	return v1beta1helper.HibernationIsEnabled(shoot), nil
}

func lastErrorsOperationInitializationFailure(lastErrors []gardencorev1beta1.LastError, err error) []gardencorev1beta1.LastError {
	var incompleteDNSConfigError *gardenerutils.IncompleteDNSConfigError

	if errors.As(err, &incompleteDNSConfigError) {
		return v1beta1helper.UpsertLastError(lastErrors, gardencorev1beta1.LastError{
			TaskID:      pointer.String(taskID),
			Description: err.Error(),
			Codes:       []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorConfigurationProblem},
		})
	}

	return lastErrors
}

func needsControlPlaneDeployment(ctx context.Context, o *operation.Operation, kubeAPIServerDeploymentFound bool, infrastructure *extensionsv1alpha1.Infrastructure) (bool, error) {
	var (
		namespace = o.Shoot.SeedNamespace
		name      = o.Shoot.GetInfo().Name
	)

	// If the `ControlPlane` resource and the kube-apiserver deployment do no longer exist then we don't want to re-deploy it.
	// The reason for the second condition is that some providers inject a cloud-provider-config into the kube-apiserver deployment
	// which is needed for it to run.
	exists, markedForDeletion, err := extensionResourceStillExists(ctx, o.SeedClientSet.APIReader(), &extensionsv1alpha1.ControlPlane{}, namespace, name)
	if err != nil {
		return false, err
	}

	switch {
	// treat `ControlPlane` in deletion as if it is already gone. If it is marked for deletion, we also shouldn't wait
	// for it to be reconciled, as it can potentially block the whole deletion flow (deletion depends on other control
	// plane components like kcm and grm) which are scaled up later in the flow
	case !exists && !kubeAPIServerDeploymentFound || markedForDeletion:
		return false, nil
	// The infrastructure resource has not been found, no need to redeploy the control plane
	case infrastructure == nil:
		return false, nil
	// The infrastructure resource has been found with a non-nil provider status, so redeploy the control plane
	case infrastructure.Status.ProviderStatus != nil:
		return true, nil
	default:
		return false, nil
	}
}

func extensionResourceStillExists(ctx context.Context, reader client.Reader, obj client.Object, namespace, name string) (bool, bool, error) {
	if err := reader.Get(ctx, kubernetesutils.Key(namespace, name), obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}
	return true, obj.GetDeletionTimestamp() != nil, nil
}

func checkIfSeedNamespaceExists(ctx context.Context, o *operation.Operation, botanist *botanistpkg.Botanist) error {
	botanist.SeedNamespaceObject = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: o.Shoot.SeedNamespace}}
	if err := botanist.SeedClientSet.APIReader().Get(ctx, client.ObjectKeyFromObject(botanist.SeedNamespaceObject), botanist.SeedNamespaceObject); err != nil {
		if apierrors.IsNotFound(err) {
			o.Logger.Info("Did not find namespace in the Seed cluster", "namespace", client.ObjectKeyFromObject(o.SeedNamespaceObject))
			return nil
		}
		return err
	}
	return nil
}

func startRotationCA(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationCA(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationServiceAccountKey(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationServiceAccountKey(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationETCDEncryptionKey(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationETCDEncryptionKey(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationKubeconfig(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootKubeconfigRotation(shoot, func(rotation *gardencorev1beta1.ShootKubeconfigRotation) {
		rotation.LastInitiationTime = now
	})
}

func startRotationSSHKeypair(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
		rotation.LastInitiationTime = now
	})
}

func startRotationObservability(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ShootObservabilityRotation) {
		rotation.LastInitiationTime = now
	})
}
