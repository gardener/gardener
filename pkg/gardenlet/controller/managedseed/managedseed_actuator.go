// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/version"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Actuator acts upon ManagedSeed resources.
type Actuator interface {
	// CreateOrUpdate reconciles ManagedSeed creation or update.
	CreateOrUpdate(context.Context, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Shoot) error
	// Delete reconciles ManagedSeed deletion.
	Delete(context.Context, *seedmanagementv1alpha1.ManagedSeed, *gardencorev1beta1.Shoot) error
}

type actuator struct {
	gardenClient kubernetes.Interface
	seedClient   kubernetes.Interface
	shootClient  kubernetes.Interface
	config       *config.GardenletConfiguration
	imageVector  imagevector.ImageVector
	logger       *logrus.Logger
}

func newActuator(gardenClient, seedClient, shootClient kubernetes.Interface, config *config.GardenletConfiguration, imageVector imagevector.ImageVector, logger *logrus.Logger) Actuator {
	return &actuator{
		gardenClient: gardenClient,
		seedClient:   seedClient,
		shootClient:  shootClient,
		config:       config,
		imageVector:  imageVector,
		logger:       logger,
	}
}

// CreateOrUpdate reconciles ManagedSeed creation or update.
func (a *actuator) CreateOrUpdate(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Create garden namespace in the shoot
	managedSeedLogger.Infof("Creating garden namespace in shoot %s", kutil.ObjectName(shoot))
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}}
	if err := a.shootClient.Client().Create(ctx, gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create garden namespace in shoot %q: %+v", kutil.ObjectName(shoot), err)
	}

	switch {
	case managedSeed.Spec.Seed != nil:
		// Register the shoot as seed
		managedSeedLogger.Infof("Registering shoot %s as seed", kutil.ObjectName(shoot))
		if err := a.registerAsSeed(ctx, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not register shoot %q as seed: %+v", kutil.ObjectName(shoot), err)
		}
	case managedSeed.Spec.Gardenlet != nil:
		// Deploy gardenlet into the shoot, it will register the seed automatically
		managedSeedLogger.Infof("Deploying gardenlet into shoot %s", kutil.ObjectName(shoot))
		if err := a.deployGardenlet(ctx, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not deploy gardenlet into shoot %q: %+v", kutil.ObjectName(shoot), err)
		}
	}

	// Set managed seed reference in the shoot
	if shoot.Spec.ManagedSeedName == nil {
		managedSeedLogger.Infof("Setting managed seed reference in shoot %s", kutil.ObjectName(shoot))
		shoot.Spec.ManagedSeedName = &managedSeed.Name
		if err := a.gardenClient.Client().Update(ctx, shoot); err != nil {
			return fmt.Errorf("could not set managed seed reference in shoot %q: %+v", kutil.ObjectName(shoot), err)
		}
	}

	return nil
}

// Delete reconciles ManagedSeed deletion.
func (a *actuator) Delete(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Remove managed seed reference from the shoot
	if shoot.Spec.ManagedSeedName != nil {
		managedSeedLogger.Infof("Removing managed seed reference from shoot %s", kutil.ObjectName(shoot))
		shoot.Spec.ManagedSeedName = nil
		if err := a.gardenClient.Client().Update(ctx, shoot); err != nil {
			return fmt.Errorf("could not remove managed seed reference from shoot %q: %+v", kutil.ObjectName(shoot), err)
		}
	}

	// Deregister the shoot as seed
	managedSeedLogger.Infof("Deregistering shoot %s as seed", kutil.ObjectName(shoot))
	if err := a.deregisterAsSeed(ctx, managedSeed, shoot, managedSeed.Spec.Seed != nil); err != nil {
		return fmt.Errorf("could not dereigster shoot %q as seed: %+v", kutil.ObjectName(shoot), err)
	}

	if managedSeed.Spec.Gardenlet != nil {
		// Delete gardenlet from the shoot
		managedSeedLogger.Infof("Deleting gardenlet from shoot %s", kutil.ObjectName(shoot))
		if err := a.deleteGardenlet(ctx, managedSeed, shoot); err != nil {
			return fmt.Errorf("could not delete gardenlet from shoot %q: %+v", kutil.ObjectName(shoot), err)
		}
	}

	// Delete garden namespace from the shoot
	managedSeedLogger.Infof("Deleting garden namespace from shoot %s", kutil.ObjectName(shoot))
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}}
	if err := a.shootClient.Client().Delete(ctx, gardenNamespace); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not delete garden namespace from shoot %q: %+v", kutil.ObjectName(shoot), err)
	}

	return nil
}

