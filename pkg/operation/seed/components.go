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
	"fmt"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/hvpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedsystem"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	"github.com/Masterminds/semver"
	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func defaultEtcdDruid(
	c client.Client,
	seedVersion string,
	conf *config.GardenletConfiguration,
	imageVector imagevector.ImageVector,
	imageVectorOverwrites map[string]string,
) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNameEtcdDruid, imagevector.RuntimeVersion(seedVersion), imagevector.TargetVersion(seedVersion))
	if err != nil {
		return nil, err
	}

	var imageVectorOverwrite *string
	if val, ok := imageVectorOverwrites[etcd.Druid]; ok {
		imageVectorOverwrite = &val
	}

	return etcd.NewBootstrapper(c, v1beta1constants.GardenNamespace, conf, image.String(), imageVectorOverwrite), nil
}

func defaultKubeStateMetrics(c client.Client, imageVector imagevector.ImageVector) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNameKubeStateMetrics)
	if err != nil {
		return nil, err
	}

	return kubestatemetrics.New(c, v1beta1constants.GardenNamespace, nil, kubestatemetrics.Values{
		ClusterType: component.ClusterTypeSeed,
		Image:       image.String(),
		Replicas:    1,
	}), nil
}

func defaultKubeScheduler(c client.Client, imageVector imagevector.ImageVector, secretsManager secretsmanager.Interface, kubernetesVersion *semver.Version) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNameKubeScheduler, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	return gardenerkubescheduler.Bootstrap(c, secretsManager, v1beta1constants.GardenNamespace, image, kubernetesVersion)
}

func defaultGardenerSeedAdmissionController(c client.Client, imageVector imagevector.ImageVector, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNameGardenerSeedAdmissionController)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	return seedadmissioncontroller.New(c, v1beta1constants.GardenNamespace, secretsManager, image.String()), nil
}

func defaultGardenerResourceManager(c client.Client, seedClientVersion string, imageVector imagevector.ImageVector, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	return resourcemanager.New(c, v1beta1constants.GardenNamespace, secretsManager, image.String(), resourcemanager.Values{
		ConcurrentSyncs:                      pointer.Int32(20),
		MaxConcurrentTokenInvalidatorWorkers: pointer.Int32(5),
		MaxConcurrentRootCAPublisherWorkers:  pointer.Int32(5),
		HealthSyncPeriod:                     pointer.Duration(time.Minute),
		Replicas:                             pointer.Int32(3),
		ResourceClass:                        pointer.String(v1beta1constants.SeedResourceManagerClass),
		SecretNameServerCA:                   v1beta1constants.SecretNameCASeed,
		SyncPeriod:                           pointer.Duration(time.Hour),
		Version:                              semver.MustParse(seedClientVersion),
		VPA: &resourcemanager.VPAConfig{
			MinAllowed: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
		DefaultSeccompProfileEnabled: gardenletfeatures.FeatureGate.Enabled(features.DefaultSeccompProfile),
	}), nil
}

func defaultIstio(ctx context.Context,
	seedClient client.Client,
	imageVector imagevector.ImageVector,
	chartRenderer chartrenderer.Interface,
	seed *Seed,
	conf *config.GardenletConfiguration,
	sniEnabledOrInUse bool,
) (
	component.DeployWaiter,
	error,
) {
	istiodImage, err := imageVector.FindImage(images.ImageNameIstioIstiod)
	if err != nil {
		return nil, err
	}

	igwImage, err := imageVector.FindImage(images.ImageNameIstioProxy)
	if err != nil {
		return nil, err
	}

	defaultIngressGatewayConfig := istio.IngressValues{
		TrustDomain:     gardencorev1beta1.DefaultDomain,
		Image:           igwImage.String(),
		IstiodNamespace: v1beta1constants.IstioSystemNamespace,
		Annotations:     seed.LoadBalancerServiceAnnotations,
		Ports:           []corev1.ServicePort{},
		LoadBalancerIP:  conf.SNI.Ingress.ServiceExternalIP,
		Labels:          conf.SNI.Ingress.Labels,
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

	istioIngressGateway := []istio.IngressGateway{{
		Values:    defaultIngressGatewayConfig,
		Namespace: *conf.SNI.Ingress.Namespace,
	}}

	istioProxyGateway := []istio.ProxyProtocol{{
		Values: istio.ProxyValues{
			Labels: conf.SNI.Ingress.Labels,
		},
		Namespace: *conf.SNI.Ingress.Namespace,
	}}

	// Add for each ExposureClass handler in the config an own Ingress Gateway and Proxy Gateway.
	for _, handler := range conf.ExposureClassHandlers {
		istioIngressGateway = append(istioIngressGateway, istio.IngressGateway{
			Values: istio.IngressValues{
				TrustDomain:     gardencorev1beta1.DefaultDomain,
				Image:           igwImage.String(),
				IstiodNamespace: v1beta1constants.IstioSystemNamespace,
				Annotations:     utils.MergeStringMaps(seed.LoadBalancerServiceAnnotations, handler.LoadBalancerService.Annotations),
				Ports:           defaultIngressGatewayConfig.Ports,
				LoadBalancerIP:  handler.SNI.Ingress.ServiceExternalIP,
				Labels:          gutil.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name),
			},
			Namespace: *handler.SNI.Ingress.Namespace,
		})

		istioProxyGateway = append(istioProxyGateway, istio.ProxyProtocol{
			Values: istio.ProxyValues{
				Labels: gutil.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name),
			},
			Namespace: *handler.SNI.Ingress.Namespace,
		})
	}

	if !gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		istioProxyGateway = nil
	}

	return istio.NewIstio(
		seedClient,
		chartRenderer,
		istio.IstiodValues{
			TrustDomain: gardencorev1beta1.DefaultDomain,
			Image:       istiodImage.String(),
		},
		v1beta1constants.IstioSystemNamespace,
		istioIngressGateway,
		istioProxyGateway,
	), nil
}

