// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver/v3"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	"github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/component/logging"
	"github.com/gardener/gardener/pkg/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/metricsserver"
	"github.com/gardener/gardener/pkg/component/monitoring"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheus"
	cacheprometheus "github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	"github.com/gardener/gardener/pkg/component/nodeexporter"
	"github.com/gardener/gardener/pkg/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/seedsystem"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/component/vpnshoot"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

func defaultIstio(
	ctx context.Context,
	seedClient client.Client,
	chartRenderer chartrenderer.Interface,
	seed *seedpkg.Seed,
	conf *config.GardenletConfiguration,
	isGardenCluster bool,
) (
	component.DeployWaiter,
	map[string]string,
	string,
	error,
) {
	var (
		seedObj = seed.GetInfo()
		labels  = shared.GetIstioZoneLabels(conf.SNI.Ingress.Labels, nil)
	)

	istioDeployer, err := shared.NewIstio(
		ctx,
		seedClient,
		chartRenderer,
		"",
		*conf.SNI.Ingress.Namespace,
		v1beta1constants.PriorityClassNameSeedSystemCritical,
		!isGardenCluster,
		labels,
		gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port),
		seed.GetLoadBalancerServiceAnnotations(),
		seed.GetLoadBalancerServiceExternalTrafficPolicy(),
		conf.SNI.Ingress.ServiceExternalIP,
		[]corev1.ServicePort{
			{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt32(8443)},
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt32(9443)},
			{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt32(vpnseedserver.GatewayPort)},
		},
		true,
		true,
		seedObj.Spec.Provider.Zones,
		seed.IsDualStack(),
	)
	if err != nil {
		return nil, nil, "", err
	}

	// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
	if len(seedObj.Spec.Provider.Zones) > 1 {
		for _, zone := range seedObj.Spec.Provider.Zones {
			if err := shared.AddIstioIngressGateway(
				ctx,
				seedClient,
				istioDeployer,
				shared.GetIstioNamespaceForZone(*conf.SNI.Ingress.Namespace, zone),
				seed.GetZonalLoadBalancerServiceAnnotations(zone),
				shared.GetIstioZoneLabels(labels, &zone),
				seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
				nil,
				&zone,
				seed.IsDualStack(),
			); err != nil {
				return nil, nil, "", err
			}
		}
	}

	// Add for each ExposureClass handler in the config an own Ingress Gateway and Proxy Gateway.
	for _, handler := range conf.ExposureClassHandlers {
		if err := shared.AddIstioIngressGateway(
			ctx,
			seedClient,
			istioDeployer,
			*handler.SNI.Ingress.Namespace,
			// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
			utils.MergeStringMaps(seed.GetLoadBalancerServiceAnnotations(), handler.LoadBalancerService.Annotations),
			shared.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), nil),
			seed.GetLoadBalancerServiceExternalTrafficPolicy(),
			handler.SNI.Ingress.ServiceExternalIP,
			nil,
			seed.IsDualStack(),
		); err != nil {
			return nil, nil, "", err
		}

		// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
		if len(seedObj.Spec.Provider.Zones) > 1 {
			for _, zone := range seedObj.Spec.Provider.Zones {
				if err := shared.AddIstioIngressGateway(
					ctx,
					seedClient,
					istioDeployer,
					shared.GetIstioNamespaceForZone(*handler.SNI.Ingress.Namespace, zone),
					// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
					utils.MergeStringMaps(seed.GetZonalLoadBalancerServiceAnnotations(zone), handler.LoadBalancerService.Annotations),
					shared.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), &zone),
					seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
					nil,
					&zone,
					seed.IsDualStack(),
				); err != nil {
					return nil, nil, "", err
				}
			}
		}
	}

	return istioDeployer, labels, istioDeployer.GetValues().IngressGateway[0].Namespace, nil
}

