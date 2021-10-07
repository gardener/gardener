// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	v1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Actuator acts upon ManagedSeed resources.
type Actuator interface {
	// Reconcile reconciles ManagedSeed creation or update.
	Reconcile(context.Context, *seedmanagementv1alpha1.ManagedSeed) (*seedmanagementv1alpha1.ManagedSeedStatus, bool, error)
	// Delete reconciles ManagedSeed deletion.
	Delete(context.Context, *seedmanagementv1alpha1.ManagedSeed) (*seedmanagementv1alpha1.ManagedSeedStatus, bool, bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenClient kubernetes.Interface
	clientMap    clientmap.ClientMap
	vp           ValuesHelper
	recorder     record.EventRecorder
	logger       *logrus.Logger
}

// newActuator creates a new Actuator with the given clients, ValuesHelper, and logger.
func newActuator(gardenClient kubernetes.Interface, clientMap clientmap.ClientMap, vp ValuesHelper, recorder record.EventRecorder, logger *logrus.Logger) Actuator {
	return &actuator{
		gardenClient: gardenClient,
		clientMap:    clientMap,
		vp:           vp,
		recorder:     recorder,
		logger:       logger,
	}
}

// Reconcile reconciles ManagedSeed creation or update.
func (a *actuator) Reconcile(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed) (status *seedmanagementv1alpha1.ManagedSeedStatus, wait bool, err error) {
	// Initialize status
	status = ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	defer func() {
		if err != nil {
			a.reconcileErrorEventf(ms, err.Error())
			updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := a.gardenClient.APIReader().Get(ctx, kutil.Key(ms.Namespace, ms.Spec.Shoot.Name), shoot); err != nil {
		return status, false, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	// Check if shoot is reconciled and update ShootReconciled condition
	if !shootReconciled(shoot) {
		a.reconcilingInfoEventf(ms, "Waiting for shoot %s to be reconciled", kutil.ObjectName(shoot))
		updateCondition(status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventReconciling, "")
		return status, true, nil
	}
	updateCondition(status, seedmanagementv1alpha1.ManagedSeedShootReconciled, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled, "")

	// Update SeedRegistered condition
	updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionProgressing, gardencorev1beta1.EventReconciling, "")

	// Get shoot client
	shootClient, err := a.clientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return status, false, fmt.Errorf("could not get shoot client for shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(ms)
	if err != nil {
		return status, false, err
	}

	// Check seed spec
	if err := a.checkSeedSpec(ctx, &seedTemplate.Spec, shoot); err != nil {
		return status, false, err
	}

	// Create or update garden namespace in the shoot
	a.reconcilingInfoEventf(ms, "Creating or updating garden namespace in shoot %s", kutil.ObjectName(shoot))
	if err := a.ensureGardenNamespace(ctx, shootClient.Client()); err != nil {
		return status, false, fmt.Errorf("could not create or update garden namespace in shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	// Create or update seed secrets
	a.reconcilingInfoEventf(ms, "Creating or updating seed %s secrets", ms.Name)
	if err := a.createOrUpdateSeedSecrets(ctx, &seedTemplate.Spec, ms, shoot); err != nil {
		return status, false, fmt.Errorf("could not create or update seed %s secrets: %w", ms.Name, err)
	}

	if ms.Spec.SeedTemplate != nil {
		// Create or update seed
		a.reconcilingInfoEventf(ms, "Creating or updating seed %s", ms.Name)
		if err := a.createOrUpdateSeed(ctx, ms); err != nil {
			return status, false, fmt.Errorf("could not create or update seed %s: %w", ms.Name, err)
		}
	} else if ms.Spec.Gardenlet != nil {
		seed, err := a.getSeed(ctx, ms)
		if err != nil {
			return status, false, fmt.Errorf("could not read seed %s: %w", ms.Name, err)
		}

		// Deploy gardenlet into the shoot, it will register the seed automatically
		a.reconcilingInfoEventf(ms, "Deploying gardenlet into shoot %s", kutil.ObjectName(shoot))
		if err := a.deployGardenlet(ctx, shootClient, ms, seed, gardenletConfig, shoot); err != nil {
			return status, false, fmt.Errorf("could not deploy gardenlet into shoot %s: %w", kutil.ObjectName(shoot), err)
		}
	}

	updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionTrue, gardencorev1beta1.EventReconciled, "")
	return status, false, nil
}

// Delete reconciles ManagedSeed deletion.
func (a *actuator) Delete(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed) (status *seedmanagementv1alpha1.ManagedSeedStatus, wait, removeFinalizer bool, err error) {
	// Initialize status
	status = ms.Status.DeepCopy()
	status.ObservedGeneration = ms.Generation

	defer func() {
		if err != nil {
			a.deleteErrorEventf(ms, err.Error())
			updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleteError, err.Error())
		}
	}()

	// Update SeedRegistered condition
	updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleting, "")

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := a.gardenClient.APIReader().Get(ctx, kutil.Key(ms.Namespace, ms.Spec.Shoot.Name), shoot); err != nil {
		return status, false, false, fmt.Errorf("could not get shoot %s/%s: %w", ms.Namespace, ms.Spec.Shoot.Name, err)
	}

	// Get shoot client
	shootClient, err := a.clientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return status, false, false, fmt.Errorf("could not get shoot client for shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	// Extract seed template and gardenlet config
	seedTemplate, gardenletConfig, err := helper.ExtractSeedTemplateAndGardenletConfig(ms)
	if err != nil {
		return status, false, false, err
	}

	// Delete seed if it still exists and is not already deleting
	seed, err := a.getSeed(ctx, ms)
	if err != nil {
		return status, false, false, fmt.Errorf("could not get seed %s: %w", ms.Name, err)
	}

	if seed != nil {
		if seed.DeletionTimestamp == nil {
			a.deletingInfoEventf(ms, "Deleting seed %s", ms.Name)
			if err := a.deleteSeed(ctx, ms); err != nil {
				return status, false, false, fmt.Errorf("could not delete seed %s: %w", ms.Name, err)
			}
		} else {
			a.deletingInfoEventf(ms, "Waiting for seed %s to be deleted", ms.Name)
		}
		return status, false, false, nil
	}

	if ms.Spec.Gardenlet != nil {
		// Delete gardenlet from the shoot if it still exists and is not already deleting
		gardenletDeployment, err := a.getGardenletDeployment(ctx, shootClient)
		if err != nil {
			return status, false, false, fmt.Errorf("could not get gardenlet deployment in shoot %s: %w", kutil.ObjectName(shoot), err)
		}
		if gardenletDeployment != nil {
			if gardenletDeployment.DeletionTimestamp == nil {
				a.deletingInfoEventf(ms, "Deleting gardenlet from shoot %s", kutil.ObjectName(shoot))
				if err := a.deleteGardenlet(ctx, shootClient, ms, seed, gardenletConfig, shoot); err != nil {
					return status, false, false, fmt.Errorf("could delete gardenlet from shoot %s: %w", kutil.ObjectName(shoot), err)
				}
			} else {
				a.deletingInfoEventf(ms, "Waiting for gardenlet to be deleted from shoot %s", kutil.ObjectName(shoot))
			}
			return status, true, false, nil
		}
	}

	// Delete seed secrets if any of them still exists and is not already deleting
	secret, backupSecret, err := a.getSeedSecrets(ctx, &seedTemplate.Spec, ms)
	if err != nil {
		return status, false, false, fmt.Errorf("could not get seed %s secrets: %w", ms.Name, err)
	}
	if secret != nil || backupSecret != nil {
		if (secret != nil && secret.DeletionTimestamp == nil) || (backupSecret != nil && backupSecret.DeletionTimestamp == nil) {
			a.deletingInfoEventf(ms, "Deleting seed %s secrets", ms.Name)
			if err := a.deleteSeedSecrets(ctx, &seedTemplate.Spec, ms); err != nil {
				return status, false, false, fmt.Errorf("could not delete seed %s secrets: %w", ms.Name, err)
			}
		} else {
			a.deletingInfoEventf(ms, "Waiting for seed %s secrets to be deleted", ms.Name)
		}
		return status, true, false, nil
	}

	// Delete garden namespace from the shoot if it still exists and is not already deleting
	gardenNamespace, err := a.getGardenNamespace(ctx, shootClient)
	if err != nil {
		return status, false, false, fmt.Errorf("could not check if garden namespace exists in shoot %s: %w", kutil.ObjectName(shoot), err)
	}
	if gardenNamespace != nil {
		if gardenNamespace.DeletionTimestamp == nil {
			a.deletingInfoEventf(ms, "Deleting garden namespace from shoot %s", kutil.ObjectName(shoot))
			if err := a.deleteGardenNamespace(ctx, shootClient); err != nil {
				return status, false, false, fmt.Errorf("could not delete garden namespace from shoot %s: %w", kutil.ObjectName(shoot), err)
			}
		} else {
			a.deletingInfoEventf(ms, "Waiting for garden namespace to be deleted from shoot %s", kutil.ObjectName(shoot))
		}
		return status, true, false, nil
	}

	updateCondition(status, seedmanagementv1alpha1.ManagedSeedSeedRegistered, gardencorev1beta1.ConditionFalse, gardencorev1beta1.EventDeleted, "")
	return status, false, true, nil
}

func (a *actuator) ensureGardenNamespace(ctx context.Context, shootClient client.Client) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.GardenNamespace,
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
			Name: v1beta1constants.GardenNamespace,
		},
	}
	return client.IgnoreNotFound(shootClient.Client().Delete(ctx, gardenNamespace))
}

func (a *actuator) getGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	if err := shootClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ns, nil
}

