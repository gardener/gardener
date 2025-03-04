// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletdeployer

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// GardenletDefaultKubeconfigSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.KubeconfigSecret.Name
	GardenletDefaultKubeconfigSecretName = "gardenlet-kubeconfig" // #nosec G101 -- No credential.
	// GardenletDefaultKubeconfigBootstrapSecretName is the default name for the field in the Gardenlet component configuration
	// .gardenClientConnection.BootstrapKubeconfig.Name
	GardenletDefaultKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap" // #nosec G101 -- No credential.
)

// Interface deploys gardenlets into target clusters.
type Interface interface {
	// Reconcile deploys or updates a gardenlet in a target cluster.
	Reconcile(context.Context, logr.Logger, client.Object, []gardencorev1beta1.Condition, *seedmanagementv1alpha1.GardenletDeployment, *runtime.RawExtension, seedmanagementv1alpha1.Bootstrap, bool) ([]gardencorev1beta1.Condition, error)
	// Delete deletes a gardenlet from a target cluster.
	Delete(context.Context, logr.Logger, client.Object, []gardencorev1beta1.Condition, *seedmanagementv1alpha1.GardenletDeployment, *runtime.RawExtension, seedmanagementv1alpha1.Bootstrap, bool) ([]gardencorev1beta1.Condition, bool, bool, error)
}

// Actuator is a concrete implementation of Interface.
type Actuator struct {
	GardenConfig            *rest.Config
	GardenClient            client.Client
	GetTargetClientFunc     func(ctx context.Context) (kubernetes.Interface, error)
	CheckIfVPAAlreadyExists func(ctx context.Context) (bool, error)
	GetInfrastructureSecret func(ctx context.Context) (*corev1.Secret, error)
	GetTargetDomain         func() string
	ApplyGardenletChart     func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error
	DeleteGardenletChart    func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error
	Clock                   clock.Clock
	ValuesHelper            ValuesHelper
	Recorder                record.EventRecorder
	GardenNamespaceTarget   string
}

// Reconcile deploys or updates gardenlets.
func (a *Actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	conditions []gardencorev1beta1.Condition,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	rawComponentConfig *runtime.RawExtension,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
) (
	[]gardencorev1beta1.Condition,
	error,
) {
	// Get target client
	targetClient, err := a.GetTargetClientFunc(ctx)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), fmt.Errorf("could not get target client: %w", err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, componentConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(obj.GetName(), rawComponentConfig)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), err
	}

	// Check seed spec
	if err := a.checkSeedSpec(ctx, &seedTemplate.Spec); err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), err
	}

	// Create or update garden namespace in the target cluster
	log.Info("Ensuring garden namespace in target cluster")
	a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Ensuring garden namespace in target cluster")
	if err := a.ensureGardenNamespace(ctx, targetClient.Client()); err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), fmt.Errorf("could not create or update garden namespace in target cluster: %w", err)
	}

	// Create or update seed secrets
	log.Info("Reconciling seed secrets")
	a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling seed secrets")
	if err := a.reconcileSeedSecrets(ctx, obj, &seedTemplate.Spec, gardenletDeployment); err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), fmt.Errorf("could not reconcile seed %s secrets: %w", obj.GetName(), err)
	}

	seed, err := GetSeed(ctx, a.GardenClient, obj.GetName())
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), fmt.Errorf("could not read seed %s: %w", obj.GetName(), err)
	}

	// Deploy gardenlet into the target cluster, it will register the seed automatically
	log.Info("Deploying gardenlet into target cluster")
	a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Deploying gardenlet into target cluster")
	if err := a.deployGardenlet(ctx, log, obj, targetClient, gardenletDeployment, seed, componentConfig, bootstrap, mergeWithParent); err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error()), fmt.Errorf("could not deploy gardenlet into target cluster: %w", err)
	}

	return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled, fmt.Sprintf("Seed %s has been registered", obj.GetName())), nil
}

