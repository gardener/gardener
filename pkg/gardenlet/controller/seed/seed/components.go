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

package seed

import (
	"context"

	"github.com/Masterminds/semver"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedsystem"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func defaultKubeStateMetrics(
	c client.Client,
	imageVector imagevector.ImageVector,
	seedVersion *semver.Version,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imageVector.FindImage(images.ImageNameKubeStateMetrics, imagevector.TargetVersion(seedVersion.String()))
	if err != nil {
		return nil, err
	}

	return kubestatemetrics.New(c, gardenNamespaceName, nil, kubestatemetrics.Values{
		ClusterType: component.ClusterTypeSeed,
		Image:       image.String(),
		Replicas:    1,
	}), nil
}

func defaultIstio(
	seedClient client.Client,
	imageVector imagevector.ImageVector,
	chartRenderer chartrenderer.Interface,
	seed *seedpkg.Seed,
	conf *config.GardenletConfiguration,
	sniEnabledOrInUse bool,
) (
	component.DeployWaiter,
	error,
) {
	var (
		minReplicas *int
		maxReplicas *int
		seedObj     = seed.GetInfo()
	)

	istiodImage, err := imageVector.FindImage(images.ImageNameIstioIstiod)
	if err != nil {
		return nil, err
	}

	igwImage, err := imageVector.FindImage(images.ImageNameIstioProxy)
	if err != nil {
		return nil, err
	}

	if len(seedObj.Spec.Provider.Zones) > 1 {
		// Each availability zone should have at least 2 replicas as on some infrastructures each
		// zonal load balancer is exposed individually via its own IP address. Therefore, having
		// just one replica may negatively affect availability.
		minReplicas = pointer.Int(len(seedObj.Spec.Provider.Zones) * 2)
		// The default configuration without availability zones has 5 as the maximum amount of
		// replicas, which apparently works in all known Gardener scenarios. Reducing it to less
		// per zone gives some room for autoscaling while it is assumed to never reach the maximum.
		maxReplicas = pointer.Int(len(seedObj.Spec.Provider.Zones) * 4)
	}

	defaultIngressGatewayConfig := istio.IngressGatewayValues{
		TrustDomain:           gardencorev1beta1.DefaultDomain,
		Image:                 igwImage.String(),
		IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
		Annotations:           seed.GetLoadBalancerServiceAnnotations(),
		ExternalTrafficPolicy: seed.GetLoadBalancerServiceExternalTrafficPolicy(),
		MinReplicas:           minReplicas,
		MaxReplicas:           maxReplicas,
		Ports:                 []corev1.ServicePort{},
		LoadBalancerIP:        conf.SNI.Ingress.ServiceExternalIP,
		Labels:                operation.GetIstioZoneLabels(conf.SNI.Ingress.Labels, nil),
		Namespace:             *conf.SNI.Ingress.Namespace,
		ProxyProtocolEnabled:  gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI),
		VPNEnabled:            true,
	}

	// even if SNI is being disabled, the existing ports must stay the same
	// until all APIServer SNI resources are removed.
	if sniEnabledOrInUse {
		defaultIngressGatewayConfig.Ports = append(
			defaultIngressGatewayConfig.Ports,
			corev1.ServicePort{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt(8443)},
			corev1.ServicePort{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
			corev1.ServicePort{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt(vpnseedserver.GatewayPort)},
		)
	}

	istioIngressGateway := []istio.IngressGatewayValues{
		defaultIngressGatewayConfig,
	}

	// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
	if len(seedObj.Spec.Provider.Zones) > 1 {
		for _, zone := range seedObj.Spec.Provider.Zones {
			namespace := operation.GetIstioNamespaceForZone(*conf.SNI.Ingress.Namespace, zone)

			istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{
				TrustDomain:           gardencorev1beta1.DefaultDomain,
				Image:                 igwImage.String(),
				IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
				Annotations:           seed.GetZonalLoadBalancerServiceAnnotations(zone),
				ExternalTrafficPolicy: seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
				Ports:                 defaultIngressGatewayConfig.Ports,
				// LoadBalancerIP can currently not be provided for automatic ingress gateways
				Labels:               operation.GetIstioZoneLabels(defaultIngressGatewayConfig.Labels, &zone),
				Zones:                []string{zone},
				Namespace:            namespace,
				ProxyProtocolEnabled: gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI),
				VPNEnabled:           true,
			})
		}
	}

	// Add for each ExposureClass handler in the config an own Ingress Gateway and Proxy Gateway.
	for _, handler := range conf.ExposureClassHandlers {
		istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{
			TrustDomain:           gardencorev1beta1.DefaultDomain,
			Image:                 igwImage.String(),
			IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
			Annotations:           utils.MergeStringMaps(seed.GetLoadBalancerServiceAnnotations(), handler.LoadBalancerService.Annotations),
			ExternalTrafficPolicy: seed.GetLoadBalancerServiceExternalTrafficPolicy(),
			MinReplicas:           minReplicas,
			MaxReplicas:           maxReplicas,
			Ports:                 defaultIngressGatewayConfig.Ports,
			LoadBalancerIP:        handler.SNI.Ingress.ServiceExternalIP,
			Labels:                operation.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), nil),
			Namespace:             *handler.SNI.Ingress.Namespace,
			ProxyProtocolEnabled:  gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI),
			VPNEnabled:            true,
		})

		// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
		if len(seedObj.Spec.Provider.Zones) > 1 {
			for _, zone := range seedObj.Spec.Provider.Zones {
				namespace := operation.GetIstioNamespaceForZone(*handler.SNI.Ingress.Namespace, zone)

				istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{
					TrustDomain:           gardencorev1beta1.DefaultDomain,
					Image:                 igwImage.String(),
					IstiodNamespace:       v1beta1constants.IstioSystemNamespace,
					Annotations:           utils.MergeStringMaps(handler.LoadBalancerService.Annotations, seed.GetZonalLoadBalancerServiceAnnotations(zone)),
					ExternalTrafficPolicy: seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone),
					Ports:                 defaultIngressGatewayConfig.Ports,
					// LoadBalancerIP can currently not be provided for automatic ingress gateways
					Labels:               operation.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), &zone),
					Zones:                []string{zone},
					Namespace:            namespace,
					ProxyProtocolEnabled: gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI),
					VPNEnabled:           true,
				})
			}
		}
	}

	return istio.NewIstio(
		seedClient,
		chartRenderer,
		istio.Values{
			Istiod: istio.IstiodValues{
				Enabled:     true,
				Image:       istiodImage.String(),
				Namespace:   v1beta1constants.IstioSystemNamespace,
				TrustDomain: gardencorev1beta1.DefaultDomain,
				Zones:       seedObj.Spec.Provider.Zones,
			},
			IngressGateway: istioIngressGateway,
		},
	), nil
}