func (a *actuator) registerAsSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Check if the shoot can be registered as seed
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return errors.New("cannot register shoot as seed as it does not specify a domain")
	}

	// Prepare seed spec
	seedSpec, err := a.prepareSeedSpec(ctx, managedSeed, shoot, &managedSeed.Spec.Seed.Spec)
	if err != nil {
		return err
	}

	// Create or update seed object
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, a.gardenClient.Client(), seed, func() error {
		seed.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
		}
		seed.Labels = utils.MergeStringMaps(managedSeed.Spec.Seed.Labels, map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		})
		seed.Annotations = managedSeed.Spec.Seed.Annotations
		seed.Spec = *seedSpec
		return nil
	})
	return err
}

func (a *actuator) deregisterAsSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot, checkOwner bool) error {
	// Get seed object
	seed := &gardencorev1beta1.Seed{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(managedSeed.Name), seed); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Delete seed object
	if err := a.gardenClient.Client().Delete(ctx, seed); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Check if there are any remaining objects associated with this seed
	if err := checkSeedAssociations(ctx, a.gardenClient.Client(), seed.Name); err != nil {
		return err
	}

	// Delete seed secrets
	if seed.Spec.SecretRef != nil {
		if err := kutil.DeleteSecretByRef(ctx, a.gardenClient.Client(), *seed.Spec.SecretRef); err != nil {
			return err
		}
	}
	if seed.Spec.Backup != nil {
		if err := kutil.DeleteSecretByRef(ctx, a.gardenClient.Client(), seed.Spec.Backup.SecretRef); err != nil {
			return err
		}
	}

	// Return an error since the seed still exists
	return fmt.Errorf("seed %q still exists", seed.Name)
}

func (a *actuator) deployGardenlet(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Get gardenlet chart values
	values, err := a.getGardenletChartValues(ctx, managedSeed, shoot)
	if err != nil {
		return err
	}

	// Apply gardenlet chart
	return a.shootClient.ChartApplier().Apply(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
}

func (a *actuator) deleteGardenlet(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	// Get gardenlet chart values
	values, err := a.getGardenletChartValues(ctx, managedSeed, shoot)
	if err != nil {
		return err
	}

	// Delete gardenlet chart
	if err := a.shootClient.ChartApplier().Delete(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values)); err != nil {
		return err
	}

	return nil
}

