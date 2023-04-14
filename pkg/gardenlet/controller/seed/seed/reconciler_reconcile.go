// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/crds"
	"github.com/gardener/gardener/pkg/operation/botanist/component/fluentoperator"
	"github.com/gardener/gardener/pkg/operation/botanist/component/hvpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeproxy"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/shared"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	resourcemanagerv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
) (
	reconcile.Result,
	error,
) {
	var (
		seed                      = seedObj.GetInfo()
		conditionSeedBootstrapped = v1beta1helper.GetOrInitConditionWithClock(r.Clock, seedObj.GetInfo().Status.Conditions, gardencorev1beta1.SeedBootstrapped)
	)

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if r.Config.Resources != nil && len(r.Config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(r.Config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(r.Config.Resources.Capacity))

		for resourceName, quantity := range r.Config.Resources.Capacity {
			capacity[resourceName] = quantity
			allocatable[resourceName] = quantity

			if reservedQuantity, ok := r.Config.Resources.Reserved[resourceName]; ok {
				allocatableQuantity := quantity.DeepCopy()
				allocatableQuantity.Sub(reservedQuantity)
				allocatable[resourceName] = allocatableQuantity
			}
		}
	}

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Add the Gardener finalizer to the referenced Seed secret to protect it from deletion as long as the Seed resource
	// does exist.
	if seed.Spec.SecretRef != nil {
		secret, err := kubernetesutils.GetSecretByReference(ctx, r.GardenClient, seed.Spec.SecretRef)
		if err != nil {
			return reconcile.Result{}, err
		}

		if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
			log.Info("Adding finalizer to referenced secret", "secret", client.ObjectKeyFromObject(secret))
			if err := controllerutils.AddFinalizers(ctx, r.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	seedKubernetesVersion, err := r.checkMinimumK8SVersion(r.SeedClientSet.Version())
	if err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "K8SVersionTooOld", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after check for minimum Kubernetes version failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	gardenSecrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(seed.Name), true)
	if err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "GardenSecretsError", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reading garden secrets failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionProgressing, "BootstrapProgressing", "Seed cluster is currently being bootstrapped.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	// Bootstrap the Seed cluster.
	if err := r.runReconcileSeedFlow(ctx, log, seedObj, seedIsGarden, gardenSecrets); err != nil {
		conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionFalse, "BootstrappingFailed", err.Error())
		if err := r.patchSeedStatus(ctx, r.GardenClient, seed, "<unknown>", capacity, allocatable, conditionSeedBootstrapped); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not patch seed status after reconciliation flow failed: %w", err)
		}
		return reconcile.Result{}, err
	}

	// Set the status of SeedSystemComponentsHealthy condition to Progressing so that the Seed does not immediately become ready
	// after being successfully bootstrapped in case the system components got updated. The SeedSystemComponentsHealthy condition
	// will be set to either True, False or Progressing by the seed care reconciler depending on the health of the system components
	// after the necessary checks are completed.
	conditionSeedSystemComponentsHealthy := v1beta1helper.GetOrInitConditionWithClock(r.Clock, seed.Status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy)
	conditionSeedSystemComponentsHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedSystemComponentsHealthy, gardencorev1beta1.ConditionProgressing, "SystemComponentsCheckProgressing", "Pending health check of system components after successful bootstrap of seed cluster.")
	conditionSeedBootstrapped = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionSeedBootstrapped, gardencorev1beta1.ConditionTrue, "BootstrappingSucceeded", "Seed cluster has been bootstrapped successfully.")
	if err = r.patchSeedStatus(ctx, r.GardenClient, seed, seedKubernetesVersion, capacity, allocatable, conditionSeedBootstrapped, conditionSeedSystemComponentsHealthy); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status of %s condition to %s and %s conditions to %s: %w", conditionSeedBootstrapped.Type, gardencorev1beta1.ConditionTrue, conditionSeedSystemComponentsHealthy.Type, gardencorev1beta1.ConditionProgressing, err)
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, r.GardenClient, seed); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Seed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) checkMinimumK8SVersion(version string) (string, error) {
	const minKubernetesVersion = "1.20"

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minKubernetesVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minKubernetesVersion)
	}

	return version, nil
}

const (
	seedBootstrapChartName        = "seed-bootstrap"
	kubeAPIServerPrefix           = "api-seed"
	grafanaPrefix                 = "g-seed"
	prometheusPrefix              = "p-seed"
	ingressTLSCertificateValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176
)