func (a *actuator) createOrUpdateSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	_, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.gardenClient.Client(), seed, func() error {
		seed.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
		}
		seed.Labels = utils.MergeStringMaps(managedSeed.Spec.SeedTemplate.Labels, map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		})
		seed.Annotations = managedSeed.Spec.SeedTemplate.Annotations
		seed.Spec = managedSeed.Spec.SeedTemplate.Spec
		return nil
	})
	return err
}

func (a *actuator) deleteSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	return client.IgnoreNotFound(a.gardenClient.Client().Delete(ctx, seed))
}

func (a *actuator) getSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) (*gardencorev1beta1.Seed, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(managedSeed.Name), seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return seed, nil
}

func (a *actuator) deployGardenlet(
	ctx context.Context,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *configv1alpha1.GardenletConfiguration,
	shoot *gardencorev1beta1.Shoot,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		shootClient,
		managedSeed,
		seed,
		gardenletConfig,
		v1alpha1helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		utils.IsTrue(managedSeed.Spec.Gardenlet.MergeWithParent),
		shoot,
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	return shootClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
}

func (a *actuator) deleteGardenlet(
	ctx context.Context,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *configv1alpha1.GardenletConfiguration,
	shoot *gardencorev1beta1.Shoot,
) error {
	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		shootClient,
		managedSeed,
		seed,
		gardenletConfig,
		v1alpha1helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		utils.IsTrue(managedSeed.Spec.Gardenlet.MergeWithParent),
		shoot,
	)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	return shootClient.ChartApplier().Delete(ctx, filepath.Join(charts.Path, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
}

func (a *actuator) getGardenletDeployment(ctx context.Context, shootClient kubernetes.Interface) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := shootClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, v1beta1constants.DeploymentNameGardenlet), deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