func defaultDependencyWatchdogs(
	c client.Client,
	seedVersion *semver.Version,
	seedSettings *gardencorev1beta1.SeedSettings,
	gardenNamespaceName string,
) (
	dwdWeeder component.DeployWaiter,
	dwdProber component.DeployWaiter,
	err error,
) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameDependencyWatchdog, imagevectorutils.RuntimeVersion(seedVersion.String()), imagevectorutils.TargetVersion(seedVersion.String()))
	if err != nil {
		return nil, nil, err
	}

	var (
		dwdWeederValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleWeeder, Image: image.String(), KubernetesVersion: seedVersion}
		dwdProberValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProber, Image: image.String(), KubernetesVersion: seedVersion}
	)

	dwdWeeder = component.OpDestroyWithoutWait(dependencywatchdog.NewBootstrapper(c, gardenNamespaceName, dwdWeederValues))
	dwdProber = component.OpDestroyWithoutWait(dependencywatchdog.NewBootstrapper(c, gardenNamespaceName, dwdProberValues))

	if v1beta1helper.SeedSettingDependencyWatchdogWeederEnabled(seedSettings) {
		// Fetch component-specific dependency-watchdog configuration
		var (
			dependencyWatchdogWeederConfigurationFuncs = []dependencywatchdog.WeederConfigurationFunc{
				func() (map[string]weederapi.DependantSelectors, error) {
					return etcd.NewDependencyWatchdogWeederConfiguration(v1beta1constants.ETCDRoleMain)
				},
				kubeapiserver.NewDependencyWatchdogWeederConfiguration,
			}
			dependencyWatchdogWeederConfiguration = weederapi.Config{
				WatchDuration:                 &metav1.Duration{Duration: dependencywatchdog.DefaultWatchDuration},
				ServicesAndDependantSelectors: make(map[string]weederapi.DependantSelectors, len(dependencyWatchdogWeederConfigurationFuncs)),
			}
		)

		for _, componentFn := range dependencyWatchdogWeederConfigurationFuncs {
			dwdConfig, err := componentFn()
			if err != nil {
				return nil, nil, err
			}
			for k, v := range dwdConfig {
				dependencyWatchdogWeederConfiguration.ServicesAndDependantSelectors[k] = v
			}
		}

		dwdWeederValues.WeederConfig = dependencyWatchdogWeederConfiguration
		dwdWeeder = dependencywatchdog.NewBootstrapper(c, gardenNamespaceName, dwdWeederValues)
	}

	if v1beta1helper.SeedSettingDependencyWatchdogProberEnabled(seedSettings) {
		// Fetch component-specific dependency-watchdog configuration
		var (
			dependencyWatchdogProberConfigurationFuncs = []dependencywatchdog.ProberConfigurationFunc{
				kubeapiserver.NewDependencyWatchdogProberConfiguration,
			}
			dependencyWatchdogProberConfiguration = proberapi.Config{
				InternalKubeConfigSecretName: dependencywatchdog.InternalProbeSecretName,
				ExternalKubeConfigSecretName: dependencywatchdog.ExternalProbeSecretName,
				ProbeInterval:                &metav1.Duration{Duration: dependencywatchdog.DefaultProbeInterval},
				DependentResourceInfos:       make([]proberapi.DependentResourceInfo, 0, len(dependencyWatchdogProberConfigurationFuncs)),
			}
		)

		for _, componentFn := range dependencyWatchdogProberConfigurationFuncs {
			dwdConfig, err := componentFn()
			if err != nil {
				return nil, nil, err
			}
			dependencyWatchdogProberConfiguration.DependentResourceInfos = append(dependencyWatchdogProberConfiguration.DependentResourceInfos, dwdConfig...)
		}

		dwdProberValues.ProberConfig = dependencyWatchdogProberConfiguration
		dwdProber = dependencywatchdog.NewBootstrapper(c, gardenNamespaceName, dwdProberValues)
	}

	return
}