func (a *actuator) prepareSeedSpec(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot, seedSpec *gardencorev1beta1.SeedSpec) (*gardencorev1beta1.SeedSpec, error) {
	// Get shoot secret
	shootSecret, err := a.getShootSecret(ctx, shoot)
	if err != nil {
		return nil, err
	}

	// Prepare seed backup
	seedBackup, err := a.prepareSeedBackup(ctx, managedSeed, shoot, shootSecret, seedSpec.Backup)
	if err != nil {
		return nil, err
	}

	// Copy seed spec
	spec := seedSpec.DeepCopy()

	// Initialize backup
	spec.Backup = seedBackup

	// Initialize DNS
	if spec.DNS.IngressDomain == "" && shoot.Spec.DNS != nil && shoot.Spec.DNS.Domain != nil {
		spec.DNS.IngressDomain = fmt.Sprintf("%s.%s", common.IngressPrefix, *shoot.Spec.DNS.Domain)
	}

	// Initialize networks
	if spec.Networks.Nodes == nil {
		spec.Networks.Nodes = shoot.Spec.Networking.Nodes
	}
	if spec.Networks.Pods == "" && shoot.Spec.Networking.Pods != nil {
		spec.Networks.Pods = *shoot.Spec.Networking.Pods
	}
	if spec.Networks.Services == "" && shoot.Spec.Networking.Services != nil {
		spec.Networks.Services = *shoot.Spec.Networking.Services
	}

	// Initialize provider
	if spec.Provider.Type == "" {
		spec.Provider.Type = shoot.Spec.Provider.Type
	}
	if spec.Provider.Region == "" {
		spec.Provider.Region = shoot.Spec.Region
	}

	// If secret reference is specified, create or update the corresponding secret
	if spec.SecretRef != nil {
		// Get shoot kubeconfig secret
		shootKubeconfigSecret, err := a.getShootKubeconfigSecret(ctx, shoot)
		if err != nil {
			return nil, err
		}

		// Initialize seed secret data
		data := shootSecret.Data
		data[kubernetes.KubeConfig] = shootKubeconfigSecret.Data[kubernetes.KubeConfig]

		// Create or update seed secret
		ownerRefs := []metav1.OwnerReference{
			*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
		}
		if err := kutil.CreateOrUpdateSecretByRef(ctx, a.gardenClient.Client(), *spec.SecretRef, corev1.SecretTypeOpaque, data, ownerRefs); err != nil {
			return nil, err
		}
	}

	// Initialize settings
	if spec.Settings == nil {
		spec.Settings = &gardencorev1beta1.SeedSettings{}
	}

	// If settings.verticalPodAutoscaler is not specified, enable it if the shoot namespace in the seed doesn't contain
	// a vpa-admission-controller deployment, and disable it otherwise
	if spec.Settings.VerticalPodAutoscaler == nil {
		disabled, err := a.disableVPA(ctx, shoot)
		if err != nil {
			return nil, err
		}
		spec.Settings.VerticalPodAutoscaler = &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
			Enabled: !disabled,
		}
	}

	return spec, nil
}

func (a *actuator) prepareSeedBackup(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot, shootSecret *corev1.Secret, seedBackup *gardencorev1beta1.SeedBackup) (*gardencorev1beta1.SeedBackup, error) {
	if seedBackup == nil {
		return nil, nil
	}

	// Copy backup
	backup := seedBackup.DeepCopy()

	// Initialize provider
	if backup.Provider == "" {
		backup.Provider = shoot.Spec.Provider.Type
	}

	// If backup secret reference is not specified, initialize it and create or update the corresponding secret
	if backup.SecretRef.Name == "" || backup.SecretRef.Namespace == "" {
		// Initialize backup secret reference
		backup.SecretRef = corev1.SecretReference{
			Name:      fmt.Sprintf("backup-%s", managedSeed.Name),
			Namespace: managedSeed.Namespace,
		}

		// Create or update backup secret
		ownerRefs := []metav1.OwnerReference{
			*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
		}
		if err := kutil.CreateOrUpdateSecretByRef(ctx, a.gardenClient.Client(), backup.SecretRef, corev1.SecretTypeOpaque, shootSecret.Data, ownerRefs); err != nil {
			return nil, err
		}
	}

	return backup, nil
}

func (a *actuator) getShootSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootSecretBinding := &gardencorev1beta1.SecretBinding{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), shootSecretBinding); err != nil {
		return nil, err
	}
	return kutil.GetSecretByRef(ctx, a.gardenClient.Client(), shootSecretBinding.SecretRef)
}

func (a *actuator) getShootKubeconfigSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootKubeconfigSecret := &corev1.Secret{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, fmt.Sprintf("%s.kubeconfig", shoot.Name)), shootKubeconfigSecret); err != nil {
		return nil, err
	}
	return shootKubeconfigSecret, nil
}

