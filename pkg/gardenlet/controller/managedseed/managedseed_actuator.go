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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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

	"github.com/Masterminds/semver"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/utils/pointer"
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

// Reconcile reconciles ManagedSeed creation or update.
func (a *actuator) Reconcile(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Create garden namespace in the shoot
	managedSeedLogger.Infof("Creating garden namespace in shoot %s", kutil.ObjectName(shoot))
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.GardenNamespace}}
	if err := a.shootClient.Client().Create(ctx, gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("could not create garden namespace in shoot %q: %+v", kutil.ObjectName(shoot), err)
	}

	switch {
	case managedSeed.Spec.SeedTemplate != nil:
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

	return nil
}

// Delete reconciles ManagedSeed deletion.
func (a *actuator) Delete(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, shoot *gardencorev1beta1.Shoot) error {
	managedSeedLogger := logger.NewFieldLogger(a.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Unregister the shoot as seed
	managedSeedLogger.Infof("Unregistering shoot %s as seed", kutil.ObjectName(shoot))
	if err := a.unregisterAsSeed(ctx, managedSeed); err != nil {
		return fmt.Errorf("could not unreigster shoot %q as seed: %+v", kutil.ObjectName(shoot), err)
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
	// Prepare seed spec
	seedSpec, err := a.prepareSeedSpec(ctx, managedSeed, shoot, &managedSeed.Spec.SeedTemplate.Spec)
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
		seed.Labels = utils.MergeStringMaps(managedSeed.Spec.SeedTemplate.Labels, map[string]string{
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
		})
		seed.Annotations = managedSeed.Spec.SeedTemplate.Annotations
		seed.Spec = *seedSpec
		return nil
	})
	return err
}