func (a *actuator) checkSeedSpec(ctx context.Context, spec *gardencorev1beta1.SeedSpec, shoot *gardencorev1beta1.Shoot) error {
	// Get seed client
	seedClient, err := a.clientMap.GetClient(ctx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		return fmt.Errorf("could not get seed client for seed %s: %w", *shoot.Spec.SeedName, err)
	}

	// If VPA is enabled, check if the shoot namespace in the seed contains a vpa-admission-controller deployment
	if gardencorev1beta1helper.SeedSettingVerticalPodAutoscalerEnabled(spec.Settings) {
		seedVPAAdmissionControllerExists, err := a.seedVPADeploymentExists(ctx, seedClient, shoot)
		if err != nil {
			return err
		}
		if seedVPAAdmissionControllerExists {
			return fmt.Errorf("seed VPA is enabled but shoot already has a VPA")
		}
	}

	// If ingress is specified, check if the shoot namespace in the seed contains an ingress DNSEntry
	if spec.Ingress != nil {
		seedNginxDNSEntryExists, err := a.seedIngressDNSEntryExists(ctx, seedClient, shoot)
		if err != nil {
			return err
		}
		if seedNginxDNSEntryExists {
			return fmt.Errorf("seed ingress controller is enabled but an ingress DNS entry still exists")
		}
	}

	return nil
}

