// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Actuator acts upon ManagedSeed resources.
type Actuator interface {
	// Reconcile reconciles ManagedSeed creation or update.
	Reconcile(context.Context, logr.Logger, *seedmanagementv1alpha1.ManagedSeed) (*seedmanagementv1alpha1.ManagedSeedStatus, bool, error)
	// Delete reconciles ManagedSeed deletion.
	Delete(context.Context, logr.Logger, *seedmanagementv1alpha1.ManagedSeed) (*seedmanagementv1alpha1.ManagedSeedStatus, bool, bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenConfig         *rest.Config
	gardenAPIReader      client.Reader
	gardenClient         client.Client
	seedClient           client.Client
	shootClientMap       clientmap.ClientMap
	clock                clock.Clock
	vp                   ValuesHelper
	recorder             record.EventRecorder
	gardenNamespaceShoot string
}

// newActuator creates a new Actuator with the given clients, ValuesHelper, and logger.
func newActuator(
	gardenConfig *rest.Config,
	gardenAPIReader client.Reader,
	gardenClient, seedClient client.Client,
	shootClientMap clientmap.ClientMap,
	clock clock.Clock,
	vp ValuesHelper,
	recorder record.EventRecorder,
	gardenNamespaceShoot string,
) Actuator {
	return &actuator{
		gardenConfig:         gardenConfig,
		gardenAPIReader:      gardenAPIReader,
		gardenClient:         gardenClient,
		seedClient:           seedClient,
		shootClientMap:       shootClientMap,
		clock:                clock,
		vp:                   vp,
		recorder:             recorder,
		gardenNamespaceShoot: gardenNamespaceShoot,
	}
}

// Reconcile reconciles ManagedSeed creation or update.
func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	status *seedmanagementv1alpha1.ManagedSeedStatus,
	wait bool,
	err error,
) {
	// Initialize status
	status = ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	defer func() {
		if err != nil {
			log.Error(err, "Error during reconciliation")
			a.recorder.Eventf(ms, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
			updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := a.gardenAPIReader.Get(ctx, client.ObjectKey{Namespace: ms.Namespace, Name: ms.Spec.Shoot.Name}, shoot); err != nil {
		return status, false, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	log = log.WithValues("shootName", shoot.Name)

	// Check if shoot is reconciled and update ShootReconciled condition
	if !shootReconciled(shoot) {
		log.Info("Waiting for shoot to be reconciled")

		msg := fmt.Sprintf("Waiting for shoot %q to be reconciled", client.ObjectKeyFromObject(shoot).String())
		a.recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, msg)
		updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconciling, msg)

		return status, true, nil
	}
	updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Shoot %q has been reconciled", client.ObjectKeyFromObject(shoot).String()))

	// Get shoot client
	shootClient, err := a.shootClientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return status, false, fmt.Errorf("could not get shoot client for shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(ms.Name, helper.GardenletConfigFromManagedSeed(ms.Spec.Gardenlet))
	if err != nil {
		return status, false, err
	}

	// Check seed spec
	if err := a.checkSeedSpec(ctx, &seedTemplate.Spec, shoot); err != nil {
		return status, false, err
	}

	// Create or update garden namespace in the shoot
	log.Info("Ensuring garden namespace in shoot")
	a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in shoot %q", client.ObjectKeyFromObject(shoot).String())
	if err := a.ensureGardenNamespace(ctx, shootClient.Client()); err != nil {
		return status, false, fmt.Errorf("could not create or update garden namespace in shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
	}

	// Create or update seed secrets
	log.Info("Reconciling seed secrets")
	a.recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
	if err := a.reconcileSeedSecrets(ctx, &seedTemplate.Spec, ms, shoot); err != nil {
		return status, false, fmt.Errorf("could not reconcile seed %s secrets: %w", ms.Name, err)
	}

	if ms.Spec.Gardenlet != nil {
		seed, err := a.getSeed(ctx, ms)
		if err != nil {
			return status, false, fmt.Errorf("could not read seed %s: %w", ms.Name, err)
		}

		// Deploy gardenlet into the shoot, it will register the seed automatically
		log.Info("Deploying gardenlet into shoot")
		a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into shoot %q", client.ObjectKeyFromObject(shoot).String())
		if err := a.deployGardenlet(ctx, log, shootClient, ms, seed, gardenletConfig, shoot); err != nil {
			return status, false, fmt.Errorf("could not deploy gardenlet into shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
		}
	}

	updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled,
		fmt.Sprintf("Seed %s has been registered", ms.Name))
	return status, false, nil
}

// Delete reconciles ManagedSeed deletion.
func (a *actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	status *seedmanagementv1alpha1.ManagedSeedStatus,
	wait, removeFinalizer bool,
	err error,
) {
	// Initialize status
	status = ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	defer func() {
		if err != nil {
			log.Error(err, "Error during deletion")
			a.recorder.Eventf(ms, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
			updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error())
		}
	}()

	// Update SeedRegistered condition
	updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting,
		"Unregistering seed "+ms.Name)

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := a.gardenAPIReader.Get(ctx, client.ObjectKey{Namespace: ms.Namespace, Name: ms.Spec.Shoot.Name}, shoot); err != nil {
		return status, false, false, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	log = log.WithValues("shootName", shoot.Name)

	// Get shoot client
	shootClient, err := a.shootClientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return status, false, false, fmt.Errorf("could not get shoot client for shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(ms.Name, helper.GardenletConfigFromManagedSeed(ms.Spec.Gardenlet))
	if err != nil {
		return status, false, false, err
	}

	// Delete seed if it still exists and is not already deleting
	seed, err := a.getSeed(ctx, ms)
	if err != nil {
		return status, false, false, fmt.Errorf("could not get seed %s: %w", ms.Name, err)
	}

	if seed != nil {
		log = log.WithValues("seedName", seed.Name)

		if seed.DeletionTimestamp == nil {
			log.Info("Deleting seed")
			a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed %s", ms.Name)
			if err := a.deleteSeed(ctx, ms); err != nil {
				return status, false, false, fmt.Errorf("could not delete seed %s: %w", ms.Name, err)
			}
		} else {
			log.Info("Waiting for seed to be deleted")
			a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed %q to be deleted", ms.Name)
		}

		return status, false, false, nil
	}

	if ms.Spec.Gardenlet != nil {
		// Delete gardenlet from the shoot if it still exists and is not already deleting
		gardenletDeployment, err := a.getGardenletDeployment(ctx, shootClient)
		if err != nil {
			return status, false, false, fmt.Errorf("could not get gardenlet deployment in shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
		}

		if gardenletDeployment != nil {
			if gardenletDeployment.DeletionTimestamp == nil {
				log.Info("Deleting gardenlet from shoot")
				a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting gardenlet from shoot %q", client.ObjectKeyFromObject(shoot).String())
				if err := a.deleteGardenlet(ctx, log, shootClient, ms, seed, gardenletConfig, shoot); err != nil {
					return status, false, false, fmt.Errorf("could delete gardenlet from shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
				}
			} else {
				log.Info("Waiting for gardenlet to be deleted from shoot")
				a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for gardenlet to be deleted from shoot %q", client.ObjectKeyFromObject(shoot).String())
			}

			return status, true, false, nil
		}
	}

	// Delete seed backup secrets if any of them still exists and is not already deleting
	backupSecret, err := a.getBackupSecret(ctx, &seedTemplate.Spec, ms)
	if err != nil {
		return status, false, false, fmt.Errorf("could not get seed %s secrets: %w", ms.Name, err)
	}

	if backupSecret != nil {
		if backupSecret.DeletionTimestamp == nil {
			log.Info("Deleting seed secrets")
			a.recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed secrets")
			if err := a.deleteBackupSecret(ctx, &seedTemplate.Spec, ms); err != nil {
				return status, false, false, fmt.Errorf("could not delete seed %s secrets: %w", ms.Name, err)
			}
		} else {
			log.Info("Waiting for seed secrets to be deleted")
			a.recorder.Event(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed secrets to be deleted")
		}

		return status, true, false, nil
	}

	// Delete garden namespace from the shoot if it still exists and is not already deleting
	gardenNamespace, err := a.getGardenNamespace(ctx, shootClient)
	if err != nil {
		return status, false, false, fmt.Errorf("could not check if garden namespace exists in shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
	}

	if gardenNamespace != nil {
		if gardenNamespace.DeletionTimestamp == nil {
			log.Info("Deleting garden namespace from shoot")
			a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting garden namespace from shoot %q", client.ObjectKeyFromObject(shoot).String())
			if err := a.deleteGardenNamespace(ctx, shootClient); err != nil {
				return status, false, false, fmt.Errorf("could not delete garden namespace from shoot %s: %w", client.ObjectKeyFromObject(shoot).String(), err)
			}
		} else {
			log.Info("Waiting for garden namespace to be deleted from shoot")
			a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for garden namespace to be deleted from shoot %q", client.ObjectKeyFromObject(shoot).String())
		}

		return status, true, false, nil
	}

	updateCondition(a.clock, status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleted,
		fmt.Sprintf("Seed %s has been unregistred", ms.Name))
	return status, false, true, nil
}

func (a *actuator) ensureGardenNamespace(ctx context.Context, shootClient client.Client) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.gardenNamespaceShoot,
		},
	}
	if err := shootClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespace), gardenNamespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return shootClient.Create(ctx, gardenNamespace)
	}
	return nil
}

func (a *actuator) deleteGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.gardenNamespaceShoot,
		},
	}
	return client.IgnoreNotFound(shootClient.Client().Delete(ctx, gardenNamespace))
}

func (a *actuator) getGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	if err := shootClient.Client().Get(ctx, client.ObjectKey{Name: a.gardenNamespaceShoot}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ns, nil
}

func (a *actuator) deleteSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	return client.IgnoreNotFound(a.gardenClient.Delete(ctx, seed))
}