// Delete deletes the gardenlet.
func (a *Actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	conditions []gardencorev1beta1.Condition,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	rawComponentConfig *runtime.RawExtension,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
) (
	[]gardencorev1beta1.Condition,
	bool,
	bool,
	error,
) {
	// Update SeedRegistered condition
	conditions = updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, "Unregistering seed "+obj.GetName())

	// Get target client
	targetClient, err := a.GetTargetClientFunc(ctx)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not get target client: %w", err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, componentConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(obj.GetName(), rawComponentConfig)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, err
	}

	// Delete seed if it still exists and is not already deleting
	seed, err := GetSeed(ctx, a.GardenClient, obj.GetName())
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not get seed %s: %w", obj.GetName(), err)
	}

	if seed != nil {
		log = log.WithValues("seedName", seed.Name)

		if seed.DeletionTimestamp == nil {
			log.Info("Deleting seed")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed %s", obj.GetName())
			if err := a.deleteSeed(ctx, obj); err != nil {
				a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
				return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not delete seed %s: %w", obj.GetName(), err)
			}
		} else {
			log.Info("Waiting for seed to be deleted")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed %q to be deleted", obj.GetName())
		}

		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, fmt.Sprintf("Waiting for seed %q to be deleted", obj.GetName())), false, false, nil
	}

	// Delete gardenlet from the target cluster if it still exists and is not already deleting
	deployment, err := a.getGardenletDeployment(ctx, targetClient)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not get gardenlet deployment from target cluster: %w", err)
	}

	if deployment != nil {
		if deployment.DeletionTimestamp == nil {
			log.Info("Deleting gardenlet from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting gardenlet from target cluster")
			if err := a.deleteGardenlet(ctx, log, obj, targetClient, gardenletDeployment, seed, componentConfig, bootstrap, mergeWithParent); err != nil {
				a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
				return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could delete gardenlet from target cluster: %w", err)
			}
		} else {
			log.Info("Waiting for gardenlet to be deleted from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for gardenlet to be deleted from  target cluster")
		}

		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, "Waiting for gardenlet to be deleted from  target cluster"), true, false, nil
	}

	// Delete seed backup secrets if any of them still exists and is not already deleting
	backupSecret, err := a.getBackupSecret(ctx, &seedTemplate.Spec, obj)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not get seed %s secrets: %w", obj.GetName(), err)
	}

	if backupSecret != nil {
		if backupSecret.DeletionTimestamp == nil {
			log.Info("Deleting seed secrets")
			a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting seed secrets")
			if err := a.deleteBackupSecret(ctx, &seedTemplate.Spec, obj); err != nil {
				a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
				return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not delete seed %s secrets: %w", obj.GetName(), err)
			}
		} else {
			log.Info("Waiting for seed secrets to be deleted")
			a.Recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for seed secrets to be deleted")
		}

		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, "Waiting for seed secrets to be deleted"), true, false, nil
	}

	// Delete garden namespace from the target cluster if it still exists and is not already deleting
	gardenNamespace, err := a.getGardenNamespace(ctx, targetClient)
	if err != nil {
		a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not check if garden namespace exists in target cluster: %w", err)
	}

	if gardenNamespace != nil {
		if gardenNamespace.DeletionTimestamp == nil {
			log.Info("Deleting garden namespace from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting garden namespace from target cluster")
			if err := a.deleteGardenNamespace(ctx, targetClient); err != nil {
				a.Recorder.Eventf(obj, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, err.Error())
				return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error()), false, false, fmt.Errorf("could not delete garden namespace from target cluster: %w", err)
			}
		} else {
			log.Info("Waiting for garden namespace to be deleted from target cluster")
			a.Recorder.Eventf(obj, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Waiting for garden namespace to be deleted from target cluster")
		}

		return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, "Waiting for garden namespace to be deleted from target cluster"), true, false, nil
	}

	return updateCondition(a.Clock, conditions, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleted, fmt.Sprintf("Seed %s has been unregistered", obj.GetName())), false, true, nil
}

func (a *Actuator) ensureGardenNamespace(ctx context.Context, targetClient client.Client) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.GardenNamespaceTarget,
		},
	}
	if err := targetClient.Get(ctx, client.ObjectKeyFromObject(gardenNamespace), gardenNamespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return targetClient.Create(ctx, gardenNamespace)
	}
	return nil
}

func (a *Actuator) deleteGardenNamespace(ctx context.Context, targetClient kubernetes.Interface) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.GardenNamespaceTarget,
		},
	}
	return client.IgnoreNotFound(targetClient.Client().Delete(ctx, gardenNamespace))
}

func (a *Actuator) getGardenNamespace(ctx context.Context, targetClient kubernetes.Interface) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	if err := targetClient.Client().Get(ctx, client.ObjectKey{Name: a.GardenNamespaceTarget}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ns, nil
}

func (a *Actuator) deleteSeed(ctx context.Context, obj client.Object) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: obj.GetName(),
		},
	}
	return client.IgnoreNotFound(a.GardenClient.Delete(ctx, seed))
}