func defaultNetworkPolicies(
	c client.Client,
	gardenNamespaceName string,
) (
	component.DeployWaiter,
	error,
) {
	return networkpolicies.NewBootstrapper(c, gardenNamespaceName), nil
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
	ctx context.Context,
	c client.Client,
	seedVersion *semver.Version,
	imageVector imagevector.ImageVector,
	gardenNamespaceName string,
) (
	extAuthzServer component.DeployWaiter,
	err error,
) {
	image, err := imageVector.FindImage(images.ImageNameExtAuthzServer, imagevector.RuntimeVersion(seedVersion.String()), imagevector.TargetVersion(seedVersion.String()))
	if err != nil {
		return nil, err
	}

	vpnAuthzServer := vpnauthzserver.New(
		c,
		gardenNamespaceName,
		image.String(),
		seedVersion,
	)

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		return vpnAuthzServer, nil
	}

	hasVPNSeedDeployments, err := kubernetesutils.ResourcesExist(ctx, c, appsv1.SchemeGroupVersion.WithKind("DeploymentList"), client.MatchingLabels(map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameVPNSeedServer}))
	if err != nil {
		return nil, err
	}
	if hasVPNSeedDeployments {
		// Even though the ManagedIstio feature gate is turned off, there are still shoots which have not been reconciled yet.
		// Thus, we cannot destroy the ext-authz-server.
		return component.NoOp(), nil
	}

	return component.OpDestroyWithoutWait(vpnAuthzServer), nil
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