func (a *actuator) getSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := a.gardenClient.Get(ctx, client.ObjectKey{Name: managedSeed.Name}, seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return seed, nil
}

func (a *actuator) deployGardenlet(
	ctx context.Context,
	log logr.Logger,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
	shoot *gardencorev1beta1.Shoot,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		shootClient,
		managedSeed,
		seed,
		gardenletConfig,
		helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		ptr.Deref(managedSeed.Spec.Gardenlet.MergeWithParent, false),
		shoot,
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	if err := shootClient.ChartApplier().ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, a.gardenNamespaceShoot, "gardenlet", kubernetes.Values(values)); err != nil {
		return err
	}

	// remove renew-kubeconfig annotation, if it exists
	if managedSeed.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		patch := client.MergeFrom(managedSeed.DeepCopy())
		delete(managedSeed.Annotations, v1beta1constants.GardenerOperation)
		if err := a.gardenClient.Patch(ctx, managedSeed, patch); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) deleteGardenlet(
	ctx context.Context,
	log logr.Logger,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
	shoot *gardencorev1beta1.Shoot,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		shootClient,
		managedSeed,
		seed,
		gardenletConfig,
		helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		ptr.Deref(managedSeed.Spec.Gardenlet.MergeWithParent, false),
		shoot,
	)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	return shootClient.ChartApplier().DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, a.gardenNamespaceShoot, "gardenlet", kubernetes.Values(values))
}