func (r *Reconciler) runReconcileSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
	secrets map[string]*corev1.Secret,
) error {
	var (
		applier       = r.SeedClientSet.Applier()
		seedClient    = r.SeedClientSet.Client()
		chartApplier  = r.SeedClientSet.ChartApplier()
		chartRenderer = r.SeedClientSet.ChartRenderer()
	)

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		clock.RealClock{},
		seedClient,
		r.GardenNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
	if err != nil {
		return err
	}

	// Deploy dedicated CA certificate for seed cluster, auto-rotate it roughly once a month and drop the old CA 24 hours
	// after rotation.
	if _, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCASeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
		Validity:   pointer.Duration(30 * 24 * time.Hour),
	}, secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour)); err != nil {
		return err
	}

	kubernetesVersion, err := semver.NewVersion(r.SeedClientSet.Version())
	if err != nil {
		return err
	}

	var (
		vpaGK    = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK   = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}

		vpaEnabled     = seed.GetInfo().Spec.Settings == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler == nil || seed.GetInfo().Spec.Settings.VerticalPodAutoscaler.Enabled
		loggingEnabled = gardenlethelper.IsLoggingEnabled(&r.Config)
		hvpaEnabled    = features.DefaultFeatureGate.Enabled(features.HVPA)

		loggingConfig   = r.Config.Logging
		gardenNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: r.GardenNamespace,
			},
		}
	)

	if !vpaEnabled {
		// VPA is a prerequisite. If it's not enabled via the seed spec it must be provided through some other mechanism.
		if _, err := seedClient.RESTMapper().RESTMapping(vpaGK); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}
	}

	// create + label garden namespace
	log.Info("Labeling and annotating namespace", "namespaceName", gardenNamespace.Name)
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, seedClient, gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, "role", v1beta1constants.GardenNamespace)

		// When the seed is the garden cluster then this information is managed by gardener-operator.
		if !seedIsGarden {
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			metav1.SetMetaDataAnnotation(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.GetInfo().Spec.Provider.Zones, ","))
		}
		return nil
	}); err != nil {
		return err
	}

	// label kube-system namespace
	namespaceKubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
	log.Info("Labeling namespace", "namespaceName", namespaceKubeSystem.Name)
	patch := client.MergeFrom(namespaceKubeSystem.DeepCopy())
	metav1.SetMetaDataLabel(&namespaceKubeSystem.ObjectMeta, "role", metav1.NamespaceSystem)
	if err := seedClient.Patch(ctx, namespaceKubeSystem, patch); err != nil {
		return err
	}

	// replicate global monitoring secret (read from garden cluster) to the seed cluster's garden namespace
	globalMonitoringSecretGarden, ok := secrets[v1beta1constants.GardenRoleGlobalMonitoring]
	if !ok {
		return errors.New("global monitoring secret not found in seed namespace")
	}

	globalMonitoringSecretSeed := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "seed-" + globalMonitoringSecretGarden.Name,
			Namespace: r.GardenNamespace,
		},
	}

	log.Info("Replicating global monitoring secret to garden namespace in seed", "secret", client.ObjectKeyFromObject(globalMonitoringSecretGarden))
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, globalMonitoringSecretSeed, func() error {
		globalMonitoringSecretSeed.Type = globalMonitoringSecretGarden.Type
		globalMonitoringSecretSeed.Data = globalMonitoringSecretGarden.Data
		globalMonitoringSecretSeed.Immutable = globalMonitoringSecretGarden.Immutable

		if _, ok := globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth]; !ok {
			globalMonitoringSecretSeed.Data[secretsutils.DataKeySHA1Auth] = utils.CreateSHA1Secret(globalMonitoringSecretGarden.Data[secretsutils.DataKeyUserName], globalMonitoringSecretGarden.Data[secretsutils.DataKeyPassword])
		}

		return nil
	}); err != nil {
		return err
	}

	seedImages, err := imagevector.FindImages(
		r.ImageVector,
		[]string{
			images.ImageNameAlertmanager,
			images.ImageNameAlpine,
			images.ImageNameConfigmapReloader,
			images.ImageNameLoki,
			images.ImageNameLokiCurator,
			images.ImageNameTune2fs,
			images.ImageNameFluentBit,
			images.ImageNameFluentBitPluginInstaller,
			images.ImageNameGrafana,
			images.ImageNamePrometheus,
		},
		imagevector.RuntimeVersion(kubernetesVersion.String()),
		imagevector.TargetVersion(kubernetesVersion.String()),
	)
	if err != nil {
		return err
	}

	// Deploy the CRDs in the seed cluster.
	log.Info("Deploying custom resource definitions")

	if hvpaEnabled {
		if err := kubernetesutils.DeleteObjects(ctx, seedClient,
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: r.GardenNamespace}},
			&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "aggregate-prometheus-vpa", Namespace: r.GardenNamespace}},
		); err != nil {
			return err
		}

		if err := hvpa.NewCRD(applier).Deploy(ctx); err != nil {
			return err
		}
	}

	if features.DefaultFeatureGate.Enabled(features.ManagedIstio) {
		istioCRDs := istio.NewCRD(chartApplier, seedClient)
		if err := istioCRDs.Deploy(ctx); err != nil {
			return err
		}
	}

	if !seedIsGarden && vpaEnabled {
		if err := vpa.NewCRD(applier, nil).Deploy(ctx); err != nil {
			return err
		}
	}

	if err := fluentoperator.NewCRDs(applier).Deploy(ctx); err != nil {
		return err
	}

	if err := crds.NewExtensionsCRD(applier).Deploy(ctx); err != nil {
		return err
	}

	// When the seed is the garden cluster then gardener-resource-manager is reconciled by the gardener-operator.
	if !seedIsGarden {
		// Deploy gardener-resource-manager first since it serves central functionality (e.g., projected token mount
		// webhook) which is required for all other components to start-up.
		gardenerResourceManager, err := sharedcomponent.NewGardenerResourceManager(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			secretsManager,
			r.Config.LogLevel, r.Config.LogFormat,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystemCritical,
			features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
			v1beta1helper.SeedSettingTopologyAwareRoutingEnabled(seed.GetInfo().Spec.Settings),
			features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
			true,
			&resourcemanagerv1alpha1.IngressControllerSelector{
				Namespace: v1beta1constants.GardenNamespace,
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
					v1beta1constants.LabelApp:      nginxingress.LabelAppValue,
					nginxingress.LabelKeyComponent: nginxingress.LabelValueController,
				}},
			},
			seed.GetInfo().Spec.Provider.Zones,
		)
		if err != nil {
			return err
		}

		log.Info("Deploying and waiting for gardener-resource-manager to be healthy")
		if err := component.OpWait(gardenerResourceManager).Deploy(ctx); err != nil {
			return err
		}
	}

	// Fetch component-specific aggregate and central monitoring configuration
	var (
		aggregateScrapeConfigs                = strings.Builder{}
		aggregateMonitoringComponentFunctions []component.AggregateMonitoringConfiguration

		centralScrapeConfigs                            = strings.Builder{}
		centralCAdvisorScrapeConfigMetricRelabelConfigs = strings.Builder{}
		centralMonitoringComponentFunctions             = []component.CentralMonitoringConfiguration{
			hvpa.CentralMonitoringConfiguration,
			kubestatemetrics.CentralMonitoringConfiguration,
		}
	)

	if features.DefaultFeatureGate.Enabled(features.ManagedIstio) {
		aggregateMonitoringComponentFunctions = append(aggregateMonitoringComponentFunctions, istio.AggregateMonitoringConfiguration)
	}

	for _, componentFn := range aggregateMonitoringComponentFunctions {
		aggregateMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range aggregateMonitoringConfig.ScrapeConfigs {
			aggregateScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	for _, componentFn := range centralMonitoringComponentFunctions {
		centralMonitoringConfig, err := componentFn()
		if err != nil {
			return err
		}

		for _, config := range centralMonitoringConfig.ScrapeConfigs {
			centralScrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		for _, config := range centralMonitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs {
			centralCAdvisorScrapeConfigMetricRelabelConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}
	}

	// Logging feature gate
	var (
		additionalEgressIPBlocks          []string
		fluentBitConfigurationsOverwrites = map[string]interface{}{}
		lokiValues                        = map[string]interface{}{}

		filters = strings.Builder{}
		parsers = strings.Builder{}
	)
	lokiValues["enabled"] = loggingEnabled

	if loggingEnabled {
		// check if loki is disabled in gardenlet config
		if !gardenlethelper.IsLokiEnabled(&r.Config) {
			lokiValues["enabled"] = false
			if err := common.DeleteLoki(ctx, seedClient, gardenNamespace.Name); err != nil {
				return err
			}
		} else {
			lokiValues["authEnabled"] = false
			lokiValues["storage"] = loggingConfig.Loki.Garden.Storage
			if err := ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, log, seedClient, *loggingConfig.Loki.Garden.Storage); err != nil {
				return err
			}

			if hvpaEnabled {
				shootInfo := &corev1.ConfigMap{}
				maintenanceBegin := "220000-0000"
				maintenanceEnd := "230000-0000"
				if err := seedClient.Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), shootInfo); err != nil {
					if !apierrors.IsNotFound(err) {
						return err
					}
				} else {
					shootMaintenanceBegin, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceBegin"])
					if err != nil {
						return err
					}
					maintenanceBegin = shootMaintenanceBegin.Add(1, 0, 0).Formatted()

					shootMaintenanceEnd, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceEnd"])
					if err != nil {
						return err
					}
					maintenanceEnd = shootMaintenanceEnd.Add(1, 0, 0).Formatted()
				}

				lokiValues["hvpa"] = map[string]interface{}{
					"enabled": true,
					"maintenanceTimeWindow": map[string]interface{}{
						"begin": maintenanceBegin,
						"end":   maintenanceEnd,
					},
				}

				currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, seedClient, kubernetesutils.Key(r.GardenNamespace, v1beta1constants.StatefulSetNameLoki))
				if err != nil {
					return err
				}
				if len(currentResources) != 0 && currentResources[v1beta1constants.StatefulSetNameLoki] != nil {
					lokiValues["resources"] = map[string]interface{}{
						// Copy requests only, effectively removing limits
						v1beta1constants.StatefulSetNameLoki: &corev1.ResourceRequirements{
							Requests: currentResources[v1beta1constants.StatefulSetNameLoki].Requests,
						},
					}
				}
			}

			lokiValues["priorityClassName"] = v1beta1constants.PriorityClassNameSeedSystem600
		}

		componentsFunctions := []component.CentralLoggingConfiguration{
			// seed system components
			dependencywatchdog.CentralLoggingConfiguration,
			resourcemanager.CentralLoggingConfiguration,
			// shoot control plane components
			etcd.CentralLoggingConfiguration,
			clusterautoscaler.CentralLoggingConfiguration,
			kubeapiserver.CentralLoggingConfiguration,
			kubescheduler.CentralLoggingConfiguration,
			kubecontrollermanager.CentralLoggingConfiguration,
			kubestatemetrics.CentralLoggingConfiguration,
			hvpa.CentralLoggingConfiguration,
			vpa.CentralLoggingConfiguration,
			vpnseedserver.CentralLoggingConfiguration,
			// shoot system components
			coredns.CentralLoggingConfiguration,
			kubeproxy.CentralLoggingConfiguration,
			metricsserver.CentralLoggingConfiguration,
			nodeproblemdetector.CentralLoggingConfiguration,
			vpnshoot.CentralLoggingConfiguration,
			// shoot addon components
			kubernetesdashboard.CentralLoggingConfiguration,
		}

		if gardenlethelper.IsEventLoggingEnabled(&r.Config) {
			componentsFunctions = append(componentsFunctions, eventlogger.CentralLoggingConfiguration)
		}

		// Fetch component specific logging configurations
		for _, componentFn := range componentsFunctions {
			loggingConfig, err := componentFn()
			if err != nil {
				return err
			}

			filters.WriteString(fmt.Sprintln(loggingConfig.Filters))
			parsers.WriteString(fmt.Sprintln(loggingConfig.Parsers))
		}

		// Read extension provider specific logging configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err = seedClient.List(ctx, existingConfigMaps,
			client.InNamespace(r.GardenNamespace),
			client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelLogging}); err != nil {
			return err
		}

		// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
		kubernetesutils.ByName().Sort(existingConfigMaps)

		// Read all filters and parsers coming from the extension provider configurations
		for _, cm := range existingConfigMaps.Items {
			filters.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapKubernetesFilter]))
			parsers.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapParser]))
		}

		if loggingConfig != nil && loggingConfig.FluentBit != nil {
			fbConfig := loggingConfig.FluentBit

			if fbConfig.NetworkPolicy != nil {
				additionalEgressIPBlocks = fbConfig.NetworkPolicy.AdditionalEgressIPBlocks
			}

			if fbConfig.ServiceSection != nil {
				fluentBitConfigurationsOverwrites["service"] = *fbConfig.ServiceSection
			}
			if fbConfig.InputSection != nil {
				fluentBitConfigurationsOverwrites["input"] = *fbConfig.InputSection
			}
			if fbConfig.OutputSection != nil {
				fluentBitConfigurationsOverwrites["output"] = *fbConfig.OutputSection
			}
		}
	} else {
		if err := common.DeleteSeedLoggingStack(ctx, seedClient); err != nil {
			return err
		}
	}

	// Monitoring resource values
	monitoringResources := map[string]interface{}{
		"prometheus":           map[string]interface{}{},
		"aggregate-prometheus": map[string]interface{}{},
	}

	if hvpaEnabled {
		for resource := range monitoringResources {
			currentResources, err := kubernetesutils.GetContainerResourcesInStatefulSet(ctx, seedClient, kubernetesutils.Key(r.GardenNamespace, resource))
			if err != nil {
				return err
			}
			if len(currentResources) != 0 && currentResources["prometheus"] != nil {
				monitoringResources[resource] = map[string]interface{}{
					"prometheus": currentResources["prometheus"],
				}
			}
		}
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	if alertingSMTPSecret, ok := secrets[v1beta1constants.GardenRoleAlerting]; ok && string(alertingSMTPSecret.Data["auth_type"]) == "smtp" {
		emailConfig := map[string]interface{}{
			"to":            string(alertingSMTPSecret.Data["to"]),
			"from":          string(alertingSMTPSecret.Data["from"]),
			"smarthost":     string(alertingSMTPSecret.Data["smarthost"]),
			"auth_username": string(alertingSMTPSecret.Data["auth_username"]),
			"auth_identity": string(alertingSMTPSecret.Data["auth_identity"]),
			"auth_password": string(alertingSMTPSecret.Data["auth_password"]),
		}
		alertManagerConfig["enabled"] = true
		alertManagerConfig["emailConfigs"] = []map[string]interface{}{emailConfig}
	} else {
		alertManagerConfig["enabled"] = false
		if err := common.DeleteAlertmanager(ctx, seedClient, r.GardenNamespace); err != nil {
			return err
		}
	}

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		grafanaHost    = seed.GetIngressFQDN(grafanaPrefix)
		prometheusHost = seed.GetIngressFQDN(prometheusPrefix)
	)

	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation

	wildcardCert, err := gardenerutils.GetWildcardCertificate(ctx, seedClient)
	if err != nil {
		return err
	}

	var (
		grafanaIngressTLSSecretName    string
		prometheusIngressTLSSecretName string
	)

	if wildcardCert != nil {
		grafanaIngressTLSSecretName = wildcardCert.GetName()
		prometheusIngressTLSSecretName = wildcardCert.GetName()
	} else {
		grafanaIngressTLSSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "grafana-tls",
			CommonName:                  "grafana",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(grafanaPrefix)},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(ingressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		prometheusIngressTLSSecret, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "aggregate-prometheus-tls",
			CommonName:                  "prometheus",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{seed.GetIngressFQDN(prometheusPrefix)},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(ingressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCASeed))
		if err != nil {
			return err
		}

		grafanaIngressTLSSecretName = grafanaIngressTLSSecret.Name
		prometheusIngressTLSSecretName = prometheusIngressTLSSecret.Name
	}

	imageVectorOverwrites := make(map[string]string, len(r.ComponentImageVectors))
	for name, data := range r.ComponentImageVectors {
		imageVectorOverwrites[name] = data
	}

	anySNIInUse, err := kubeapiserverexposure.AnyDeployedSNI(ctx, seedClient)
	if err != nil {
		return err
	}
	sniEnabledOrInUse := anySNIInUse || features.DefaultFeatureGate.Enabled(features.APIServerSNI)

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	if err := cleanupOrphanExposureClassHandlerResources(ctx, log, seedClient, r.Config.ExposureClassHandlers, seed.GetInfo().Spec.Provider.Zones); err != nil {
		return err
	}

	ingressClass, err := gardenerutils.ComputeNginxIngressClassForSeed(seed.GetInfo(), seed.GetInfo().Status.KubernetesVersion)
	if err != nil {
		return err
	}

	values := kubernetes.Values(map[string]interface{}{
		"global": map[string]interface{}{
			"ingressClass": ingressClass,
			"images":       imagevector.ImageMapToValues(seedImages),
		},
		"prometheus": map[string]interface{}{
			"deployAllowAllAccessNetworkPolicy": !features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster),
			"resources":                         monitoringResources["prometheus"],
			"storage":                           seed.GetValidVolumeSize("10Gi"),
			"additionalScrapeConfigs":           centralScrapeConfigs.String(),
			"additionalCAdvisorScrapeConfigMetricRelabelConfigs": centralCAdvisorScrapeConfigMetricRelabelConfigs.String(),
		},
		"aggregatePrometheus": map[string]interface{}{
			"resources":               monitoringResources["aggregate-prometheus"],
			"storage":                 seed.GetValidVolumeSize("20Gi"),
			"seed":                    seed.GetInfo().Name,
			"hostName":                prometheusHost,
			"secretName":              prometheusIngressTLSSecretName,
			"additionalScrapeConfigs": aggregateScrapeConfigs.String(),
		},
		"grafana": map[string]interface{}{
			"hostName":   grafanaHost,
			"secretName": grafanaIngressTLSSecretName,
		},
		"fluent-bit": map[string]interface{}{
			"enabled":                           loggingEnabled,
			"additionalParsers":                 parsers.String(),
			"additionalFilters":                 filters.String(),
			"fluentBitConfigurationsOverwrites": fluentBitConfigurationsOverwrites,
			"exposedComponentsTagPrefix":        "user-exposed",
			"networkPolicy": map[string]interface{}{
				"additionalEgressIPBlocks": additionalEgressIPBlocks,
			},
		},
		"loki":         lokiValues,
		"alertmanager": alertManagerConfig,
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"istio": map[string]interface{}{
			"enabled": features.DefaultFeatureGate.Enabled(features.ManagedIstio),
		},
		"ingress": map[string]interface{}{
			"authSecretName": globalMonitoringSecretSeed.Name,
		},
	})

	if err := chartApplier.Apply(ctx, filepath.Join(r.ChartsPath, seedBootstrapChartName), r.GardenNamespace, seedBootstrapChartName, values, applierOptions); err != nil {
		return err
	}

	if !v1beta1helper.SeedUsesNginxIngressController(seed.GetInfo()) {
		nginxIngress := nginxingress.New(seedClient, r.GardenNamespace, nginxingress.Values{})

		if err := component.OpDestroyAndWait(nginxIngress).Destroy(ctx); err != nil {
			return err
		}
	}

	if err := migrateIngressClassForShootIngresses(ctx, r.GardenClient, seedClient, seed, ingressClass, kubernetesVersion); err != nil {
		return err
	}

	// setup for flow graph
	var (
		dnsRecord component.DeployMigrateWaiter
		istio     component.DeployWaiter
	)

	if features.DefaultFeatureGate.Enabled(features.ManagedIstio) {
		istio, err = defaultIstio(seedClient, r.ImageVector, chartRenderer, seed, &r.Config, sniEnabledOrInUse)
		if err != nil {
			return err
		}
	}

	networkPolicies, err := defaultNetworkPolicies(seedClient, r.GardenNamespace)
	if err != nil {
		return err
	}
	kubeStateMetrics, err := defaultKubeStateMetrics(seedClient, r.ImageVector, kubernetesVersion, r.GardenNamespace)
	if err != nil {
		return err
	}
	dwdWeeder, dwdProber, err := defaultDependencyWatchdogs(seedClient, kubernetesVersion, r.ImageVector, seed.GetInfo().Spec.Settings, r.GardenNamespace)
	if err != nil {
		return err
	}
	systemResources, err := defaultSystem(seedClient, seed, r.ImageVector, seed.GetInfo().Spec.Settings.ExcessCapacityReservation.Enabled, r.GardenNamespace)
	if err != nil {
		return err
	}
	vpnAuthzServer, err := defaultVPNAuthzServer(ctx, seedClient, kubernetesVersion, r.ImageVector, r.GardenNamespace)
	if err != nil {
		return err
	}

	// TODO(rfranzke): Delete this in a future version.
	{
		if err := kubernetesutils.DeleteObjects(ctx, seedClient,
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: r.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: r.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-aggregate-prometheus", Namespace: r.GardenNamespace}},
		); err != nil {
			return err
		}

	}

	if features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster) {
		if err := kubernetesutils.DeleteObject(ctx, seedClient, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-seed-prometheus", Namespace: r.GardenNamespace}}); err != nil {
			return err
		}
	}

	var (
		g = flow.NewGraph("Seed cluster creation")
		_ = g.Add(flow.Task{
			Name: "Deploying Istio",
			Fn:   flow.TaskFn(func(ctx context.Context) error { return istio.Deploy(ctx) }).DoIf(features.DefaultFeatureGate.Enabled(features.ManagedIstio)),
		})
		_ = g.Add(flow.Task{
			Name: "Ensuring network policies",
			Fn:   networkPolicies.Deploy,
		})
		nginxLBReady = g.Add(flow.Task{
			Name: "Waiting until nginx ingress LoadBalancer is ready",
			Fn: func(ctx context.Context) error {
				dnsRecord, err = waitForNginxIngressServiceAndGetDNSComponent(ctx, log, seed, r.GardenClient, seedClient, r.ImageVector, kubernetesVersion, ingressClass, r.GardenNamespace)
				return err
			},
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying managed ingress DNS record",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return deployDNSResources(ctx, dnsRecord) }).DoIf(v1beta1helper.SeedWantsManagedIngress(seed.GetInfo())),
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying managed ingress DNS record (if existing)",
			Fn:           flow.TaskFn(func(ctx context.Context) error { return destroyDNSResources(ctx, dnsRecord) }).DoIf(!v1beta1helper.SeedWantsManagedIngress(seed.GetInfo())),
			Dependencies: flow.NewTaskIDs(nginxLBReady),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-autoscaler",
			Fn:   clusterautoscaler.NewBootstrapper(seedClient, r.GardenNamespace).Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying kube-state-metrics",
			Fn:   kubeStateMetrics.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-weeder",
			Fn:   dwdWeeder.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying dependency-watchdog-prober",
			Fn:   dwdProber.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying VPN authorization server",
			Fn:   vpnAuthzServer.Deploy,
		})
		_ = g.Add(flow.Task{
			Name: "Deploying system resources",
			Fn:   systemResources.Deploy,
		})
	)

	// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
	// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
	if seedIsOriginOfClusterIdentity {
		_ = g.Add(flow.Task{
			Name: "Deploying cluster-identity",
			Fn:   clusteridentity.NewForSeed(seedClient, r.GardenNamespace, *seed.GetInfo().Status.ClusterIdentity).Deploy,
		})
	} else {
		// This is the migration scenario for the "cluster-identity" managed resource.
		// In the first step the "cluster-identity" config map is annotated with "resources.gardener.cloud/mode: Ignore"
		// In the second step (next release) the migration managed resource will be destroyed.
		// In the last step the migration scenario will be removed entirely.
		// TODO(oliver-goetz): Remove this migration scenario in a future release.
		clusterIdentity := clusteridentity.NewIgnoredManagedResourceForSeed(seedClient, r.GardenNamespace, "")
		_ = g.Add(flow.Task{
			Name: "Destroying cluster-identity migration",
			Fn:   component.OpDestroyAndWait(clusterIdentity).Destroy,
		})
	}

	// When the seed is the garden cluster then the VPA is reconciled by the gardener-operator
	if !seedIsGarden {
		vpa, err := sharedcomponent.NewVerticalPodAutoscaler(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			secretsManager,
			vpaEnabled,
			v1beta1constants.SecretNameCASeed,
			v1beta1constants.PriorityClassNameSeedSystem800,
			v1beta1constants.PriorityClassNameSeedSystem700,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		hvpa, err := sharedcomponent.NewHVPA(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			hvpaEnabled,
			v1beta1constants.PriorityClassNameSeedSystem700,
		)
		if err != nil {
			return err
		}

		etcdDruid, err := sharedcomponent.NewEtcdDruid(
			seedClient,
			r.GardenNamespace,
			kubernetesVersion,
			r.ImageVector,
			r.ComponentImageVectors,
			r.Config.ETCDConfig,
			v1beta1constants.PriorityClassNameSeedSystem800,
		)
		if err != nil {
			return err
		}

		var (
			_ = g.Add(flow.Task{
				Name: "Deploying Kubernetes vertical pod autoscaler",
				Fn:   vpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying HVPA controller",
				Fn:   hvpa.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying ETCD Druid",
				Fn:   etcdDruid.Deploy,
			})
		)
	}

	kubeAPIServerService := kubeapiserverexposure.NewInternalNameService(seedClient, r.GardenNamespace)
	if wildcardCert != nil {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{
			Host:             seed.GetIngressFQDN(kubeAPIServerPrefix),
			IngressClassName: &ingressClass,
			ServiceName:      v1beta1constants.DeploymentNameKubeAPIServer,
			TLSSecretName:    &wildcardCert.Name,
		})
		var (
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver service",
				Fn:   kubeAPIServerService.Deploy,
			})
			_ = g.Add(flow.Task{
				Name: "Deploying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Deploy,
			})
		)
	} else {
		kubeAPIServerIngress := kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{})
		var (
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver service",
				Fn:   kubeAPIServerService.Destroy,
			})
			_ = g.Add(flow.Task{
				Name: "Destroying kube-apiserver ingress",
				Fn:   kubeAPIServerIngress.Destroy,
			})
		)
	}

	if err := g.Compile().Run(ctx, flow.Opts{Log: log}); err != nil {
		return flow.Errors(err)
	}

	return secretsManager.Cleanup(ctx)
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Provider.Type as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Provider.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	ownerRef := metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))

	_, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, k8sGardenClient, backupBucket, func() error {
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   seed.Spec.Backup.Provider,
				Region: region,
			},
			ProviderConfig: seed.Spec.Backup.ProviderConfig,
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			SeedName: &seed.Name, // In future this will be moved to gardener-scheduler.
		}
		return nil
	})
	return err
}

// ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame updates the garden Loki PVC if passed storage value is not the same as the current one.
// Caution: If the passed storage capacity is less than the current one the existing PVC and its PV will be deleted.
func ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx context.Context, log logr.Logger, k8sClient client.Client, newStorageQuantity resource.Quantity) error {
	// Check if we need resizing
	pvc := &corev1.PersistentVolumeClaim{}
	if err := k8sClient.Get(ctx, kubernetesutils.Key(v1beta1constants.GardenNamespace, "loki-loki-0"), pvc); err != nil {
		return client.IgnoreNotFound(err)
	}

	log = log.WithValues("persistentVolumeClaim", client.ObjectKeyFromObject(pvc))

	storageCmpResult := newStorageQuantity.Cmp(*pvc.Spec.Resources.Requests.Storage())
	if storageCmpResult == 0 {
		return nil
	}

	statefulSetKey := client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: v1beta1constants.StatefulSetNameLoki}
	log.Info("Scaling StatefulSet to zero in order to detach PVC", "statefulSet", statefulSetKey)
	if err := kubernetes.ScaleStatefulSetAndWaitUntilScaled(ctx, k8sClient, statefulSetKey, 0); client.IgnoreNotFound(err) != nil {
		return err
	}

	switch {
	case storageCmpResult > 0:
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: newStorageQuantity,
		}
		log.Info("Patching storage of PVC", "storage", newStorageQuantity.String())
		if err := k8sClient.Patch(ctx, pvc, patch); client.IgnoreNotFound(err) != nil {
			return err
		}
	case storageCmpResult < 0:
		log.Info("Deleting PVC because size needs to be reduced")
		if err := client.IgnoreNotFound(k8sClient.Delete(ctx, pvc)); err != nil {
			return err
		}
	}

	lokiSts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.StatefulSetNameLoki, Namespace: v1beta1constants.GardenNamespace}}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, lokiSts))
}