func defaultVPNAuthzServer(
	c client.Client,
	seedVersion *semver.Version,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameExtAuthzServer, imagevectorutils.RuntimeVersion(seedVersion.String()), imagevectorutils.TargetVersion(seedVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpnauthzserver.New(
		c,
		gardenNamespaceName,
		image.String(),
		seedVersion,
	), nil
}

func defaultSystem(
	c client.Client,
	seed *seedpkg.Seed,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePauseContainer)
	if err != nil {
		return nil, err
	}

	var replicasExcessCapacityReservation int32 = 2
	if numberOfZones := len(seed.GetInfo().Spec.Provider.Zones); numberOfZones > 1 {
		replicasExcessCapacityReservation = int32(numberOfZones)
	}

	return seedsystem.New(
		c,
		gardenNamespaceName,
		seedsystem.Values{
			ReserveExcessCapacity: seedsystem.ReserveExcessCapacityValues{
				Enabled:  v1beta1helper.SeedSettingExcessCapacityReservationEnabled(seed.GetInfo().Spec.Settings),
				Image:    image.String(),
				Replicas: replicasExcessCapacityReservation,
				Configs:  seed.GetInfo().Spec.Settings.ExcessCapacityReservation.Configs,
			},
		},
	), nil
}

func defaultVali(
	ctx context.Context,
	c client.Client,
	loggingConfig *config.Logging,
	gardenNamespaceName string,
	isLoggingEnabled bool,
	hvpaEnabled bool,
) (
	component.Deployer,
	error,
) {
	maintenanceBegin, maintenanceEnd := "220000-0000", "230000-0000"

	if hvpaEnabled {
		shootInfo := &corev1.ConfigMap{}
		if err := c.Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), shootInfo); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		} else {
			shootMaintenanceBegin, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceBegin"])
			if err != nil {
				return nil, err
			}

			shootMaintenanceEnd, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceEnd"])
			if err != nil {
				return nil, err
			}

			maintenanceBegin = shootMaintenanceBegin.Add(1, 0, 0).Formatted()
			maintenanceEnd = shootMaintenanceEnd.Add(1, 0, 0).Formatted()
		}
	}

	var storage *resource.Quantity
	if loggingConfig != nil && loggingConfig.Vali != nil && loggingConfig.Vali.Garden != nil {
		storage = loggingConfig.Vali.Garden.Storage
	}

	deployer, err := shared.NewVali(
		c,
		gardenNamespaceName,
		nil,
		component.ClusterTypeSeed,
		1,
		false,
		v1beta1constants.PriorityClassNameSeedSystem600,
		storage,
		"",
		hvpaEnabled,
		&hvpav1alpha1.MaintenanceTimeWindow{
			Begin: maintenanceBegin,
			End:   maintenanceEnd,
		},
	)
	if err != nil {
		return nil, err
	}

	if !isLoggingEnabled {
		return component.OpDestroy(deployer), err
	}

	return deployer, err
}

func defaultPlutono(
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	ingressHot string,
	authSecret string,
	wildcardCertName *string,
) (
	plutono.Interface,
	error,
) {
	return shared.NewPlutono(
		c,
		namespace,
		secretsManager,
		component.ClusterTypeSeed,
		1,
		authSecret,
		ingressHot,
		v1beta1constants.PriorityClassNameSeedSystem600,
		true,
		false,
		false,
		false,
		false,
		false,
		wildcardCertName,
	)
}

func defaultMonitoring(
	c client.Client,
	chartApplier kubernetes.ChartApplier,
	secretsManager secretsmanager.Interface,
	namespace string,
	seed *seedpkg.Seed,
	alertingSMTPSecret *corev1.Secret,
	globalMonitoringSecret *corev1.Secret,
	hvpaEnabled bool,
	ingressHost string,
	wildcardCertName *string,
) (
	component.Deployer,
	error,
) {
	imageAlertmanager, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlertmanager)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}
	imageConfigmapReloader, err := imagevector.ImageVector().FindImage(imagevector.ImageNameConfigmapReloader)
	if err != nil {
		return nil, err
	}
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}

	return monitoring.NewBootstrap(
		c,
		chartApplier,
		secretsManager,
		namespace,
		monitoring.ValuesBootstrap{
			AlertingSMTPSecret:                 alertingSMTPSecret,
			GlobalMonitoringSecret:             globalMonitoringSecret,
			HVPAEnabled:                        hvpaEnabled,
			ImageAlertmanager:                  imageAlertmanager.String(),
			ImageAlpine:                        imageAlpine.String(),
			ImageConfigmapReloader:             imageConfigmapReloader.String(),
			ImagePrometheus:                    imagePrometheus.String(),
			IngressHost:                        ingressHost,
			SeedName:                           seed.GetInfo().Name,
			StorageCapacityAlertmanager:        seed.GetValidVolumeSize("1Gi"),
			StorageCapacityPrometheus:          seed.GetValidVolumeSize("10Gi"),
			StorageCapacityAggregatePrometheus: seed.GetValidVolumeSize("20Gi"),
			WildcardCertName:                   wildcardCertName,
		},
	), nil
}