func (a *actuator) getGardenletDeployment(ctx context.Context, shootClient kubernetes.Interface) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := shootClient.Client().Get(ctx, client.ObjectKey{Namespace: a.gardenNamespaceShoot, Name: v1beta1constants.DeploymentNameGardenlet}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (a *actuator) checkSeedSpec(ctx context.Context, spec *gardencorev1beta1.SeedSpec, shoot *gardencorev1beta1.Shoot) error {
	// If VPA is enabled, check if the shoot namespace in the seed contains a vpa-admission-controller deployment
	if v1beta1helper.SeedSettingVerticalPodAutoscalerEnabled(spec.Settings) {
		seedVPAAdmissionControllerExists, err := a.seedVPADeploymentExists(ctx, a.seedClient, shoot)
		if err != nil {
			return err
		}
		if seedVPAAdmissionControllerExists {
			return errors.New("seed VPA is enabled but shoot already has a VPA")
		}
	}

	return nil
}

func (a *actuator) reconcileSeedSecrets(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Get shoot secret
	shootSecret, err := a.getShootSecret(ctx, shoot)
	if err != nil {
		return err
	}

	// If backup is specified, create or update the backup secret if it doesn't exist or is owned by the managed seed
	if spec.Backup != nil {
		var checksum string

		// Get backup secret
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.gardenClient, &spec.Backup.SecretRef)
		if err == nil {
			checksum = utils.ComputeSecretChecksum(backupSecret.Data)[:8]
		} else if client.IgnoreNotFound(err) != nil {
			return err
		}

		// Create or update backup secret if it doesn't exist or is owned by the managed seed
		if apierrors.IsNotFound(err) || metav1.IsControlledBy(backupSecret, managedSeed) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: spec.Backup.SecretRef.Namespace, Name: spec.Backup.SecretRef.Name},
			}
			if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.gardenClient, secret, func() error {
				secret.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				}
				secret.Type = corev1.SecretTypeOpaque
				secret.Data = shootSecret.Data
				return nil
			}); err != nil {
				return err
			}

			checksum = utils.ComputeSecretChecksum(secret.Data)[:8]
		}

		// Inject backup-secret hash into the pod annotations
		managedSeed.Spec.Gardenlet.Deployment.PodAnnotations = utils.MergeStringMaps[string](managedSeed.Spec.Gardenlet.Deployment.PodAnnotations, map[string]string{
			"checksum/seed-backup-secret": spec.Backup.SecretRef.Name + "-" + checksum,
		})
	}

	return nil
}