func cleanupOrphanExposureClassHandlerResources(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	exposureClassHandlers []config.ExposureClassHandler,
	zones []string,
) error {
	// Remove ordinary, orphaned istio exposure class namespaces
	exposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, exposureClassHandlerNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler}); err != nil {
		return err
	}

	for _, namespace := range exposureClassHandlerNamespaces.Items {
		if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
			for _, handler := range exposureClassHandlers {
				if *handler.SNI.Ingress.Namespace == namespace.Name {
					return true
				}
			}
			return false
		}); err != nil {
			return err
		}
	}

	// Remove zonal, orphaned istio exposure class namespaces
	zonalExposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalExposureClassHandlerNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)).Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
	}); err != nil {
		return err
	}

	zoneSet := sets.New(zones...)
	for _, namespace := range zonalExposureClassHandlerNamespaces.Items {
		if ok, zone := operation.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
				if !zoneSet.Has(zone) {
					return false
				}
				for _, handler := range exposureClassHandlers {
					if handler.Name == namespace.Labels[v1beta1constants.LabelExposureClassHandlerName] {
						return true
					}
				}
				return false
			}); err != nil {
				return err
			}
		}
	}

	// Remove zonal, orphaned istio default namespaces
	zonalIstioNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalIstioNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(istio.DefaultZoneKey, selection.Exists)),
	}); err != nil {
		return err
	}

	for _, namespace := range zonalIstioNamespaces.Items {
		if ok, zone := operation.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, false, func() bool {
				return zoneSet.Has(zone)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanupOrphanIstioNamespace(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespace corev1.Namespace,
	needsHandler bool,
	isAliveFunc func() bool,
) error {
	log = log.WithValues("namespace", client.ObjectKeyFromObject(&namespace))

	if isAlive := isAliveFunc(); isAlive {
		return nil
	}
	log.Info("Namespace is orphan as there is no ExposureClass handler in the gardenlet configuration anymore or the zone was removed")

	// Determine the corresponding handler name to the ExposureClass handler resources.
	handlerName, ok := namespace.Labels[v1beta1constants.LabelExposureClassHandlerName]
	if !ok && needsHandler {
		log.Info("Cannot delete ExposureClass handler resources as the corresponding handler is unknown and it is not save to remove them")
		return nil
	}

	gatewayList := &istiov1beta1.GatewayList{}
	if err := c.List(ctx, gatewayList); err != nil {
		return err
	}

	for _, gateway := range gatewayList.Items {
		if gateway.Name != v1beta1constants.DeploymentNameKubeAPIServer && gateway.Name != v1beta1constants.DeploymentNameVPNSeedServer {
			continue
		}
		if needsHandler {
			// Check if the gateway still selects the ExposureClass handler ingress gateway.
			if value, ok := gateway.Spec.Selector[v1beta1constants.LabelExposureClassHandlerName]; ok && value == handlerName {
				log.Info("Resources of ExposureClass handler cannot be deleted as they are still in use", "exposureClassHandler", handlerName)
				return nil
			}
		} else {
			_, zone := operation.IsZonalIstioExtension(namespace.Labels)
			if value, ok := gateway.Spec.Selector[istio.DefaultZoneKey]; ok && strings.HasSuffix(value, zone) {
				log.Info("Resources of default zonal istio handler cannot be deleted as they are still in use", "zone", zone)
				return nil
			}
		}
	}

	// ExposureClass handler is orphan and not used by any Shoots anymore
	// therefore it is save to clean it up.
	log.Info("Delete orphan ExposureClass handler namespace")
	if err := c.Delete(ctx, &namespace); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// WaitUntilLoadBalancerIsReady is an alias for kubernetesutils.WaitUntilLoadBalancerIsReady. Exposed for tests.
var WaitUntilLoadBalancerIsReady = kubernetesutils.WaitUntilLoadBalancerIsReady

func waitForNginxIngressServiceAndGetDNSComponent(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	gardenClient, seedClient client.Client,
	imageVector imagevector.ImageVector,
	kubernetesVersion *semver.Version,
	ingressClass string,
	gardenNamespaceName string,
) (
	component.DeployMigrateWaiter,
	error,
) {
	secretData, err := getDNSProviderSecretData(ctx, gardenClient, seed.GetInfo())
	if err != nil {
		return nil, err
	}

	var ingressLoadBalancerAddress string
	if v1beta1helper.SeedUsesNginxIngressController(seed.GetInfo()) {
		providerConfig, err := getConfig(seed.GetInfo())
		if err != nil {
			return nil, err
		}

		nginxIngress, err := defaultNginxIngress(seedClient, imageVector, kubernetesVersion, ingressClass, providerConfig, gardenNamespaceName)
		if err != nil {
			return nil, err
		}

		if err = component.OpWait(nginxIngress).Deploy(ctx); err != nil {
			return nil, err
		}

		ingressLoadBalancerAddress, err = WaitUntilLoadBalancerIsReady(
			ctx,
			log,
			seedClient,
			gardenNamespaceName,
			"nginx-ingress-controller",
			time.Minute,
		)
		if err != nil {
			return nil, err
		}
	}

	return getManagedIngressDNSRecord(log, seedClient, gardenNamespaceName, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), ingressLoadBalancerAddress), nil
}