func (a *actuator) disableVPA(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := a.seedClient.Client().Get(ctx, kutil.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

const (
	gardenletKubeconfigBootstrapSecretName = "gardenlet-kubeconfig-bootstrap"
	gardenletKubeconfigSecretName          = "gardenlet-kubeconfig"
)

func (a *actuator) getGardenletChartValues(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) (map[string]interface{}, error) {
	var err error

	// Determine deployment values
	var deploymentValues map[string]interface{}
	if deploymentValues, err = utils.ToValues(managedSeed.Spec.Gardenlet.Deployment); err != nil {
		return nil, err
	}
	if managedSeed.Spec.Gardenlet.MergeParentConfig {
		parentDeployment, err := getParentDeployment(a.imageVector, shoot)
		if err != nil {
			return nil, err
		}
		parentDeploymentValues, err := utils.ToValues(parentDeployment)
		if err != nil {
			return nil, err
		}
		deploymentValues = utils.MergeMaps(parentDeploymentValues, deploymentValues)
	}

	// Determine config values
	var configValues map[string]interface{}
	if configValues, err = utils.ToValues(managedSeed.Spec.Gardenlet.Config); err != nil {
		return nil, err
	}
	if managedSeed.Spec.Gardenlet.MergeParentConfig {
		parentConfig, err := getParentConfig(a.config)
		if err != nil {
			return nil, err
		}
		parentConfigValues, err := utils.ToValues(parentConfig)
		if err != nil {
			return nil, err
		}
		configValues = utils.MergeMaps(parentConfigValues, configValues)
	}

	// Marshal config values back to an object
	var configObj *configv1alpha1.GardenletConfiguration
	if err := utils.FromValues(configValues, &configObj); err != nil {
		return nil, err
	}

	// If a bootstrap mechanism is specified, compute bootstrap kubeconfig and set gardenClientConnection values accordingly
	// Otherwise, if kubeconfig path is specified in gardenClientConnection, read it and store its contents
	var gccValues map[string]interface{}
	if gccValues, err = utils.GetMapFromValues(configValues, "gardenClientConnection"); err != nil {
		return nil, err
	}
	if managedSeed.Spec.Gardenlet.GardenConnectionBootstrap != nil {
		// Compute bootstrap kubeconfig
		address, _ := gccValues["gardenClusterAddress"].(*string)
		caCert, _ := gccValues["gardenClusterCACert"].([]byte)
		bootstrapKubeconfig, err := a.getBootstrapKubeconfig(ctx, managedSeed.Name, address, caCert, *managedSeed.Spec.Gardenlet.GardenConnectionBootstrap)
		if err != nil {
			return nil, err
		}

		// Set bootstrapKubeconfig and kubeconfigSecret and unset kubeconfig in gardenClientConnection values
		gccValues = utils.InitValues(gccValues)
		if bootstrapKubeconfig != "" {
			gccValues["bootstrapKubeconfig"] = map[string]interface{}{
				"name":       gardenletKubeconfigBootstrapSecretName,
				"namespace":  v1beta1constants.GardenNamespace,
				"kubeconfig": bootstrapKubeconfig,
			}
		}
		gccValues["kubeconfigSecret"] = map[string]interface{}{
			"name":      gardenletKubeconfigSecretName,
			"namespace": v1beta1constants.GardenNamespace,
		}
		delete(gccValues, "kubeconfig")

		if configValues, err = utils.SetMapToValues(configValues, gccValues, "gardenClientConnection"); err != nil {
			return nil, err
		}
	} else if kubeconfigPath, ok := gccValues["kubeconfig"].(string); ok && kubeconfigPath != "" {
		gccValues = utils.InitValues(gccValues)
		delete(gccValues, "bootstrapKubeconfig")
		delete(gccValues, "kubeconfigSecret")
		kubeconfig, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, err
		}
		gccValues["kubeconfig"] = string(kubeconfig)

		if configValues, err = utils.SetMapToValues(configValues, gccValues, "gardenClientConnection"); err != nil {
			return nil, err
		}
	}

	// If a seed connection mechanism is specified, set seedClientConnection values accordingly
	// Otherwise, if kubeconfig path is specified in seedClientConnection, read it and store its contents
	var sccValues map[string]interface{}
	if sccValues, err = utils.GetMapFromValues(configValues, "seedClientConnection"); err != nil {
		return nil, err
	}
	if managedSeed.Spec.Gardenlet.SeedConnection != nil && *managedSeed.Spec.Gardenlet.SeedConnection == seedmanagementv1alpha1.SeedConnectionServiceAccount {
		delete(sccValues, "kubeconfig")

		if configValues, err = utils.SetMapToValues(configValues, sccValues, "seedClientConnection"); err != nil {
			return nil, err
		}
	} else if kubeconfigPath, ok := sccValues["kubeconfig"].(string); ok && kubeconfigPath != "" {
		sccValues = utils.InitValues(sccValues)
		kubeconfig, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, err
		}
		sccValues["kubeconfig"] = string(kubeconfig)

		if configValues, err = utils.SetMapToValues(configValues, sccValues, "seedClientConnection"); err != nil {
			return nil, err
		}
	}

	// Read TLS certificate and key files and store their contents
	var tlsValues map[string]interface{}
	if tlsValues, err = utils.GetMapFromValues(configValues, "server", "https", "tls"); err != nil {
		return nil, err
	}
	if certPath, ok := tlsValues["serverCertPath"].(string); ok && certPath != "" && !strings.Contains(certPath, secrets.TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern) {
		tlsValues = utils.InitValues(tlsValues)
		cert, err := ioutil.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		tlsValues["crt"] = string(cert)
	}
	delete(tlsValues, "serverCertPath")
	if keyPath, ok := tlsValues["serverKeyPath"].(string); ok && keyPath != "" && !strings.Contains(keyPath, secrets.TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern) {
		tlsValues = utils.InitValues(tlsValues)
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		tlsValues["key"] = string(key)
	}
	delete(tlsValues, "serverKeyPath")
	if configValues, err = utils.SetMapToValues(configValues, tlsValues, "server", "https", "tls"); err != nil {
		return nil, err
	}

	// Prepare seed config, set seedConfig, and unset seedSelector
	var seedConfig *configv1alpha1.SeedConfig
	if configObj != nil {
		seedConfig = configObj.SeedConfig
	}
	seedConfig, err = a.prepareSeedConfig(ctx, managedSeed, shoot, seedConfig)
	if err != nil {
		return nil, err
	}
	var seedConfigValues map[string]interface{}
	if seedConfigValues, err = utils.ToValues(seedConfig); err != nil {
		return nil, err
	}
	if configValues, err = utils.SetMapToValues(configValues, seedConfigValues, "seedConfig"); err != nil {
		return nil, err
	}
	delete(configValues, "seedSelector")

	// Compute gardenlet values
	gardenletValues := deploymentValues
	gardenletValues = utils.InitValues(gardenletValues)
	if gardenletValues, err = utils.SetMapToValues(gardenletValues, configValues, "config"); err != nil {
		return nil, err
	}

	// Return gardenlet chart values
	return map[string]interface{}{
		"global": map[string]interface{}{
			"gardenlet": gardenletValues,
		},
	}, nil
}

