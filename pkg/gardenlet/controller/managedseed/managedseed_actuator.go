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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	v1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Actuator acts upon ManagedSeed resources.
type Actuator interface {
	// Reconcile reconciles ManagedSeed creation or update.
	Reconcile(context.Context, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Shoot) error
	// Delete reconciles ManagedSeed deletion.
	Delete(context.Context, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Shoot) error
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenClient kubernetes.Interface
	clientMap    clientmap.ClientMap
	vp           ValuesHelper
	logger       *logrus.Logger
}

// newActuator creates a new Actuator with the given clients, ValuesHelper, and logger.
func newActuator(gardenClient kubernetes.Interface, clientMap clientmap.ClientMap, vp ValuesHelper, logger *logrus.Logger) Actuator {
	return &actuator{
		gardenClient: gardenClient,
		clientMap:    clientMap,
		vp:           vp,
		logger:       logger,
	}
}

// Reconcile reconciles ManagedSeed creation or update.
func (a *actuator) Reconcile(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Get shoot client
	shootClient, err := a.clientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return fmt.Errorf("could not get shoot client for shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	// Create or update garden namespace in the shoot
	managedSeedLogger.Infof("Creating or updating garden namespace in shoot %s", kutil.ObjectName(shoot))
	if err := a.createOrUpdateGardenNamespace(ctx, shootClient); err != nil {
		return fmt.Errorf("could not create or update garden namespace in shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	switch {
	case managedSeed.Spec.SeedTemplate != nil:
		// Register the shoot as seed
		managedSeedLogger.Infof("Registering shoot %s as seed", kutil.ObjectName(shoot))
		if err := a.registerAsSeed(ctx, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not register shoot %s as seed: %w", kutil.ObjectName(shoot), err)
		}

	case managedSeed.Spec.Gardenlet != nil:
		// Deploy gardenlet into the shoot, it will register the seed automatically
		managedSeedLogger.Infof("Deploying gardenlet into shoot %s", kutil.ObjectName(shoot))
		if err := a.deployGardenlet(ctx, shootClient, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not deploy gardenlet into shoot %s: %w", kutil.ObjectName(shoot), err)
		}
	}

	return nil
}

// Delete reconciles ManagedSeed deletion.
func (a *actuator) Delete(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Get shoot client
	shootClient, err := a.clientMap.GetClient(ctx, keys.ForShoot(shoot))
	if err != nil {
		return fmt.Errorf("could not get shoot client for shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	switch {
	case managedSeed.Spec.SeedTemplate != nil:
		// Unregister the shoot as seed
		managedSeedLogger.Infof("Unregistering shoot %s as seed", kutil.ObjectName(shoot))
		if err := a.unregisterAsSeed(ctx, managedSeed); err != nil {
			return fmt.Errorf("could not unregister shoot %s as seed: %w", kutil.ObjectName(shoot), err)
		}

	case managedSeed.Spec.Gardenlet != nil:
		// Ensure the seed is deleted
		managedSeedLogger.Infof("Ensuring seed %s is deleted", managedSeed.Name)
		if err := a.ensureSeedDeleted(ctx, managedSeed); err != nil {
			return fmt.Errorf("could not ensure seed %s is deleted: %w", managedSeed.Name, err)
		}

		// Delete gardenlet from the shoot
		managedSeedLogger.Infof("Deleting gardenlet from shoot %s", kutil.ObjectName(shoot))
		if err := a.deleteGardenlet(ctx, shootClient, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not delete gardenlet from shoot %s: %w", kutil.ObjectName(shoot), err)
		}
	}

	// Ensure garden namespace is deleted from the shoot
	managedSeedLogger.Infof("Ensuring garden namespace is deleted from shoot %s", kutil.ObjectName(shoot))
	if err := a.ensureGardenNamespaceDeleted(ctx, shootClient); err != nil {
		return fmt.Errorf("could not ensure garden namespace is deleted from shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	return nil
}

func (a *actuator) createOrUpdateGardenNamespace(ctx context.Context, shootClient kubernetes.Interface) error {
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.GardenNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, shootClient.Client(), gardenNamespace, func() error {
		return nil
	})
	return err
}

func (a *actuator) ensureGardenNamespaceDeleted(ctx context.Context, shootClient kubernetes.Interface) error {
	// Delete garden namespace
	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.GardenNamespace,
		},
	}
	if err := shootClient.Client().Delete(ctx, gardenNamespace); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Check if garden namespace still exists
	err := shootClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), &corev1.Namespace{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if err == nil {
		return fmt.Errorf("namespace %s still exists", v1beta1constants.GardenNamespace)
	}

	return nil
}

func (a *actuator) registerAsSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Check seed spec
	if err := a.checkSeedSpec(ctx, &managedSeed.Spec.SeedTemplate.Spec, shoot); err != nil {
		return err
	}

	// Create or update seed secrets
	if err := a.createOrUpdateSeedSecrets(ctx, &managedSeed.Spec.SeedTemplate.Spec, managedSeed, shoot); err != nil {
		return err
	}

	// Create or update the seed
	return a.createOrUpdateSeed(ctx, managedSeed)
}

func (a *actuator) unregisterAsSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	// Ensure the seed is deleted
	if err := a.ensureSeedDeleted(ctx, managedSeed); err != nil {
		return err
	}

	// Ensure seed secrets are deleted
	return a.ensureSeedSecretsDeleted(ctx, &managedSeed.Spec.SeedTemplate.Spec, managedSeed)
}