func (a *actuator) deleteBackupSecret(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	// If backup is specified, delete the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.gardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil && metav1.IsControlledBy(backupSecret, managedSeed) {
			if err := kubernetesutils.DeleteSecretByReference(ctx, a.gardenClient, &spec.Backup.SecretRef); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *actuator) getBackupSecret(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed) (*corev1.Secret, error) {
	var (
		backupSecret *corev1.Secret
		err          error
	)

	// If backup is specified, get the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err = kubernetesutils.GetSecretByReference(ctx, a.gardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		if backupSecret != nil && !metav1.IsControlledBy(backupSecret, managedSeed) {
			backupSecret = nil
		}
	}

	return backupSecret, nil
}

func (a *actuator) getShootSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootSecretBinding := &gardencorev1beta1.SecretBinding{}
	if shoot.Spec.SecretBindingName == nil {
		return nil, fmt.Errorf("secretbinding name is nil for the Shoot: %s/%s", shoot.Namespace, shoot.Name)
	}
	if err := a.gardenClient.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: *shoot.Spec.SecretBindingName}, shootSecretBinding); err != nil {
		return nil, err
	}
	return kubernetesutils.GetSecretByReference(ctx, a.gardenClient, &shootSecretBinding.SecretRef)
}

func (a *actuator) seedVPADeploymentExists(ctx context.Context, seedClient client.Client, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := seedClient.Get(ctx, client.ObjectKey{Namespace: shoot.Status.TechnicalID, Name: "vpa-admission-controller"}, &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *actuator) prepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *gardenletv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
	shoot *gardencorev1beta1.Shoot,
) (map[string]any, error) {
	// Merge gardenlet deployment with parent values
	deployment, err := a.vp.MergeGardenletDeployment(managedSeed.Spec.Gardenlet.Deployment, shoot)
	if err != nil {
		return nil, err
	}

	// Merge gardenlet configuration with parent if specified
	if mergeWithParent {
		gardenletConfig, err = a.vp.MergeGardenletConfiguration(gardenletConfig)
		if err != nil {
			return nil, err
		}
	}

	// Ensure garden client connection is set
	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletv1alpha1.GardenClientConnection{}
	}

	// Prepare garden client connection
	var bootstrapKubeconfig string
	if bootstrap == seedmanagementv1alpha1.BootstrapNone {
		a.removeBootstrapConfigFromGardenClientConnection(gardenletConfig.GardenClientConnection)
	} else {
		bootstrapKubeconfig, err = a.prepareGardenClientConnectionWithBootstrap(ctx, log, shootClient, gardenletConfig.GardenClientConnection, managedSeed, seed, bootstrap)
		if err != nil {
			return nil, err
		}
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &gardenletv1alpha1.SeedConfig{}
	}

	// Set the seed name
	gardenletConfig.SeedConfig.SeedTemplate.Name = managedSeed.Name

	// Get gardenlet chart values
	return a.vp.GetGardenletChartValues(
		ensureGardenletEnvironment(deployment, shoot.Spec.DNS),
		gardenletConfig,
		bootstrapKubeconfig,
	)
}

// ensureGardenletEnvironment sets the KUBERNETES_SERVICE_HOST to the API of the ManagedSeed cluster.
// This is needed so that the deployed gardenlet can properly set the network policies allowing
// access of control plane components of the hosted shoots to the API server of the (managed) seed.
func ensureGardenletEnvironment(deployment *seedmanagementv1alpha1.GardenletDeployment, dns *gardencorev1beta1.DNS) *seedmanagementv1alpha1.GardenletDeployment {
	const kubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	var shootDomain = ""

	if deployment.Env == nil {
		deployment.Env = []corev1.EnvVar{}
	}

	for _, env := range deployment.Env {
		if env.Name == kubernetesServiceHost {
			return deployment
		}
	}

	if dns != nil && dns.Domain != nil && len(*dns.Domain) != 0 {
		shootDomain = gardenerutils.GetAPIServerDomain(*dns.Domain)
	}

	if len(shootDomain) != 0 {
		deployment.Env = append(
			deployment.Env,
			corev1.EnvVar{
				Name:  kubernetesServiceHost,
				Value: shootDomain,
			},
		)
	}

	return deployment
}