func defaultNetworkPolicies(c client.Client, seed *gardencorev1beta1.Seed, sniEnabled bool) (component.DeployWaiter, error) {
	networks := []string{seed.Spec.Networks.Pods, seed.Spec.Networks.Services}
	if v := seed.Spec.Networks.Nodes; v != nil {
		networks = append(networks, *v)
	}
	privateNetworkPeers, err := networkpolicies.ToNetworkPolicyPeersWithExceptions(networkpolicies.AllPrivateNetworkBlocks(), networks...)
	if err != nil {
		return nil, err
	}

	_, seedServiceCIDR, err := net.ParseCIDR(seed.Spec.Networks.Services)
	if err != nil {
		return nil, err
	}
	seedDNSServerAddress, err := common.ComputeOffsetIP(seedServiceCIDR, 10)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP: %v", err)
	}

	return networkpolicies.NewBootstrapper(c, v1beta1constants.GardenNamespace, networkpolicies.GlobalValues{
		SNIEnabled:           sniEnabled,
		DenyAllTraffic:       false,
		PrivateNetworkPeers:  privateNetworkPeers,
		NodeLocalIPVSAddress: pointer.String(nodelocaldns.IPVSAddress),
		DNSServerAddress:     pointer.String(seedDNSServerAddress.String()),
	}), nil
}

func defaultDependencyWatchdogs(
	c client.Client,
	seedVersion string,
	imageVector imagevector.ImageVector,
	seedSettings *gardencorev1beta1.SeedSettings,
) (
	dwdEndpoint component.DeployWaiter,
	dwdProbe component.DeployWaiter,
	err error,
) {
	image, err := imageVector.FindImage(images.ImageNameDependencyWatchdog, imagevector.RuntimeVersion(seedVersion), imagevector.TargetVersion(seedVersion))
	if err != nil {
		return nil, nil, err
	}

	var (
		dwdEndpointValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleEndpoint, Image: image.String()}
		dwdProbeValues    = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProbe, Image: image.String()}
	)

	dwdEndpoint = component.OpDestroy(dependencywatchdog.NewBootstrapper(c, v1beta1constants.GardenNamespace, dwdEndpointValues))
	dwdProbe = component.OpDestroy(dependencywatchdog.NewBootstrapper(c, v1beta1constants.GardenNamespace, dwdProbeValues))

	if gardencorev1beta1helper.SeedSettingDependencyWatchdogEndpointEnabled(seedSettings) {
		// Fetch component-specific dependency-watchdog configuration
		var (
			dependencyWatchdogEndpointConfigurationFuncs = []dependencywatchdog.EndpointConfigurationFunc{
				func() (map[string]restarterapi.Service, error) {
					return etcd.DependencyWatchdogEndpointConfiguration(v1beta1constants.ETCDRoleMain)
				},
				kubeapiserver.DependencyWatchdogEndpointConfiguration,
			}
			dependencyWatchdogEndpointConfigurations = restarterapi.ServiceDependants{
				Services: make(map[string]restarterapi.Service, len(dependencyWatchdogEndpointConfigurationFuncs)),
			}
		)

		for _, componentFn := range dependencyWatchdogEndpointConfigurationFuncs {
			dwdConfig, err := componentFn()
			if err != nil {
				return nil, nil, err
			}
			for k, v := range dwdConfig {
				dependencyWatchdogEndpointConfigurations.Services[k] = v
			}
		}

		dwdEndpointValues.ValuesEndpoint = dependencywatchdog.ValuesEndpoint{ServiceDependants: dependencyWatchdogEndpointConfigurations}
		dwdEndpoint = dependencywatchdog.NewBootstrapper(c, v1beta1constants.GardenNamespace, dwdEndpointValues)
	}

	if gardencorev1beta1helper.SeedSettingDependencyWatchdogProbeEnabled(seedSettings) {
		// Fetch component-specific dependency-watchdog configuration
		var (
			dependencyWatchdogProbeConfigurationFuncs = []dependencywatchdog.ProbeConfigurationFunc{
				kubeapiserver.DependencyWatchdogProbeConfiguration,
			}
			dependencyWatchdogProbeConfigurations = scalerapi.ProbeDependantsList{
				Probes: make([]scalerapi.ProbeDependants, 0, len(dependencyWatchdogProbeConfigurationFuncs)),
			}
		)

		for _, componentFn := range dependencyWatchdogProbeConfigurationFuncs {
			dwdConfig, err := componentFn()
			if err != nil {
				return nil, nil, err
			}
			dependencyWatchdogProbeConfigurations.Probes = append(dependencyWatchdogProbeConfigurations.Probes, dwdConfig...)
		}

		dwdProbeValues.ValuesProbe = dependencywatchdog.ValuesProbe{ProbeDependantsList: dependencyWatchdogProbeConfigurations}
		dwdProbe = dependencywatchdog.NewBootstrapper(c, v1beta1constants.GardenNamespace, dwdProbeValues)
	}

	return
}