func (a *actuator) unregisterAsSeed(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
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
		if err := kutil.DeleteSecretByReference(ctx, a.gardenClient.Client(), seed.Spec.SecretRef); err != nil {
			return err
		}
	}
	if seed.Spec.Backup != nil {
		if err := kutil.DeleteSecretByReference(ctx, a.gardenClient.Client(), &seed.Spec.Backup.SecretRef); err != nil {
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
	return a.shootClient.ChartApplier().Delete(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(values))
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

	// Initialize DNS and ingress
	ingressDomain := fmt.Sprintf("%s.%s", common.IngressPrefix, *(shoot.Spec.DNS.Domain))
	enableIngressController, err := a.shouldEnableIngressController(ctx, shoot)
	if err != nil {
		return nil, err
	}
	if !enableIngressController {
		spec.Ingress = nil
	}
	if spec.Ingress != nil {
		if spec.DNS.Provider == nil {
			dnsProvider, err := a.getSeedDNSProvider(ctx, shoot)
			if err != nil {
				return nil, err
			}
			spec.DNS.Provider = dnsProvider
		}
		if spec.Ingress.Domain == "" {
			spec.Ingress.Domain = ingressDomain
		}
	} else {
		if spec.DNS.IngressDomain == nil || *spec.DNS.IngressDomain == "" {
			spec.DNS.IngressDomain = &ingressDomain
		}
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
		if err := kutil.CreateOrUpdateSecretByReference(ctx, a.gardenClient.Client(), spec.SecretRef, corev1.SecretTypeOpaque, data, ownerRefs); err != nil {
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
		disableVPA, err := a.shouldDisableVPA(ctx, shoot)
		if err != nil {
			return nil, err
		}
		spec.Settings.VerticalPodAutoscaler = &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
			Enabled: !disableVPA,
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
		if err := kutil.CreateOrUpdateSecretByReference(ctx, a.gardenClient.Client(), &backup.SecretRef, corev1.SecretTypeOpaque, shootSecret.Data, ownerRefs); err != nil {
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
	return kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &shootSecretBinding.SecretRef)
}

func (a *actuator) getShootKubeconfigSecret(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*corev1.Secret, error) {
	shootKubeconfigSecret := &corev1.Secret{}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, fmt.Sprintf("%s.kubeconfig", shoot.Name)), shootKubeconfigSecret); err != nil {
		return nil, err
	}
	return shootKubeconfigSecret, nil
}

func (a *actuator) shouldDisableVPA(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if err := a.seedClient.Client().Get(ctx, kutil.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *actuator) shouldEnableIngressController(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	if gardencorev1beta1helper.NginxIngressEnabled(shoot.Spec.Addons) {
		return false, nil
	}

	// If migrating from shoot nginx addon to seed ingress controller wait until DNSEntry pointing to ShootNginxAddon service
	// got deleted before activating seed ingress controller.
	if err := a.seedClient.Client().Get(ctx, kutil.Key(shoot.Status.TechnicalID, common.ShootDNSIngressName), &dnsv1alpha1.DNSEntry{}); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

func (a *actuator) getSeedDNSProvider(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.SeedDNSProvider, error) {
	var dnsProvider *gardencorev1beta1.SeedDNSProvider
	dnsProvider, err := a.getSeedDNSProviderForCustomDomain(ctx, shoot)
	if err != nil {
		return nil, err
	}
	if dnsProvider == nil {
		dnsProvider, err = a.getSeedDNSProviderForDefaultDomain(ctx, shoot)
		if err != nil {
			return nil, err
		}
	}
	if dnsProvider == nil {
		return nil, fmt.Errorf("domain of shoot %s is neither a custom domain nor a default domain", kutil.ObjectName(shoot))
	}
	return dnsProvider, nil
}

func (a *actuator) getSeedDNSProviderForCustomDomain(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.SeedDNSProvider, error) {
	// Find a primary DNS provider in the list of shoot DNS providers
	primaryProvider := gardencorev1beta1helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	if primaryProvider == nil {
		return nil, nil
	}
	if primaryProvider.Type == nil {
		return nil, fmt.Errorf("primary DNS provider of shoot %s does not have a type", kutil.ObjectName(shoot))
	}
	if *primaryProvider.Type == core.DNSUnmanaged {
		return nil, nil
	}

	// Initialize a reference to the primary DNS provider secret
	var secretRef corev1.SecretReference
	if primaryProvider.SecretName != nil {
		secretRef.Name = *primaryProvider.SecretName
		secretRef.Namespace = shoot.Namespace
	} else {
		secretBinding := &gardencorev1beta1.SecretBinding{}
		if err := a.gardenClient.Client().Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), secretBinding); err != nil {
			return nil, err
		}
		secretRef = secretBinding.SecretRef
	}

	return &gardencorev1beta1.SeedDNSProvider{
		Type:      *primaryProvider.Type,
		SecretRef: secretRef,
		Domains:   primaryProvider.Domains,
		Zones:     primaryProvider.Zones,
	}, nil
}

func (a *actuator) getSeedDNSProviderForDefaultDomain(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.SeedDNSProvider, error) {
	// Get all default domain secrets in the garden namespace
	defaultDomainSecrets := &corev1.SecretList{}
	if err := a.gardenClient.Client().List(ctx, defaultDomainSecrets, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{
		v1beta1constants.GardenRole: common.GardenRoleDefaultDomain,
	}); err != nil {
		return nil, err
	}

	// Search for a default domain secret that matches the shoot domain
	for _, secret := range defaultDomainSecrets.Items {
		provider, domain, includeZones, excludeZones, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(*shoot.Spec.DNS.Domain, domain) {
			var zones *gardencorev1beta1.DNSIncludeExclude
			if includeZones != nil || excludeZones != nil {
				zones = &gardencorev1beta1.DNSIncludeExclude{
					Include: includeZones,
					Exclude: excludeZones,
				}
			}

			return &gardencorev1beta1.SeedDNSProvider{
				Type: provider,
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				Domains: &gardencorev1beta1.DNSIncludeExclude{
					Include: []string{domain},
				},
				Zones: zones,
			}, nil
		}
	}

	return nil, nil
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
	if managedSeed.Spec.Gardenlet.DisableMergingWithParent == nil || !*managedSeed.Spec.Gardenlet.DisableMergingWithParent {
		parentDeployment, err := getParentGardenletDeployment(a.imageVector, shoot)
		if err != nil {
			return nil, err
		}
		parentDeploymentValues, err := utils.ToValues(parentDeployment)
		if err != nil {
			return nil, err
		}

		// Set imageVectorOverwrite and componentImageVectorOverwrites in parent values
		parentDeploymentValues["imageVectorOverwrite"], err = getParentImageVectorOverwrite()
		if err != nil {
			return nil, err
		}
		parentDeploymentValues["componentImageVectorOverwrites"], err = getParentComponentImageVectorOverwrites()
		if err != nil {
			return nil, err
		}

		deploymentValues = utils.MergeMaps(parentDeploymentValues, deploymentValues)
	}

	// Determine config values
	gardenletConfig, err := getGardenletConfig(managedSeed.Spec.Gardenlet.Config)
	if err != nil {
		return nil, err
	}
	var configValues map[string]interface{}
	if configValues, err = utils.ToValues(gardenletConfig); err != nil {
		return nil, err
	}
	if managedSeed.Spec.Gardenlet.DisableMergingWithParent == nil || !*managedSeed.Spec.Gardenlet.DisableMergingWithParent {
		parentConfig, err := getParentGardenletConfig(a.config)
		if err != nil {
			return nil, err
		}
		parentConfigValues, err := utils.ToValues(parentConfig)
		if err != nil {
			return nil, err
		}

		// Delete seedClientConnection.kubeconfig in parent values
		var parentSCCValues map[string]interface{}
		if parentSCCValues, err = utils.GetMapFromValues(parentConfigValues, "seedClientConnection"); err != nil {
			return nil, err
		}
		delete(parentSCCValues, "kubeconfig")
		if parentConfigValues, err = utils.SetMapToValues(parentConfigValues, parentSCCValues, "seedClientConnection"); err != nil {
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
		gccValues = utils.InitValues(gccValues)

		// Compute bootstrap kubeconfig
		address, _ := gccValues["gardenClusterAddress"].(*string)
		caCert, _ := gccValues["gardenClusterCACert"].([]byte)
		bootstrapKubeconfig, err := a.getBootstrapKubeconfig(ctx, managedSeed.Name, address, caCert, *managedSeed.Spec.Gardenlet.GardenConnectionBootstrap)
		if err != nil {
			return nil, err
		}

		// Ensure bootstrapKubeconfig and kubeconfigSecret are set in gardenClientConnection values
		if bootstrapKubeconfig != "" {
			var bkcValues map[string]interface{}
			if bkcValues, err = utils.GetMapFromValues(gccValues, "bootstrapKubeconfig"); err != nil {
				return nil, err
			}
			bkcValues = utils.SetStringValueIfEmpty(bkcValues, "name", gardenletKubeconfigBootstrapSecretName)
			bkcValues = utils.SetStringValueIfEmpty(bkcValues, "namespace", v1beta1constants.GardenNamespace)
			bkcValues["kubeconfig"] = bootstrapKubeconfig
			if gccValues, err = utils.SetMapToValues(gccValues, bkcValues, "bootstrapKubeconfig"); err != nil {
				return nil, err
			}
		}
		var kcsValues map[string]interface{}
		if kcsValues, err = utils.GetMapFromValues(gccValues, "kubeconfigSecret"); err != nil {
			return nil, err
		}
		kcsValues = utils.SetStringValueIfEmpty(kcsValues, "name", gardenletKubeconfigSecretName)
		kcsValues = utils.SetStringValueIfEmpty(kcsValues, "namespace", v1beta1constants.GardenNamespace)
		if gccValues, err = utils.SetMapToValues(gccValues, kcsValues, "kubeconfigSecret"); err != nil {
			return nil, err
		}

		// Unset kubeconfig in gardenClientConnection values
		delete(gccValues, "kubeconfig")
	} else if kubeconfigPath, ok := gccValues["kubeconfig"].(string); ok && kubeconfigPath != "" {
		gccValues = utils.InitValues(gccValues)

		// Unset bootstrapKubeconfig and kubeconfigSecret in gardenClientConnection values
		delete(gccValues, "bootstrapKubeconfig")
		delete(gccValues, "kubeconfigSecret")

		// Set kubeconfig in gardenClientConnection values
		kubeconfig, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, err
		}
		gccValues["kubeconfig"] = string(kubeconfig)
	}
	if configValues, err = utils.SetMapToValues(configValues, gccValues, "gardenClientConnection"); err != nil {
		return nil, err
	}

	// If kubeconfig path is specified in seedClientConnection, read it and store its contents
	var sccValues map[string]interface{}
	if sccValues, err = utils.GetMapFromValues(configValues, "seedClientConnection"); err != nil {
		return nil, err
	}
	if kubeconfigPath, ok := sccValues["kubeconfig"].(string); ok && kubeconfigPath != "" {
		sccValues = utils.InitValues(sccValues)
		kubeconfig, err := ioutil.ReadFile(kubeconfigPath)
		if err != nil {
			return nil, err
		}
		sccValues["kubeconfig"] = string(kubeconfig)
	}
	if configValues, err = utils.SetMapToValues(configValues, sccValues, "seedClientConnection"); err != nil {
		return nil, err
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
	if err = a.shootClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, gardenletKubeconfigSecretName), &corev1.Secret{}); client.IgnoreNotFound(err) != nil {
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

func getParentGardenletDeployment(imageVector imagevector.ImageVector, shoot *gardencorev1beta1.Shoot) (*seedmanagementv1alpha1.GardenletDeployment, error) {
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

	// Create and return result
	return &seedmanagementv1alpha1.GardenletDeployment{
		RevisionHistoryLimit: pointer.Int32Ptr(0),
		Image: &seedmanagementv1alpha1.Image{
			Repository: &imageRepository,
			Tag:        &imageTag,
		},
		VPA:            pointer.BoolPtr(true),
		PodAnnotations: getParentPodAnnotations(shoot),
	}, nil
}

func getParentImageVectorOverwrite() (string, error) {
	var imageVectorOverwrite string
	if overWritePath := os.Getenv(imagevector.OverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return "", err
		}
		imageVectorOverwrite = string(data)
	}
	return imageVectorOverwrite, nil
}

func getParentComponentImageVectorOverwrites() (string, error) {
	var componentImageVectorOverwrites string
	if overWritePath := os.Getenv(imagevector.ComponentOverrideEnv); len(overWritePath) > 0 {
		data, err := ioutil.ReadFile(overWritePath)
		if err != nil {
			return "", err
		}
		componentImageVectorOverwrites = string(data)
	}
	return componentImageVectorOverwrites, nil
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

func getGardenletConfig(rawConfig *runtime.RawExtension) (*configv1alpha1.GardenletConfiguration, error) {
	scheme, err := getGardenletConfigScheme()
	if err != nil {
		return nil, err
	}
	cfg := &config.GardenletConfiguration{}
	if _, _, err := serializer.NewCodecFactory(scheme).UniversalDecoder().Decode(rawConfig.Raw, nil, cfg); err != nil {
		return nil, err
	}
	return convertGardenletConfig(cfg, scheme)
}

func getParentGardenletConfig(cfg *config.GardenletConfiguration) (*configv1alpha1.GardenletConfiguration, error) {
	scheme, err := getGardenletConfigScheme()
	if err != nil {
		return nil, err
	}
	return convertGardenletConfig(cfg, scheme)
}

func getGardenletConfigScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := config.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := configv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func convertGardenletConfig(cfg *config.GardenletConfiguration, scheme *runtime.Scheme) (*configv1alpha1.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(cfg, configv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*configv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert Gardenlet config to version %s", configv1alpha1.SchemeGroupVersion.String())
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