func (a *actuator) createOrUpdateSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), seed, func() error {
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

func (a *actuator) ensureSeedDeleted(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	// Delete the seed
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	if err := a.gardenClient.Client().Delete(ctx, seed); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Check if the seed still exists
	err := a.gardenClient.Client().Get(ctx, kutil.Key(managedSeed.Name), &gardencorev1beta1.Seed{})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if err == nil {
		return fmt.Errorf("seed %s still exists", managedSeed.Name)
	}

	return nil
}

func (a *actuator) deployGardenlet(ctx context.Context, shootClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Decode gardenlet configuration
	gardenletConfig, err := helper.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
	if err != nil {
		return err
	}

	// Check seed spec
	if err := a.checkSeedSpec(ctx, &gardenletConfig.SeedConfig.SeedTemplate.Spec, shoot); err != nil {
		return err
	}

	// Create or update seed secrets
	if err := a.createOrUpdateSeedSecrets(ctx, &gardenletConfig.SeedConfig.SeedTemplate.Spec, managedSeed, shoot); err != nil {
		return err
	}

	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		shootClient,
		managedSeed.Spec.Gardenlet.Deployment,
		gardenletConfig,
		managedSeed.Name,
		v1alpha1helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		utils.IsTrue(managedSeed.Spec.Gardenlet.MergeWithParent),
		shoot,
	)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	return shootClient.ChartApplier().Apply(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
}

func (a *actuator) deleteGardenlet(ctx context.Context, shootClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Decode gardenlet configuration
	gardenletConfig, err := helper.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
	if err != nil {
		return err
	}

	// Ensure seed secrets are deleted
	if err := a.ensureSeedSecretsDeleted(ctx, &gardenletConfig.SeedConfig.SeedTemplate.Spec, managedSeed); err != nil {
		return err
	}

	// Prepare gardenlet chart values
	values, err := a.prepareGardenletChartValues(
		ctx,
		shootClient,
		managedSeed.Spec.Gardenlet.Deployment,
		gardenletConfig,
		managedSeed.Name,
		v1alpha1helper.GetBootstrap(managedSeed.Spec.Gardenlet.Bootstrap),
		utils.IsTrue(managedSeed.Spec.Gardenlet.MergeWithParent), shoot,
	)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	return shootClient.ChartApplier().Delete(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
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
			ownerRefs := []metav1.OwnerReference{
				*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
			}
			if err := kutil.CreateOrUpdateSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef, corev1.SecretTypeOpaque, shootSecret.Data, ownerRefs); err != nil {
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
		ownerRefs := []metav1.OwnerReference{
			*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
		}
		if err := kutil.CreateOrUpdateSecretByReference(ctx, a.gardenClient.Client(), spec.SecretRef, corev1.SecretTypeOpaque, data, ownerRefs); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) ensureSeedSecretsDeleted(ctx context.Context, spec *gardencorev1beta1.SeedSpec, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
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

	// If backup is specified, check if the backup secret still exists and is owned by the managed seed
	if spec.Backup != nil {
		backupSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &spec.Backup.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil && metav1.IsControlledBy(backupSecret, managedSeed) {
			return fmt.Errorf("backup secret %s still exists", kutil.ObjectName(backupSecret))
		}
	}

	// If secret reference is specified, check if the corresponding secret still exists
	if spec.SecretRef != nil {
		secret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), spec.SecretRef)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil {
			return fmt.Errorf("seed secret %s still exists", kutil.ObjectName(secret))
		}
	}

	return nil
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
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, fmt.Sprintf("%s.kubeconfig", shoot.Name)), shootKubeconfigSecret); err != nil {
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
	deployment *seedmanagementv1alpha1.GardenletDeployment,
	gardenletConfig *configv1alpha1.GardenletConfiguration,
	name string,
	bootstrap seedmanagementv1alpha1.Bootstrap,
	mergeWithParent bool,
	shoot *gardencorev1beta1.Shoot,
) (map[string]interface{}, error) {
	var err error

	// Merge gardenlet deployment with parent values
	deployment, err = a.vp.MergeGardenletDeployment(deployment, shoot)
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
	if bootstrap != seedmanagementv1alpha1.BootstrapNone {
		bootstrapKubeconfig, err = a.prepareGardenClientConnectionWithBootstrap(ctx, shootClient, gardenletConfig.GardenClientConnection, name, bootstrap)
		if err != nil {
			return nil, err
		}
	} else {
		a.prepareGardenClientConnectionWithoutBootstrap(gardenletConfig.GardenClientConnection)
	}

	// Ensure seed config is set
	if gardenletConfig.SeedConfig == nil {
		gardenletConfig.SeedConfig = &configv1alpha1.SeedConfig{}
	}

	// Set the seed name
	gardenletConfig.SeedConfig.SeedTemplate.Name = name

	// Ensure seed selector is not set
	gardenletConfig.SeedSelector = nil

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
		shootDomain = common.GetAPIServerDomain(*dns.Domain)
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

const (
	gardenletKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
	gardenletKubeconfigSecretName          = "gardenlet-kubeconfig"
)

func (a *actuator) prepareGardenClientConnectionWithBootstrap(ctx context.Context, shootClient kubernetes.Interface, gcc *configv1alpha1.GardenClientConnection, name string, bootstrap seedmanagementv1alpha1.Bootstrap) (string, error) {
	// Ensure kubeconfig is not set
	gcc.Kubeconfig = ""

	// Ensure kubeconfig secret is set
	if gcc.KubeconfigSecret == nil {
		gcc.KubeconfigSecret = &corev1.SecretReference{
			Name:      gardenletKubeconfigSecretName,
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	// If kubeconfig secret exists, return an empty result, since the bootstrap can be skipped
	secret, err := kutil.GetSecretByReference(ctx, shootClient.Client(), gcc.KubeconfigSecret)
	if client.IgnoreNotFound(err) != nil {
		return "", err
	}
	if secret != nil {
		return "", nil
	}

	// Ensure bootstrap kubeconfig secret is set
	if gcc.BootstrapKubeconfig == nil {
		gcc.BootstrapKubeconfig = &corev1.SecretReference{
			Name:      gardenletKubeconfigBootstrapSecretName,
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	// Prepare bootstrap kubeconfig
	return a.prepareBootstrapKubeconfig(ctx, name, bootstrap, gcc.GardenClusterAddress, gcc.GardenClusterCACert)
}

func (a *actuator) prepareGardenClientConnectionWithoutBootstrap(gcc *configv1alpha1.GardenClientConnection) {
	// Ensure kubeconfig secret and bootstrap kubeconfig secret are not set
	gcc.KubeconfigSecret = nil
	gcc.BootstrapKubeconfig = nil
}

func (a *actuator) prepareBootstrapKubeconfig(ctx context.Context, name string, bootstrap seedmanagementv1alpha1.Bootstrap, address *string, caCert []byte) (string, error) {
	var err error

	// Prepare RESTConfig
	restConfig := *a.gardenClient.RESTConfig()
	if address != nil {
		restConfig.Host = *address
	}
	if caCert != nil {
		restConfig.TLSClientConfig = rest.TLSClientConfig{
			CAData: caCert,
		}
	}

	var bootstrapKubeconfig []byte
	switch bootstrap {
	case seedmanagementv1alpha1.BootstrapServiceAccount:
		// Create a temporary service account with bootstrap kubeconfig in order to create CSR

		// Create a temporary service account
		saName := "gardenlet-bootstrap-" + name
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
		if _, err := controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), sa, func() error { return nil }); err != nil {
			return "", err
		}

		// Get the service account secret
		if len(sa.Secrets) == 0 {
			return "", fmt.Errorf("service account token controller has not yet created a secret for the service account")
		}
		saSecret := &corev1.Secret{}
		if err := a.gardenClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, sa.Secrets[0].Name), saSecret); err != nil {
			return "", err
		}

		// Create a ClusterRoleBinding
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: bootstraputil.BuildBootstrapperName(name),
			},
		}
		if _, err := controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), clusterRoleBinding, func() error {
			clusterRoleBinding.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     bootstraputil.GardenerSeedBootstrapper,
			}
			clusterRoleBinding.Subjects = []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      saName,
					Namespace: v1beta1constants.GardenNamespace,
				},
			}
			return nil
		}); err != nil {
			return "", err
		}

		// Get bootstrap kubeconfig from service account secret
		bootstrapKubeconfig, err = bootstraputil.MarshalKubeconfigWithToken(&restConfig, string(saSecret.Data[corev1.ServiceAccountTokenKey]))
		if err != nil {
			return "", err
		}

	case seedmanagementv1alpha1.BootstrapToken:
		// Create bootstrap token with bootstrap kubeconfig in order to create CSR

		// Get bootstrap token secret
		tokenID := utils.ComputeSHA256Hex([]byte(name))[:6]
		secret := &corev1.Secret{}
		if err := a.gardenClient.Client().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstraptokenutil.BootstrapTokenSecretName(tokenID)), secret); client.IgnoreNotFound(err) != nil {
			return "", err
		}

		// Refresh bootstrap token if needed
		refreshBootstrapToken := true
		var bootstrapTokenSecret *corev1.Secret
		if expirationTime, ok := secret.Data[bootstraptokenapi.BootstrapTokenExpirationKey]; ok {
			t, err := time.Parse(time.RFC3339, string(expirationTime))
			if err != nil {
				return "", err
			}
			if !t.Before(metav1.Now().UTC()) {
				refreshBootstrapToken = false
				bootstrapTokenSecret = secret
			}
		}
		if refreshBootstrapToken {
			bootstrapTokenSecret, err = kutil.ComputeBootstrapToken(ctx, a.gardenClient.Client(), tokenID, fmt.Sprintf("A bootstrap token for the Gardenlet for shooted seed %q.", name), 24*time.Hour)
			if err != nil {
				return "", err
			}
		}

		// Get bootstrap kubeconfig from bootstrap token
		bootstrapKubeconfig, err = bootstraputil.MarshalKubeconfigWithToken(&restConfig, kutil.BootstrapTokenFrom(bootstrapTokenSecret.Data))
		if err != nil {
			return "", err
		}
	}

	return string(bootstrapKubeconfig), nil
}