func defaultCachePrometheus(
	log logr.Logger,
	c client.Client,
	namespace string,
	seed *seedpkg.Seed,
) (
	component.DeployWaiter,
	error,
) {
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	return prometheus.New(log, c, namespace, prometheus.Values{
		Name:              "cache",
		Image:             imagePrometheus.String(),
		Version:           ptr.Deref(imagePrometheus.Version, "v0.0.0"),
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   resource.MustParse(seed.GetValidVolumeSize("10Gi")),
		CentralConfigs: prometheus.CentralConfigs{
			AdditionalScrapeConfigs: cacheprometheus.AdditionalScrapeConfigs(),
			ServiceMonitors:         cacheprometheus.CentralServiceMonitors(),
			PrometheusRules:         cacheprometheus.CentralPrometheusRules(),
		},
		AdditionalResources: []client.Object{cacheprometheus.NetworkPolicyToNodeExporter(namespace)},
		// TODO(rfranzke): Remove this after v1.92 has been released.
		DataMigration: prometheus.DataMigration{
			ImageAlpine:     imageAlpine.String(),
			StatefulSetName: "prometheus",
			PVCName:         "prometheus-db-prometheus-0",
		},
	}), nil
}

// getFluentBitInputsFilterAndParsers returns all fluent-bit inputs, filters and parsers for the seed
func getFluentOperatorCustomResources(
	c client.Client,
	namespace string,
	loggingEnabled bool,
	seedIsGarden bool,
	isEventLoggingEnabled bool,
) (
	deployer component.DeployWaiter,
	err error,
) {
	centralLoggingConfigurations := []component.CentralLoggingConfiguration{
		// seed system components
		extensions.CentralLoggingConfiguration,
		dependencywatchdog.CentralLoggingConfiguration,
		monitoring.CentralLoggingConfiguration,
		plutono.CentralLoggingConfiguration,
		// shoot control plane components
		clusterautoscaler.CentralLoggingConfiguration,
		vpnseedserver.CentralLoggingConfiguration,
		kubescheduler.CentralLoggingConfiguration,
		machinecontrollermanager.CentralLoggingConfiguration,
		// shoot worker components
		downloader.CentralLoggingConfiguration,
		nodeagent.CentralLoggingConfiguration,
		// shoot system components
		nodeexporter.CentralLoggingConfiguration,
		nodeproblemdetector.CentralLoggingConfiguration,
		vpnshoot.CentralLoggingConfiguration,
		coredns.CentralLoggingConfiguration,
		kubeproxy.CentralLoggingConfiguration,
		metricsserver.CentralLoggingConfiguration,
		// shoot addon components
		kubernetesdashboard.CentralLoggingConfiguration,
	}

	if !seedIsGarden {
		centralLoggingConfigurations = append(centralLoggingConfigurations, logging.GardenCentralLoggingConfigurations...)
	}
	if isEventLoggingEnabled {
		centralLoggingConfigurations = append(centralLoggingConfigurations, eventlogger.CentralLoggingConfiguration)
	}

	return shared.NewFluentOperatorCustomResources(
		c,
		namespace,
		loggingEnabled,
		"",
		centralLoggingConfigurations,
		customresources.GetDynamicClusterOutput(map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource}),
	)
}