// getBootstrapKubeconfig returns the bootstrap kubeconfig.
func (a *actuator) getBootstrapKubeconfig(ctx context.Context, name string, address *string, caCert []byte, bootstrap seedmanagementv1alpha1.GardenConnectionBootstrap) (string, error) {
	// If a Gardenlet kubeconfig secret already exists, return an empty result
	var err error
	if err = a.shootClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, gardenletKubeconfigSecretName), &corev1.Secret{}); err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	if err == nil {
		return "", nil
	}

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
	case seedmanagementv1alpha1.GardenConnectionBootstrapServiceAccount:
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

	case seedmanagementv1alpha1.GardenConnectionBootstrapToken:
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

func (a *actuator) prepareSeedConfig(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot, seedConfig *configv1alpha1.SeedConfig) (*configv1alpha1.SeedConfig, error) {
	// Initialize the seed config if it's nil
	if seedConfig == nil {
		seedConfig = &configv1alpha1.SeedConfig{}
	}

	// Set the seed name to the managed seed name
	seedConfig.Seed.Name = managedSeed.Name

	// Prepare the seed spec
	seedSpec, err := a.prepareSeedSpec(ctx, managedSeed, shoot, &seedConfig.Spec)
	if err != nil {
		return nil, err
	}

	// Set the seed spec to the prepared spec
	seedConfig.Seed.Spec = *seedSpec

	return seedConfig, nil
}

