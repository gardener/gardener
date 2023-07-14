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

	"github.com/Masterminds/semver"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/monitoring"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/seedsystem"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

func defaultIstio(
	seedClient client.Client,
	imageVector imagevector.ImageVector,
	chartRenderer chartrenderer.Interface,
	seed *seedpkg.Seed,
	conf *config.GardenletConfiguration,
	isGardenCluster bool,
) (
	component.DeployWaiter,
	error,
) {
	var (
		seedObj = seed.GetInfo()
		labels  = shared.GetIstioZoneLabels(conf.SNI.Ingress.Labels, nil)
	)

	istioDeployer, err := shared.NewIstio(
		seedClient,
		imageVector,
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
			{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt(8443)},
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
			{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt(vpnseedserver.GatewayPort)},
		},
		true,
		true,
		seedObj.Spec.Provider.Zones,
	)
	if err != nil {
		return nil, err
	}

	// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
	if len(seedObj.Spec.Provider.Zones) > 1 {
		for _, zone := range seedObj.Spec.Provider.Zones {
			if err := shared.AddIstioIngressGateway(
				istioDeployer,
				shared.GetIstioNamespaceForZone(*conf.SNI.Ingress.Namespace, zone),
				seed.GetZonalLoadBalancerServiceAnnotations(zone),
				shared.GetIstioZoneLabels(labels, &zone),
				seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
				nil,
				&zone,
			); err != nil {
				return nil, err
			}
		}
	}

	// Add for each ExposureClass handler in the config an own Ingress Gateway and Proxy Gateway.
	for _, handler := range conf.ExposureClassHandlers {
		if err := shared.AddIstioIngressGateway(
			istioDeployer,
			*handler.SNI.Ingress.Namespace,
			// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
			utils.MergeStringMaps(seed.GetLoadBalancerServiceAnnotations(), handler.LoadBalancerService.Annotations),
			shared.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), nil),
			seed.GetLoadBalancerServiceExternalTrafficPolicy(),
			handler.SNI.Ingress.ServiceExternalIP,
			nil,
		); err != nil {
			return nil, err
		}

		// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
		if len(seedObj.Spec.Provider.Zones) > 1 {
			for _, zone := range seedObj.Spec.Provider.Zones {
				if err := shared.AddIstioIngressGateway(
					istioDeployer,
					shared.GetIstioNamespaceForZone(*handler.SNI.Ingress.Namespace, zone),
					// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
					utils.MergeStringMaps(seed.GetZonalLoadBalancerServiceAnnotations(zone), handler.LoadBalancerService.Annotations),
					shared.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), &zone),
					seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
					nil,
					&zone,
				); err != nil {
					return nil, err
				}
			}
		}
	}

	return istioDeployer, nil
}

func defaultDependencyWatchdogs(
	c client.Client,
	seedVersion *semver.Version,
	imageVector imagevector.ImageVector,
	seedSettings *gardencorev1beta1.SeedSettings,
	gardenNamespaceName string,
) (
	dwdWeeder component.DeployWaiter,
	dwdProber component.DeployWaiter,
	err error,
) {
	image, err := imageVector.FindImage(images.ImageNameDependencyWatchdog, imagevector.RuntimeVersion(seedVersion.String()), imagevector.TargetVersion(seedVersion.String()))
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
	imageVector imagevector.ImageVector,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imageVector.FindImage(images.ImageNameExtAuthzServer, imagevector.RuntimeVersion(seedVersion.String()), imagevector.TargetVersion(seedVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpnauthzserver.New(
		c,
		gardenNamespaceName,
		image.String(),
	), nil
}

func defaultSystem(
	c client.Client,
	seed *seedpkg.Seed,
	imageVector imagevector.ImageVector,
	reserveExcessCapacity bool,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imageVector.FindImage(images.ImageNamePauseContainer)
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
				Enabled:  reserveExcessCapacity,
				Image:    image.String(),
				Replicas: replicasExcessCapacityReservation,
			},
		},
	), nil
}

func defaultVali(
	ctx context.Context,
	c client.Client,
	imageVector imagevector.ImageVector,
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
		imageVector,
		nil,
		component.ClusterTypeSeed,
		1,
		false,
		v1beta1constants.PriorityClassNameSeedSystem600,
		storage,
		"",
		false,
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
	imageVector imagevector.ImageVector,
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
		imageVector,
		secretsManager,
		authSecret,
		component.ClusterTypeSeed,
		ingressHot,
		true,
		false,
		false,
		false,
		false,
		v1beta1constants.PriorityClassNameSeedSystem600,
		1,
		wildcardCertName,
		false,
	)
}

func defaultMonitoring(
	c client.Client,
	namespace string,
	globalMonitoringSecret *corev1.Secret,
) (
	component.Deployer,
	error,
) {
	return monitoring.New(
		c,
		namespace,
		monitoring.Values{
			GlobalMonitoringSecret: globalMonitoringSecret,
		},
	), nil
}