// GetSeed returns the seed with the given name if found. If not, nil is returned.
func GetSeed(ctx context.Context, gardenClient client.Client, seedName string) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: seedName}, seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return seed, nil
}

func (a *Actuator) deployGardenlet(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	targetClient kubernetes.Interface,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	seed *gardencorev1beta1.Seed,
	componentConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		obj,
		targetClient.Client(),
		gardenletDeployment,
		seed,
		componentConfig,
		bootstrap,
		mergeWithParent,
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	if err := a.ApplyGardenletChart(ctx, targetClient.ChartApplier(), values); err != nil {
		return err
	}

	// remove renew-kubeconfig annotation, if it exists
	if obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotations := obj.GetAnnotations()
		delete(annotations, v1beta1constants.GardenerOperation)
		obj.SetAnnotations(annotations)
		return a.GardenClient.Patch(ctx, obj, patch)
	}

	return nil
}

func (a *Actuator) deleteGardenlet(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	targetClient kubernetes.Interface,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	seed *gardencorev1beta1.Seed,
	componentConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		log,
		obj,
		targetClient.Client(),
		gardenletDeployment,
		seed,
		componentConfig,
		bootstrap,
		mergeWithParent,
	)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	return a.DeleteGardenletChart(ctx, targetClient.ChartApplier(), values)
}

func (a *Actuator) getGardenletDeployment(ctx context.Context, targetClient kubernetes.Interface) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := targetClient.Client().Get(ctx, client.ObjectKey{Namespace: a.GardenNamespaceTarget, Name: v1beta1constants.DeploymentNameGardenlet}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (a *Actuator) checkSeedSpec(ctx context.Context, spec *gardencorev1beta1.SeedSpec) error {
	// If VPA is enabled, check if the runtime namespace in the runtime cluster contains a vpa-admission-controller
	// deployment.
	if v1beta1helper.SeedSettingVerticalPodAutoscalerEnabled(spec.Settings) {
		runtimeVPAAdmissionControllerExists, err := a.CheckIfVPAAlreadyExists(ctx)
		if err != nil {
			return err
		}
		if runtimeVPAAdmissionControllerExists {
			return fmt.Errorf("runtime VPA is enabled but target cluster already has a VPA")
		}
	}

	return nil
}

func (a *Actuator) reconcileSeedSecrets(ctx context.Context, obj client.Object, spec *gardencorev1beta1.SeedSpec, gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment) error {
	// Get infrastructure secret
	infrastructureSecret, err := a.GetInfrastructureSecret(ctx)
	if err != nil {
		return err
	}

	// If backup is specified, create or update the backup secret if it doesn't exist or is owned by the object
	if spec.Backup != nil {
		var checksum string

		// Get backup secret
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if err == nil {
			checksum = utils.ComputeSecretChecksum(backupSecret.Data)[:8]
		} else if client.IgnoreNotFound(err) != nil {
			return err
		}

		// Create or update backup secret if it doesn't exist or is owned by the object
		if apierrors.IsNotFound(err) || metav1.IsControlledBy(backupSecret, obj) {
			gvk, err := apiutil.GVKForObject(obj, a.GardenClient.Scheme())
			if err != nil {
				return fmt.Errorf("could not get GroupVersionKind from object %v: %w", obj, err)
			}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: spec.Backup.SecretRef.Namespace, Name: spec.Backup.SecretRef.Name},
			}

			if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.GardenClient, secret, func() error {
				secret.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(obj, gvk),
				}
				secret.Type = corev1.SecretTypeOpaque
				secret.Data = infrastructureSecret.Data
				return nil
			}); err != nil {
				return err
			}

			checksum = utils.ComputeSecretChecksum(secret.Data)[:8]
		}

		// Inject backup-secret hash into the pod annotations
		if gardenletDeployment == nil {
			gardenletDeployment = &seedmanagementv1alpha1.GardenletDeployment{}
		}
		gardenletDeployment.PodAnnotations = utils.MergeStringMaps[string](gardenletDeployment.PodAnnotations, map[string]string{
			"checksum/seed-backup-secret": spec.Backup.SecretRef.Name + "-" + checksum,
		})
	}

	return nil
}