func (a *actuator) createOrUpdateSeedSecrets(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Get shoot secret
	shootSecret, err := a.getShootSecret(ctx, shoot)
	if err != nil {
		return err
	}

	// If backup is specified, create or update the backup secret if it doesn't exist or is owned by the managed seed
	if spec.Backup != nil {
		// Get backup secret
		backupSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		// Create or update backup secret if it doesn't exist or is owned by the managed seed
		if apierrors.IsNotFound(err) || metav1.IsControlledBy(backupSecret, managedSeed) {
			secret := &corev1.Secret{
				ObjectMeta: kutil.ObjectMeta(spec.Backup.SecretRef.Namespace, spec.Backup.SecretRef.Name),
			}
			if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.gardenClient.Client(), secret, func() error {
				secret.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
				}
				secret.Type = corev1.SecretTypeOpaque
				secret.Data = shootSecret.Data
				return nil
			}); err != nil {
				return err
			}
		}
	}

	// If secret reference is specified, create or update the corresponding secret
	if spec.SecretRef != nil {
		// Get shoot kubeconfig secret
		shootKubeconfigSecret, err := a.getShootKubeconfigSecret(ctx, shoot)
		if err != nil {
			return err
		}

		// Initialize seed secret data
		data := shootSecret.Data
		data[kubernetes.KubeConfig] = shootKubeconfigSecret.Data[kubernetes.KubeConfig]

		// Create or update seed secret
		secret := &corev1.Secret{
			ObjectMeta: kutil.ObjectMeta(spec.SecretRef.Namespace, spec.SecretRef.Name),
		}
		if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.gardenClient.Client(), secret, func() error {
			secret.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
			}
			secret.Type = corev1.SecretTypeOpaque
			secret.Data = data
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) deleteSeedSecrets(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	// If backup is specified, delete the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil && metav1.IsControlledBy(backupSecret, managedSeed) {
			if err := kutil.DeleteSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef); err != nil {
				return err
			}
		}
	}

	// If secret reference is specified, delete the corresponding secret
	if spec.SecretRef != nil {
		if err := kutil.DeleteSecretByReference(ctx, a.gardenClient.Client(), spec.SecretRef); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) getSeedSecrets(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed) (*corev1.Secret, *corev1.Secret, error) {
	var secret, backupSecret *corev1.Secret
	var err error

	// If backup is specified, get the backup secret if it exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err = kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, nil, err
		}
		if backupSecret != nil && !metav1.IsControlledBy(backupSecret, managedSeed) {
			backupSecret = nil
		}
	}

	// If secret reference is specified, get the corresponding secret if it exists
	if spec.SecretRef != nil {
		secret, err = kutil.GetSecretByReference(ctx, a.gardenClient.Client(), spec.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return nil, nil, err
		}
	}

	return secret, backupSecret, nil
}

func (a *actuator) getShootSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootSecretBinding := &gardencorev1beta1.SecretBinding{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), shootSecretBinding); err != nil {
		return nil, err
	}
	return kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &shootSecretBinding.SecretRef)
}

func (a *actuator) getShootKubeconfigSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootKubeconfigSecret := &corev1.Secret{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, gutil.ComputeShootProjectSecretName(shoot.Name, gutil.ShootProjectSecretSuffixKubeconfig)), shootKubeconfigSecret); err != nil {
		return nil, err
	}
	return shootKubeconfigSecret, nil
}