func (a *actuator) prepareGardenClientConnectionWithBootstrap(
	ctx context.Context,
	log logr.Logger,
	shootClient kubernetes.Interface,
	gcc *gardenletv1alpha1.GardenClientConnection,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	bootstrap seedmanagementv1alpha1.Bootstrap,
) (
	string,
	error,
) {
	// Ensure kubeconfig secret is set
	if gcc.KubeconfigSecret == nil {
		gcc.KubeconfigSecret = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigSecretName,
			Namespace: a.gardenNamespaceShoot,
		}
	}

	if seed != nil && seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Check if client certificate is expired. If yes then delete the existing kubeconfig secret to make sure that the
		// seed can be re-bootstrapped.
		if err := kubernetesutils.DeleteSecretByReference(ctx, shootClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else if managedSeed.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		// Also remove the kubeconfig if the renew-kubeconfig operation annotation is set on the ManagedSeed resource.
		log.Info("Renewing gardenlet kubeconfig secret due to operation annotation")
		a.recorder.Event(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Renewing gardenlet kubeconfig secret due to operation annotation")

		if err := kubernetesutils.DeleteSecretByReference(ctx, shootClient.Client(), gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else {
		seedIsAlreadyBootstrapped, err := isAlreadyBootstrapped(ctx, shootClient.Client(), gcc.KubeconfigSecret)
		if err != nil {
			return "", err
		}

		if seedIsAlreadyBootstrapped {
			return "", nil
		}
	}

	// Ensure kubeconfig is not set
	gcc.Kubeconfig = ""

	// Ensure bootstrap kubeconfig secret is set
	if gcc.BootstrapKubeconfig == nil {
		gcc.BootstrapKubeconfig = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigBootstrapSecretName,
			Namespace: a.gardenNamespaceShoot,
		}
	}

	return a.createBootstrapKubeconfig(ctx, managedSeed.ObjectMeta, bootstrap, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
}

// isAlreadyBootstrapped checks if the gardenlet already has a valid Garden cluster certificate through TLS bootstrapping
// by checking if the specified secret reference already exists
func isAlreadyBootstrapped(ctx context.Context, c client.Client, s *corev1.SecretReference) (bool, error) {
	// If kubeconfig secret exists, return an empty result, since the bootstrap can be skipped
	secret, err := kubernetesutils.GetSecretByReference(ctx, c, s)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return secret != nil, nil
}

func (a *actuator) removeBootstrapConfigFromGardenClientConnection(gcc *gardenletv1alpha1.GardenClientConnection) {
	// Ensure kubeconfig secret and bootstrap kubeconfig secret are not set
	gcc.KubeconfigSecret = nil
	gcc.BootstrapKubeconfig = nil
}

// createBootstrapKubeconfig creates a kubeconfig for the Garden cluster
// containing either the token of a service account or a bootstrap token
// returns the kubeconfig as a string
func (a *actuator) createBootstrapKubeconfig(ctx context.Context, objectMeta metav1.ObjectMeta, bootstrap seedmanagementv1alpha1.Bootstrap, address *string, caCert []byte) (string, error) {
	var (
		err                 error
		bootstrapKubeconfig []byte
	)

	gardenClientRestConfig := gardenerutils.PrepareGardenClientRestConfig(a.gardenConfig, address, caCert)

	switch bootstrap {
	case seedmanagementv1alpha1.BootstrapServiceAccount:
		// Create a kubeconfig containing the token of a temporary service account as client credentials
		var (
			serviceAccountName      = gardenletbootstraputil.ServiceAccountName(objectMeta.Name)
			serviceAccountNamespace = objectMeta.Namespace
		)

		// Create a kubeconfig containing a valid service account token as client credentials
		kubernetesClientSet, err := kubernetesclientset.NewForConfig(gardenClientRestConfig)
		if err != nil {
			return "", fmt.Errorf("failed creating Kubernetes client: %w", err)
		}

		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, a.gardenClient, kubernetesClientSet.CoreV1(), gardenClientRestConfig, serviceAccountName, serviceAccountNamespace)
		if err != nil {
			return "", err
		}

	case seedmanagementv1alpha1.BootstrapToken:
		var (
			tokenID          = gardenletbootstraputil.TokenID(objectMeta)
			tokenDescription = gardenletbootstraputil.Description(gardenletbootstraputil.KindManagedSeed, objectMeta.Namespace, objectMeta.Name)
			tokenValidity    = 24 * time.Hour
		)

		// Create a kubeconfig containing a valid bootstrap token as client credentials
		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, a.gardenClient, gardenClientRestConfig, tokenID, tokenDescription, tokenValidity)
		if err != nil {
			return "", err
		}
	}

	return string(bootstrapKubeconfig), nil
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