func (a *Actuator) deleteBackupSecret(ctx context.Context, spec *gardencorev1beta1.SeedSpec, obj client.Object) error {
	// If backup is specified, delete the backup secret if it exists and is owned by the object
	if spec.Backup != nil {
		backupSecret, err := kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil && metav1.IsControlledBy(backupSecret, obj) {
			if err := kubernetesutils.DeleteSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *Actuator) getBackupSecret(ctx context.Context, spec *gardencorev1beta1.SeedSpec, obj client.Object) (*corev1.Secret, error) {
	var (
		backupSecret *corev1.Secret
		err          error
	)

	// If backup is specified, get the backup secret if it exists and is owned by the object
	if spec.Backup != nil {
		backupSecret, err = kubernetesutils.GetSecretByReference(ctx, a.GardenClient, &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		if backupSecret != nil && !metav1.IsControlledBy(backupSecret, obj) {
			backupSecret = nil
		}
	}

	return backupSecret, nil
}

func (a *Actuator) prepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	obj client.Object,
	targetClient client.Client,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	seed *gardencorev1beta1.Seed,
	componentConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
) (
	map[string]any,
	error,
) {
	// Merge gardenlet deployment with parent values
	deployment, err := a.ValuesHelper.MergeGardenletDeployment(gardenletDeployment)
	if err != nil {
		return nil, err
	}

	// Merge gardenlet configuration with parent if specified
	if mergeWithParent {
		componentConfig, err = a.ValuesHelper.MergeGardenletConfiguration(componentConfig)
		if err != nil {
			return nil, err
		}
	}

	// Get gardenlet chart values
	return PrepareGardenletChartValues(
		ctx,
		log,
		a.GardenClient,
		a.GardenConfig,
		targetClient,
		a.Recorder,
		obj,
		seed,
		a.ValuesHelper,
		bootstrap,
		ensureGardenletEnvironment(deployment, a.GetTargetDomain()),
		componentConfig,
		a.GardenNamespaceTarget,
	)
}

// PrepareGardenletChartValues prepares the gardenlet chart values based on the config.
func PrepareGardenletChartValues(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	gardenRESTConfig *rest.Config,
	targetClusterClient client.Client,
	recorder record.EventRecorder,
	obj client.Object,
	seed *gardencorev1beta1.Seed,
	vp ValuesHelper,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	gardenletDeployment *seedmanagementv1alpha1.GardenletDeployment,
	gardenletConfig *gardenletconfigv1alpha1.GardenletConfiguration,
	gardenNamespaceTargetCluster string,
) (
	map[string]any,
	error,
) {
	// Ensure garden client connection is set
	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{}
	}

	// Prepare garden client connection
	var (
		bootstrapKubeconfig string
		err                 error
	)

	if bootstrap == seedmanagementv1alpha1.BootstrapNone {
		removeBootstrapConfigFromGardenClientConnection(gardenletConfig.GardenClientConnection)
	} else {
		bootstrapKubeconfig, err = prepareGardenClientConnectionWithBootstrap(
			ctx,
			log,
			gardenClient,
			gardenRESTConfig,
			targetClusterClient,
			recorder,
			obj,
			gardenletConfig.GardenClientConnection,
			seed,
			bootstrap,
			gardenNamespaceTargetCluster,
		)
		if err != nil {
			return nil, err
		}
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &gardenletconfigv1alpha1.SeedConfig{}
	}

	// Set the seed name
	gardenletConfig.SeedConfig.SeedTemplate.Name = obj.GetName()

	// Get gardenlet chart values
	return vp.GetGardenletChartValues(
		gardenletDeployment,
		gardenletConfig,
		bootstrapKubeconfig,
	)
}

// ensureGardenletEnvironment sets the KUBERNETES_SERVICE_HOST to the provided domain.
// This may be needed so that the deployed gardenlet can properly set the network policies allowing access of control
// plane components of the hosted shoots to the API server of the seed.
func ensureGardenletEnvironment(deployment *seedmanagementv1alpha1.GardenletDeployment, domain string) *seedmanagementv1alpha1.GardenletDeployment {
	const kubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	var serviceHost = ""

	if deployment.Env == nil {
		deployment.Env = []corev1.EnvVar{}
	}

	for _, env := range deployment.Env {
		if env.Name == kubernetesServiceHost {
			return deployment
		}
	}

	if len(domain) != 0 {
		serviceHost = gardenerutils.GetAPIServerDomain(domain)
	}

	if len(serviceHost) != 0 {
		deployment.Env = append(
			deployment.Env,
			corev1.EnvVar{
				Name:  kubernetesServiceHost,
				Value: serviceHost,
			},
		)
	}

	return deployment
}

func prepareGardenClientConnectionWithBootstrap(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	gardenRESTConfig *rest.Config,
	targetClusterClient client.Client,
	recorder record.EventRecorder,
	obj client.Object,
	gcc *gardenletconfigv1alpha1.GardenClientConnection,
	seed *gardencorev1beta1.Seed,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	gardenNamespaceTargetCluster string,
) (
	string,
	error,
) {
	// Ensure kubeconfig secret is set
	if gcc.KubeconfigSecret == nil {
		gcc.KubeconfigSecret = &corev1.SecretReference{
			Name:      GardenletDefaultKubeconfigSecretName,
			Namespace: gardenNamespaceTargetCluster,
		}
	}

	if seed != nil && seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Check if client certificate is expired. If yes then delete the existing kubeconfig secret to make sure that the
		// seed can be re-bootstrapped.
		if err := kubernetesutils.DeleteSecretByReference(ctx, targetClusterClient, gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else if obj.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRenewKubeconfig {
		// Also remove the kubeconfig if the renew-kubeconfig operation annotation is set on the resource.
		log.Info("Renewing gardenlet kubeconfig secret due to operation annotation")
		recorder.Event(obj, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Renewing gardenlet kubeconfig secret due to operation annotation")

		if err := kubernetesutils.DeleteSecretByReference(ctx, targetClusterClient, gcc.KubeconfigSecret); err != nil {
			return "", err
		}
	} else {
		seedIsAlreadyBootstrapped, err := isAlreadyBootstrapped(ctx, targetClusterClient, gcc.KubeconfigSecret)
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
			Namespace: gardenNamespaceTargetCluster,
		}
	}

	return createBootstrapKubeconfig(ctx, gardenClient, gardenRESTConfig, obj, bootstrap, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
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

func removeBootstrapConfigFromGardenClientConnection(gcc *gardenletconfigv1alpha1.GardenClientConnection) {
	// Ensure kubeconfig secret and bootstrap kubeconfig secret are not set
	gcc.KubeconfigSecret = nil
	gcc.BootstrapKubeconfig = nil
}

// createBootstrapKubeconfig creates a kubeconfig for the Garden cluster
// containing either the token of a service account or a bootstrap token
// returns the kubeconfig as a string
func createBootstrapKubeconfig(
	ctx context.Context,
	gardenClient client.Client,
	gardenRESTConfig *rest.Config,
	obj client.Object,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	address *string,
	caCert []byte,
) (
	string,
	error,
) {
	var (
		err                 error
		bootstrapKubeconfig []byte
	)

	gardenClientRestConfig := gardenerutils.PrepareGardenClientRestConfig(gardenRESTConfig, address, caCert)

	switch bootstrap {
	case seedmanagementv1alpha1.BootstrapServiceAccount:
		// Create a kubeconfig containing the token of a temporary service account as client credentials
		var (
			serviceAccountName      = gardenletbootstraputil.ServiceAccountName(obj.GetName())
			serviceAccountNamespace = obj.GetNamespace()
		)

		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, gardenClient, gardenClientRestConfig, serviceAccountName, serviceAccountNamespace)
		if err != nil {
			return "", err
		}

	case seedmanagementv1alpha1.BootstrapToken:
		var kind string
		switch obj.(type) {
		case *seedmanagementv1alpha1.ManagedSeed:
			kind = gardenletbootstraputil.KindManagedSeed
		case *seedmanagementv1alpha1.Gardenlet:
			kind = gardenletbootstraputil.KindGardenlet
		default:
			return "", fmt.Errorf("unknown bootstrap kind for object of type %T", obj)
		}

		var (
			tokenID          = gardenletbootstraputil.TokenID(metav1.ObjectMeta{Name: obj.GetName(), Namespace: obj.GetNamespace()})
			tokenDescription = gardenletbootstraputil.Description(kind, obj.GetNamespace(), obj.GetName())
			tokenValidity    = 24 * time.Hour
		)

		// Create a kubeconfig containing a valid bootstrap token as client credentials
		bootstrapKubeconfig, err = gardenletbootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, gardenClient, gardenClientRestConfig, tokenID, tokenDescription, tokenValidity)
		if err != nil {
			return "", err
		}
	}

	return string(bootstrapKubeconfig), nil
}

func updateCondition(clock clock.Clock, conditions []gardencorev1beta1.Condition, cs gardencorev1beta1.ConditionStatus, reason, message string) []gardencorev1beta1.Condition {
	condition := v1beta1helper.GetOrInitConditionWithClock(clock, conditions, seedmanagementv1alpha1.SeedRegistered)
	condition = v1beta1helper.UpdatedConditionWithClock(clock, condition, cs, reason, message)
	return v1beta1helper.MergeConditions(conditions, condition)
}