func (a *actuator) seedVPADeploymentExists(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := seedClient.Client().Get(ctx, kutil.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *actuator) seedIngressDNSEntryExists(ctx context.Context, seedClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := seedClient.Client().Get(ctx, kutil.Key(shoot.Status.TechnicalID, common.ShootDNSIngressName), &dnsv1alpha1.DNSEntry{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *actuator) prepareGardenletChartValues(
	ctx context.Context,
	shootClient kubernetes.Interface,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	seed *gardencorev1beta1.Seed,
	gardenletConfig *configv1alpha1.GardenletConfiguration,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
	shoot *gardencorev1beta1.Shoot,
) (map[string]interface{}, error) {
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
		gardenletConfig.GardenClientConnection = &configv1alpha1.GardenClientConnection{}
	}

	// Prepare garden client connection
	var bootstrapKubeconfig string
	if bootstrap == seedmanagementv1alpha1.BootstrapNone {
		a.removeBootstrapConfigFromGardenClientConnection(gardenletConfig.GardenClientConnection)
	} else {
		bootstrapKubeconfig, err = a.prepareGardenClientConnectionWithBootstrap(ctx, shootClient, gardenletConfig.GardenClientConnection, managedSeed, seed, bootstrap)
		if err != nil {
			return nil, err
		}
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &configv1alpha1.SeedConfig{}
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
		shootDomain = gutil.GetAPIServerDomain(*dns.Domain)
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
	shootClient kubernetes.Interface,
	gcc *configv1alpha1.GardenClientConnection,
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
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	if seed != nil && seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(time.Now().UTC()) {
		// Check if client certificate is expired. If yes then delete the existing kubeconfig secret to make sure that the
		// seed can be re-bootstrapped.
		if err := kutil.DeleteSecretByReference(ctx, shootClient.Client(), gcc.KubeconfigSecret); err != nil {
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
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	return a.createBootstrapKubeconfig(ctx, managedSeed.ObjectMeta, bootstrap, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
}

// isAlreadyBootstrapped checks if the gardenlet already has a valid Garden cluster certificate through TLS bootstrapping
// by checking if the specified secret reference already exists
func isAlreadyBootstrapped(ctx context.Context, c client.Client, s *corev1.SecretReference) (bool, error) {
	// If kubeconfig secret exists, return an empty result, since the bootstrap can be skipped
	secret, err := kutil.GetSecretByReference(ctx, c, s)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	return secret != nil, nil
}

func (a *actuator) removeBootstrapConfigFromGardenClientConnection(gcc *configv1alpha1.GardenClientConnection) {
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

	gardenClientRestConfig := a.prepareGardenClientRestConfig(address, caCert)

	switch bootstrap {
	case seedmanagementv1alpha1.BootstrapServiceAccount:
		// Create a kubeconfig containing the token of a temporary service account as client credentials
		var (
			serviceAccountName      = bootstraputil.ServiceAccountName(objectMeta.Name)
			serviceAccountNamespace = objectMeta.Namespace
		)

		// Create a kubeconfig containing a valid service account token as client credentials
		bootstrapKubeconfig, err = bootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, a.gardenClient.Client(), &gardenClientRestConfig, serviceAccountName, serviceAccountNamespace)
		if err != nil {
			return "", err
		}

	case seedmanagementv1alpha1.BootstrapToken:
		var (
			tokenID          = bootstraputil.TokenID(objectMeta)
			tokenDescription = bootstraputil.Description(bootstraputil.KindManagedSeed, objectMeta.Namespace, objectMeta.Name)
			tokenValidity    = 24 * time.Hour
		)

		// Create a kubeconfig containing a valid bootstrap token as client credentials
		bootstrapKubeconfig, err = bootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, a.gardenClient.Client(), &gardenClientRestConfig, tokenID, tokenDescription, tokenValidity)
		if err != nil {
			return "", err
		}
	}

	return string(bootstrapKubeconfig), nil
}

// prepareGardenClientRestConfig adds an optional host and CA certificate to the garden client rest config
func (a *actuator) prepareGardenClientRestConfig(address *string, caCert []byte) rest.Config {
	gardenClientRestConfig := *a.gardenClient.RESTConfig()
	if address != nil {
		gardenClientRestConfig.Host = *address
	}
	if caCert != nil {
		gardenClientRestConfig.TLSClientConfig = rest.TLSClientConfig{
			CAData: caCert,
		}
	}
	return gardenClientRestConfig
}

func (a *actuator) reconcilingInfoEventf(ms *seedmanagementv1alpha1.ManagedSeed, fmt string, args ...interface{}) {
	a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, fmt, args...)
	a.getLogger(ms).Infof(fmt, args...)
}

func (a *actuator) deletingInfoEventf(ms *seedmanagementv1alpha1.ManagedSeed, fmt string, args ...interface{}) {
	a.recorder.Eventf(ms, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, fmt, args...)
	a.getLogger(ms).Infof(fmt, args...)
}

func (a *actuator) reconcileErrorEventf(ms *seedmanagementv1alpha1.ManagedSeed, fmt string, args ...interface{}) {
	a.recorder.Eventf(ms, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, fmt, args...)
	a.getLogger(ms).Errorf(fmt, args...)
}

func (a *actuator) deleteErrorEventf(ms *seedmanagementv1alpha1.ManagedSeed, fmt string, args ...interface{}) {
	a.recorder.Eventf(ms, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, fmt, args...)
	a.getLogger(ms).Errorf(fmt, args...)
}

func (a *actuator) getLogger(ms *seedmanagementv1alpha1.ManagedSeed) *logrus.Entry {
	return logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(ms))
}

func shootReconciled(shoot *gardencorev1beta1.Shoot) bool {
	lastOp := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration && lastOp != nil && lastOp.State == gardencorev1beta1.LastOperationStateSucceeded
}

func updateCondition(status *seedmanagementv1alpha1.ManagedSeedStatus, ct gardencorev1beta1.ConditionType, cs gardencorev1beta1.ConditionStatus, reason, message string) {
	condition := gardencorev1beta1helper.GetOrInitCondition(status.Conditions, ct)
	condition = gardencorev1beta1helper.UpdatedCondition(condition, cs, reason, message)
	status.Conditions = gardencorev1beta1helper.MergeConditions(status.Conditions, condition)
}