func defaultHVPA(c client.Client, imageVector imagevector.ImageVector, enabled bool) (deployer component.DeployWaiter, err error) {
	image, err := imageVector.FindImage(images.ImageNameHvpaController)
	if err != nil {
		return nil, err
	}

	deployer = hvpa.New(
		c,
		v1beta1constants.GardenNamespace,
		hvpa.Values{
			Image: image.String(),
		},
	)

	if !enabled {
		deployer = component.OpDestroy(deployer)
	}

	return deployer, nil
}

func defaultVerticalPodAutoscaler(c client.Client, imageVector imagevector.ImageVector, secretsManager secretsmanager.Interface, enabled bool) (component.DeployWaiter, error) {
	imageAdmissionController, err := imageVector.FindImage(images.ImageNameVpaAdmissionController)
	if err != nil {
		return nil, err
	}

	imageExporter, err := imageVector.FindImage(images.ImageNameVpaExporter)
	if err != nil {
		return nil, err
	}

	imageRecommender, err := imageVector.FindImage(images.ImageNameVpaRecommender)
	if err != nil {
		return nil, err
	}

	imageUpdater, err := imageVector.FindImage(images.ImageNameVpaUpdater)
	if err != nil {
		return nil, err
	}

	return vpa.New(
		c,
		v1beta1constants.GardenNamespace,
		secretsManager,
		vpa.Values{
			ClusterType:        component.ClusterTypeSeed,
			Enabled:            enabled,
			SecretNameServerCA: v1beta1constants.SecretNameCASeed,
			AdmissionController: vpa.ValuesAdmissionController{
				Image:    imageAdmissionController.String(),
				Replicas: 1,
			},
			Exporter: vpa.ValuesExporter{
				Image: imageExporter.String(),
			},
			Recommender: vpa.ValuesRecommender{
				Image:                        imageRecommender.String(),
				RecommendationMarginFraction: pointer.Float64(0.05),
				Replicas:                     1,
			},
			Updater: vpa.ValuesUpdater{
				EvictionTolerance:      pointer.Float64(1.0),
				EvictAfterOOMThreshold: &metav1.Duration{Duration: 48 * time.Hour},
				Image:                  imageUpdater.String(),
				Replicas:               1,
			},
		},
	), nil
}

func defaultVPNAuthzServer(
	ctx context.Context,
	c client.Client,
	seedVersion string,
	imageVector imagevector.ImageVector,
) (
	extAuthzServer component.DeployWaiter,
	err error,
) {
	image, err := imageVector.FindImage(images.ImageNameExtAuthzServer, imagevector.RuntimeVersion(seedVersion), imagevector.TargetVersion(seedVersion))
	if err != nil {
		return nil, err
	}

	vpnAuthzServer := vpnauthzserver.New(
		c,
		v1beta1constants.GardenNamespace,
		image.String(),
		3,
	)

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		return vpnAuthzServer, nil
	}

	vpnSeedDeployments := &metav1.PartialObjectMetadataList{}
	vpnSeedDeployments.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DeploymentList"))

	if err := c.List(ctx, vpnSeedDeployments, client.MatchingLabels(map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameVPNSeedServer}), client.Limit(1)); err != nil {
		return nil, err
	}

	// Even though the ManagedIstio feature gate is turned off, there are still shoots which have not been reconciled yet.
	// Thus, we cannot destroy the ext-authz-server.
	if len(vpnSeedDeployments.Items) > 0 {
		return component.NoOp(), nil
	}

	return component.OpDestroy(vpnAuthzServer), nil
}

func defaultSystem(c client.Client, imageVector imagevector.ImageVector, reserveExcessCapacity bool) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(images.ImageNamePauseContainer)
	if err != nil {
		return nil, err
	}

	return seedsystem.New(
		c,
		v1beta1constants.GardenNamespace,
		seedsystem.Values{
			ReserveExcessCapacity: seedsystem.ReserveExcessCapacityValues{
				Enabled: reserveExcessCapacity,
				Image:   image.String(),
			},
		},
	), nil
}