func getParentDeployment(imageVector imagevector.ImageVector, shoot *gardencorev1beta1.Shoot) (*seedmanagementv1alpha1.GardenletDeployment, error) {
	// Get image repository and tag
	var imageRepository, imageTag string
	gardenletImage, err := imageVector.FindImage("gardenlet")
	if err != nil {
		return nil, err
	}
	if gardenletImage.Tag != nil {
		imageRepository = gardenletImage.Repository
		imageTag = *gardenletImage.Tag
	} else {
		imageRepository = gardenletImage.String()
		imageTag = version.Get().GitVersion
	}

	// Get image vector overwrite and component image vector overwrites
	var imageVectorOverwrite string
	if overWritePath := os.Getenv(imagevector.OverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return nil, err
		}
		imageVectorOverwrite = string(data)
	}
	var componentImageVectorOverwrites string
	if overWritePath := os.Getenv(imagevector.ComponentOverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return nil, err
		}
		componentImageVectorOverwrites = string(data)
	}

	// Create and return result
	return &seedmanagementv1alpha1.GardenletDeployment{
		RevisionHistoryLimit: pointer.Int32Ptr(0),
		Image: &seedmanagementv1alpha1.Image{
			Repository: imageRepository,
			Tag:        imageTag,
		},
		VPA:                            pointer.BoolPtr(true),
		ImageVectorOverwrite:           imageVectorOverwrite,
		ComponentImageVectorOverwrites: componentImageVectorOverwrites,
		PodAnnotations:                 getParentPodAnnotations(shoot),
	}, nil
}

var minimumAPIServerSNISidecarConstraint *semver.Constraints

func init() {
	var err error
	// 1.13.0-0 must be used or no 1.13.0-dev version can be matched
	minimumAPIServerSNISidecarConstraint, err = semver.NewConstraint(">= 1.13.0-0")
	utilruntime.Must(err)
}

func getParentPodAnnotations(shoot *gardencorev1beta1.Shoot) map[string]string {
	// If APIServerSNI is enabled for the seed cluster then the gardenlet must be restarted, so the Pod injector would
	// add `KUBERNETES_SERVICE_HOST` environment variable.
	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		vers, err := semver.NewVersion(shoot.Status.Gardener.Version)
		if err != nil {
			// We can't really do anything in case of error, since it's not a transient error.
			// Returning an error would force another reconciliation that would fail again here.
			// Reconciling from this point makes no sense, unless the shoot is updated.
			return nil
		}
		if vers != nil && minimumAPIServerSNISidecarConstraint.Check(vers) {
			return map[string]string{
				"networking.gardener.cloud/seed-sni-enabled": "true",
			}
		}
	}
	return nil
}

func getParentConfig(cfg *config.GardenletConfiguration) (*configv1alpha1.GardenletConfiguration, error) {
	// Convert config from internal version to v1alpha1
	scheme := runtime.NewScheme()
	if err := config.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := configv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	obj, err := scheme.ConvertToVersion(cfg, configv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*configv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert Gardenlet config to external version")
	}
	return result, nil
}

func checkSeedAssociations(ctx context.Context, c client.Client, seedName string) error {
	for name, f := range map[string]func(context.Context, client.Client, string) ([]string, error){
		"BackupBuckets":           controllerutils.DetermineBackupBucketAssociations,
		"BackupEntries":           controllerutils.DetermineBackupEntryAssociations,
		"ControllerInstallations": controllerutils.DetermineControllerInstallationAssociations,
		"Shoots":                  controllerutils.DetermineShootAssociations,
	} {
		results, err := f(ctx, c, seedName)
		if err != nil {
			return err
		}
		if len(results) > 0 {
			return fmt.Errorf("%s still associated with seed %q: %+v", name, seedName, results)
		}
	}
	return nil
}
