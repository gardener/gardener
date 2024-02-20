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

	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	"github.com/gardener/gardener/pkg/component/kubernetesdashboard"
	"github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/component/logging"
	"github.com/gardener/gardener/pkg/component/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/metricsserver"
	"github.com/gardener/gardener/pkg/component/monitoring"
	"github.com/gardener/gardener/pkg/component/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheus"
	aggregateprometheus "github.com/gardener/gardener/pkg/component/monitoring/prometheus/aggregate"
	cacheprometheus "github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	seedprometheus "github.com/gardener/gardener/pkg/component/monitoring/prometheus/seed"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/component/nodeexporter"
	"github.com/gardener/gardener/pkg/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/seedsystem"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/component/vpnshoot"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

type components struct {
	machineCRD    component.Deployer
	extensionCRD  component.Deployer
	etcdCRD       component.Deployer
	istioCRD      component.Deployer
	vpaCRD        component.Deployer
	hvpaCRD       component.Deployer
	fluentCRD     component.Deployer
	prometheusCRD component.Deployer

	clusterIdentity          component.DeployWaiter
	gardenerResourceManager  component.DeployWaiter
	system                   component.DeployWaiter
	istio                    component.DeployWaiter
	istioDefaultLabels       map[string]string
	istioDefaultNamespace    string
	nginxIngressController   component.DeployWaiter
	verticalPodAutoscaler    component.DeployWaiter
	hvpaController           component.DeployWaiter
	etcdDruid                component.DeployWaiter
	clusterAutoscaler        component.DeployWaiter
	machineControllerManager component.DeployWaiter
	dwdWeeder                component.DeployWaiter
	dwdProber                component.DeployWaiter
	vpnAuthzServer           component.DeployWaiter

	kubeAPIServerService component.Deployer
	kubeAPIServerIngress component.Deployer
	ingressDNSRecord     component.DeployWaiter

	monitoring                    component.Deployer
	fluentOperator                component.DeployWaiter
	fluentBit                     component.DeployWaiter
	fluentOperatorCustomResources component.DeployWaiter
	plutono                       plutono.Interface
	vali                          component.Deployer
	kubeStateMetrics              component.DeployWaiter
	prometheusOperator            component.DeployWaiter
	cachePrometheus               component.DeployWaiter
	seedPrometheus                component.DeployWaiter
	aggregatePrometheus           component.DeployWaiter
	alertManager                  component.DeployWaiter
}

func (r *Reconciler) instantiateComponents(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	secretsManager secretsmanager.Interface,
	seedIsGarden bool,
	globalMonitoringSecretSeed *corev1.Secret,
	alertingSMTPSecret *corev1.Secret,
	wildCardCertSecret *corev1.Secret,
) (
	c components,
	err error,
) {
	// crds
	c.machineCRD = machinecontrollermanager.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	c.extensionCRD = extensioncrds.NewCRD(r.SeedClientSet.Applier())
	c.etcdCRD = etcd.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	c.istioCRD = istio.NewCRD(r.SeedClientSet.ChartApplier())
	c.vpaCRD = vpa.NewCRD(r.SeedClientSet.Applier(), nil)
	c.hvpaCRD = hvpa.NewCRD(r.SeedClientSet.Applier())
	if !hvpaEnabled() {
		c.hvpaCRD = component.OpDestroy(c.hvpaCRD)
	}
	c.fluentCRD = fluentoperator.NewCRDs(r.SeedClientSet.Applier())
	c.prometheusCRD = prometheusoperator.NewCRDs(r.SeedClientSet.Applier())

	// seed system components
	c.clusterIdentity = r.newClusterIdentity(seed.GetInfo())
	c.gardenerResourceManager, err = r.newGardenerResourceManager(seed.GetInfo(), secretsManager)
	if err != nil {
		return
	}
	c.system, err = r.newSystem(seed.GetInfo())
	if err != nil {
		return
	}
	c.istio, c.istioDefaultLabels, c.istioDefaultNamespace, err = r.newIstio(ctx, seed, seedIsGarden)
	if err != nil {
		return
	}
	c.nginxIngressController, err = r.newNginxIngressController(seed, c.istioDefaultLabels)
	if err != nil {
		return
	}
	c.verticalPodAutoscaler, err = r.newVerticalPodAutoscaler(seed.GetInfo().Spec.Settings, secretsManager)
	if err != nil {
		return
	}
	c.hvpaController, err = r.newHVPA()
	if err != nil {
		return
	}
	c.etcdDruid, err = r.newEtcdDruid()
	if err != nil {
		return
	}
	c.clusterAutoscaler = r.newClusterAutoscaler()
	c.machineControllerManager = r.newMachineControllerManager()
	c.dwdWeeder, c.dwdProber, err = r.newDependencyWatchdogs(seed.GetInfo().Spec.Settings)
	if err != nil {
		return
	}
	c.vpnAuthzServer, err = r.newVPNAuthzServer()
	if err != nil {
		return
	}

	c.kubeAPIServerService = r.newKubeAPIServerService(wildCardCertSecret)
	c.kubeAPIServerIngress = r.newKubeAPIServerIngress(seed, wildCardCertSecret)
	c.ingressDNSRecord, err = r.newIngressDNSRecord(ctx, log, seed, "")
	if err != nil {
		return
	}

	// observability components
	c.fluentOperator, err = r.newFluentOperator()
	if err != nil {
		return
	}
	c.fluentBit, err = r.newFluentBit()
	if err != nil {
		return
	}
	c.fluentOperatorCustomResources, err = r.newFluentCustomResources(seedIsGarden)
	if err != nil {
		return
	}
	c.vali, err = r.newVali(ctx)
	if err != nil {
		return
	}
	c.plutono, err = r.newPlutono(seed, secretsManager, globalMonitoringSecretSeed, wildCardCertSecret)
	if err != nil {
		return
	}
	c.monitoring, err = r.newMonitoring(secretsManager, seed, globalMonitoringSecretSeed, seed.GetIngressFQDN("p-seed"), wildCardCertSecret)
	if err != nil {
		return
	}
	c.kubeStateMetrics, err = r.newKubeStateMetrics()
	if err != nil {
		return
	}
	c.prometheusOperator, err = r.newPrometheusOperator()
	if err != nil {
		return
	}
	c.cachePrometheus, err = r.newCachePrometheus(log, seed)
	if err != nil {
		return
	}
	c.alertManager, err = r.newAlertmanager(log, seed, alertingSMTPSecret)
	if err != nil {
		return
	}
	c.seedPrometheus, err = r.newSeedPrometheus(log, seed)
	if err != nil {
		return
	}
	c.aggregatePrometheus, err = r.newAggregatePrometheus(log, seed, secretsManager, globalMonitoringSecretSeed, wildCardCertSecret, alertingSMTPSecret)
	if err != nil {
		return
	}

	return c, nil
}

func (r *Reconciler) newGardenerResourceManager(seed *gardencorev1beta1.Seed, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
	if nodeToleration := r.Config.NodeToleration; nodeToleration != nil {
		defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
	}

	var additionalNetworkPolicyNamespaceSelectors []metav1.LabelSelector
	if config := r.Config.Controllers.NetworkPolicy; config != nil {
		additionalNetworkPolicyNamespaceSelectors = config.AdditionalNamespaceSelectors
	}

	return sharedcomponent.NewRuntimeGardenerResourceManager(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		secretsManager,
		r.Config.LogLevel, r.Config.LogFormat,
		v1beta1constants.SecretNameCASeed,
		v1beta1constants.PriorityClassNameSeedSystemCritical,
		defaultNotReadyTolerationSeconds,
		defaultUnreachableTolerationSeconds,
		features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		v1beta1helper.SeedSettingTopologyAwareRoutingEnabled(seed.Spec.Settings),
		additionalNetworkPolicyNamespaceSelectors,
		seed.Spec.Provider.Zones,
	)
}

func (r *Reconciler) newIstio(ctx context.Context, seed *seedpkg.Seed, isGardenCluster bool) (component.DeployWaiter, map[string]string, string, error) {
	labels := sharedcomponent.GetIstioZoneLabels(r.Config.SNI.Ingress.Labels, nil)

	istioDeployer, err := sharedcomponent.NewIstio(
		ctx,
		r.SeedClientSet.Client(),
		r.SeedClientSet.ChartRenderer(),
		"",
		*r.Config.SNI.Ingress.Namespace,
		v1beta1constants.PriorityClassNameSeedSystemCritical,
		!isGardenCluster,
		labels,
		gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-"+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port),
		seed.GetLoadBalancerServiceAnnotations(),
		seed.GetLoadBalancerServiceExternalTrafficPolicy(),
		r.Config.SNI.Ingress.ServiceExternalIP,
		[]corev1.ServicePort{
			{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt32(8443)},
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt32(9443)},
			{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt32(vpnseedserver.GatewayPort)},
		},
		true,
		true,
		seed.GetInfo().Spec.Provider.Zones,
		seed.IsDualStack(),
	)
	if err != nil {
		return nil, nil, "", err
	}

	// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
	if len(seed.GetInfo().Spec.Provider.Zones) > 1 {
		for _, zone := range seed.GetInfo().Spec.Provider.Zones {
			if err := sharedcomponent.AddIstioIngressGateway(
				ctx,
				r.SeedClientSet.Client(),
				istioDeployer,
				sharedcomponent.GetIstioNamespaceForZone(*r.Config.SNI.Ingress.Namespace, zone),
				seed.GetZonalLoadBalancerServiceAnnotations(zone),
				sharedcomponent.GetIstioZoneLabels(labels, &zone),
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
	for _, handler := range r.Config.ExposureClassHandlers {
		if err := sharedcomponent.AddIstioIngressGateway(
			ctx,
			r.SeedClientSet.Client(),
			istioDeployer,
			*handler.SNI.Ingress.Namespace,
			// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
			utils.MergeStringMaps(seed.GetLoadBalancerServiceAnnotations(), handler.LoadBalancerService.Annotations),
			sharedcomponent.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), nil),
			seed.GetLoadBalancerServiceExternalTrafficPolicy(),
			handler.SNI.Ingress.ServiceExternalIP,
			nil,
			seed.IsDualStack(),
		); err != nil {
			return nil, nil, "", err
		}

		// Automatically create ingress gateways for single-zone control planes on multi-zonal seeds
		if len(seed.GetInfo().Spec.Provider.Zones) > 1 {
			for _, zone := range seed.GetInfo().Spec.Provider.Zones {
				if err := sharedcomponent.AddIstioIngressGateway(
					ctx,
					r.SeedClientSet.Client(),
					istioDeployer,
					sharedcomponent.GetIstioNamespaceForZone(*handler.SNI.Ingress.Namespace, zone),
					// handler.LoadBalancerService.Annotations must put last to override non-exposure class related keys.
					utils.MergeStringMaps(seed.GetZonalLoadBalancerServiceAnnotations(zone), handler.LoadBalancerService.Annotations),
					sharedcomponent.GetIstioZoneLabels(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(handler.SNI.Ingress.Labels, handler.Name), &zone),
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

func (r *Reconciler) newDependencyWatchdogs(seedSettings *gardencorev1beta1.SeedSettings) (dwdWeeder component.DeployWaiter, dwdProber component.DeployWaiter, err error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameDependencyWatchdog, imagevectorutils.RuntimeVersion(r.SeedVersion.String()), imagevectorutils.TargetVersion(r.SeedVersion.String()))
	if err != nil {
		return nil, nil, err
	}

	var (
		dwdWeederValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleWeeder, Image: image.String(), KubernetesVersion: r.SeedVersion}
		dwdProberValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProber, Image: image.String(), KubernetesVersion: r.SeedVersion}
	)

	dwdWeeder = component.OpDestroyWithoutWait(dependencywatchdog.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace, dwdWeederValues))
	dwdProber = component.OpDestroyWithoutWait(dependencywatchdog.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace, dwdProberValues))

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
		dwdWeeder = dependencywatchdog.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace, dwdWeederValues)
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
		dwdProber = dependencywatchdog.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace, dwdProberValues)
	}

	return
}

func (r *Reconciler) newVPNAuthzServer() (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameExtAuthzServer, imagevectorutils.RuntimeVersion(r.SeedVersion.String()), imagevectorutils.TargetVersion(r.SeedVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpnauthzserver.New(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		image.String(),
		r.SeedVersion,
	), nil
}

func (r *Reconciler) newSystem(seed *gardencorev1beta1.Seed) (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePauseContainer)
	if err != nil {
		return nil, err
	}

	var replicasExcessCapacityReservation int32 = 2
	if numberOfZones := len(seed.Spec.Provider.Zones); numberOfZones > 1 {
		replicasExcessCapacityReservation = int32(numberOfZones)
	}

	return seedsystem.New(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		seedsystem.Values{
			ReserveExcessCapacity: seedsystem.ReserveExcessCapacityValues{
				Enabled:  v1beta1helper.SeedSettingExcessCapacityReservationEnabled(seed.Spec.Settings),
				Image:    image.String(),
				Replicas: replicasExcessCapacityReservation,
				Configs:  seed.Spec.Settings.ExcessCapacityReservation.Configs,
			},
		},
	), nil
}

func (r *Reconciler) newVali(ctx context.Context) (component.Deployer, error) {
	maintenanceBegin, maintenanceEnd := "220000-0000", "230000-0000"

	if hvpaEnabled() {
		shootInfo := &corev1.ConfigMap{}
		if err := r.SeedClientSet.Client().Get(ctx, kubernetesutils.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), shootInfo); err != nil {
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
	if r.Config.Logging != nil && r.Config.Logging.Vali != nil && r.Config.Logging.Vali.Garden != nil {
		storage = r.Config.Logging.Vali.Garden.Storage
	}

	deployer, err := sharedcomponent.NewVali(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		nil,
		component.ClusterTypeSeed,
		1,
		false,
		v1beta1constants.PriorityClassNameSeedSystem600,
		storage,
		"",
		hvpaEnabled(),
		&hvpav1alpha1.MaintenanceTimeWindow{
			Begin: maintenanceBegin,
			End:   maintenanceEnd,
		},
	)
	if err != nil {
		return nil, err
	}

	if !gardenlethelper.IsLoggingEnabled(&r.Config) {
		return component.OpDestroy(deployer), err
	}

	return deployer, err
}

func (r *Reconciler) newPlutono(seed *seedpkg.Seed, secretsManager secretsmanager.Interface, authSecret, wildcardCertSecret *corev1.Secret) (plutono.Interface, error) {
	var wildcardCertName *string
	if wildcardCertSecret != nil {
		wildcardCertName = ptr.To(wildcardCertSecret.GetName())
	}

	var authSecretName string
	if authSecret != nil {
		authSecretName = authSecret.Name
	}

	return sharedcomponent.NewPlutono(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		component.ClusterTypeSeed,
		1,
		authSecretName,
		seed.GetIngressFQDN("g-seed"),
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

func (r *Reconciler) newMonitoring(secretsManager secretsmanager.Interface, seed *seedpkg.Seed, globalMonitoringSecret *corev1.Secret, ingressHost string, wildcardCertSecret *corev1.Secret) (component.Deployer, error) {
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

	var wildcardCertName *string
	if wildcardCertSecret != nil {
		wildcardCertName = ptr.To(wildcardCertSecret.GetName())
	}

	return monitoring.NewBootstrap(
		r.SeedClientSet.Client(),
		r.SeedClientSet.ChartApplier(),
		secretsManager,
		r.GardenNamespace,
		monitoring.ValuesBootstrap{
			GlobalMonitoringSecret:             globalMonitoringSecret,
			HVPAEnabled:                        hvpaEnabled(),
			ImageAlpine:                        imageAlpine.String(),
			ImageConfigmapReloader:             imageConfigmapReloader.String(),
			ImagePrometheus:                    imagePrometheus.String(),
			IngressHost:                        ingressHost,
			SeedName:                           seed.GetInfo().Name,
			StorageCapacityAggregatePrometheus: seed.GetValidVolumeSize("20Gi"),
			WildcardCertName:                   wildcardCertName,
		},
	), nil
}

func (r *Reconciler) newCachePrometheus(log logr.Logger, seed *seedpkg.Seed) (component.DeployWaiter, error) {
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	storageCapacity := resource.MustParse(seed.GetValidVolumeSize("10Gi"))

	return prometheus.New(log, r.SeedClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "cache",
		Image:             imagePrometheus.String(),
		Version:           ptr.Deref(imagePrometheus.Version, "v0.0.0"),
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   storageCapacity,
		Retention:         ptr.To(monitoringv1.Duration("1d")),
		RetentionSize:     "5GB",
		CentralConfigs: prometheus.CentralConfigs{
			AdditionalScrapeConfigs: cacheprometheus.AdditionalScrapeConfigs(),
			ServiceMonitors:         cacheprometheus.CentralServiceMonitors(),
			PrometheusRules:         cacheprometheus.CentralPrometheusRules(),
		},
		AdditionalResources: []client.Object{cacheprometheus.NetworkPolicyToNodeExporter(r.GardenNamespace)},
		// TODO(rfranzke): Remove this after v1.92 has been released.
		DataMigration: monitoring.DataMigration{
			Client:          r.SeedClientSet.Client(),
			Namespace:       r.GardenNamespace,
			StorageCapacity: storageCapacity,
			ImageAlpine:     imageAlpine.String(),
			StatefulSetName: "prometheus",
			FullName:        "prometheus-cache",
			PVCName:         "prometheus-db-prometheus-0",
		},
	}), nil
}

func (r *Reconciler) newSeedPrometheus(log logr.Logger, seed *seedpkg.Seed) (component.DeployWaiter, error) {
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	storageCapacity := resource.MustParse(seed.GetValidVolumeSize("100Gi"))

	return prometheus.New(log, r.SeedClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "seed",
		Image:             imagePrometheus.String(),
		Version:           ptr.Deref(imagePrometheus.Version, "v0.0.0"),
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   storageCapacity,
		RetentionSize:     "85GB",
		VPAMinAllowed:     &corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("400Mi")},
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-extensions-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
			// TODO: For whatever reasons, the seed-prometheus also scrapes vpa-recommenders in all shoot namespaces.
			//  Conceptionally, this is wrong and should be improved (seed-prometheus should only scrape
			//  vpa-recommenders in garden namespace, and prometheis in shoot namespaces should scrape their
			//  vpa-recommenders, respectively).
			gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-vpa-recommender", 8942): v1beta1constants.LabelNetworkPolicyAllowed,
		},
		CentralConfigs: prometheus.CentralConfigs{
			PodMonitors:   seedprometheus.CentralPodMonitors(),
			ScrapeConfigs: seedprometheus.CentralScrapeConfigs(),
		},
		// TODO(rfranzke): Remove this after v1.92 has been released.
		DataMigration: monitoring.DataMigration{
			Client:          r.SeedClientSet.Client(),
			Namespace:       r.GardenNamespace,
			StorageCapacity: storageCapacity,
			ImageAlpine:     imageAlpine.String(),
			StatefulSetName: "seed-prometheus",
			FullName:        "prometheus-seed",
			PVCName:         "prometheus-db-seed-prometheus-0",
		},
	}), nil
}

func (r *Reconciler) newAggregatePrometheus(log logr.Logger, seed *seedpkg.Seed, secretsManager secretsmanager.Interface, globalMonitoringSecret, wildcardCertSecret, alertingSMTPSecret *corev1.Secret) (component.DeployWaiter, error) {
	imagePrometheus, err := imagevector.ImageVector().FindImage(imagevector.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	var (
		storageCapacity  = resource.MustParse(seed.GetValidVolumeSize("20Gi"))
		alerting         *prometheus.AlertingValues
		wildcardCertName *string
	)

	if wildcardCertSecret != nil {
		wildcardCertName = ptr.To(wildcardCertSecret.GetName())
	}

	if alertingSMTPSecret != nil {
		alerting = &prometheus.AlertingValues{
			AlertmanagerName: "alertmanager-seed",
		}
	}

	return prometheus.New(log, r.SeedClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "aggregate",
		Image:             imagePrometheus.String(),
		Version:           ptr.Deref(imagePrometheus.Version, "v0.0.0"),
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   storageCapacity,
		Retention:         ptr.To(monitoringv1.Duration("30d")),
		RetentionSize:     "15GB",
		ExternalLabels:    map[string]string{"seed": seed.GetInfo().Name},
		VPAMinAllowed:     &corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1000M")},
		CentralConfigs: prometheus.CentralConfigs{
			PrometheusRules: aggregateprometheus.CentralPrometheusRules(),
			ServiceMonitors: aggregateprometheus.CentralServiceMonitors(),
		},
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-" + v1beta1constants.IstioSystemNamespace + "-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets:                         v1beta1constants.LabelNetworkPolicyAllowed,
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyIstioIngressNamespaceAlias + "-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-prometheus-web", 9090):                                                       v1beta1constants.LabelNetworkPolicyAllowed,
		},
		Ingress: &prometheus.IngressValues{
			AuthSecretName:   globalMonitoringSecret.Name,
			Host:             seed.GetIngressFQDN("p-seed"),
			SecretsManager:   secretsManager,
			WildcardCertName: wildcardCertName,
		},
		Alerting: alerting,
		// TODO(rfranzke): Remove this after v1.93 has been released.
		DataMigration: monitoring.DataMigration{
			Client:          r.SeedClientSet.Client(),
			Namespace:       r.GardenNamespace,
			StorageCapacity: storageCapacity,
			ImageAlpine:     imageAlpine.String(),
			StatefulSetName: "aggregate-prometheus",
			FullName:        "prometheus-aggregate",
			PVCName:         "prometheus-db-aggregate-prometheus-0",
		},
	}), nil
}

func (r *Reconciler) newAlertmanager(log logr.Logger, seed *seedpkg.Seed, alertingSMTPSecret *corev1.Secret) (component.DeployWaiter, error) {
	imageAlertmanager, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlertmanager)
	if err != nil {
		return nil, err
	}
	imageAlpine, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine)
	if err != nil {
		return nil, err
	}

	storageCapacity := resource.MustParse(seed.GetValidVolumeSize("1Gi"))

	alertManager := alertmanager.New(log, r.SeedClientSet.Client(), r.GardenNamespace, alertmanager.Values{
		Name:               "seed",
		Image:              imageAlertmanager.String(),
		Version:            ptr.Deref(imageAlertmanager.Version, "v0.0.0"),
		PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:    storageCapacity,
		AlertingSMTPSecret: alertingSMTPSecret,
		// TODO(rfranzke): Remove this after v1.92 has been released.
		DataMigration: monitoring.DataMigration{
			Client:          r.SeedClientSet.Client(),
			Namespace:       r.GardenNamespace,
			StorageCapacity: storageCapacity,
			ImageAlpine:     imageAlpine.String(),
			StatefulSetName: "alertmanager",
			FullName:        "alertmanager-seed",
			PVCName:         "alertmanager-db-alertmanager-0",
		},
	})

	if alertingSMTPSecret == nil {
		return component.OpDestroyAndWait(alertManager), nil
	}

	return alertManager, nil
}

func (r *Reconciler) newFluentCustomResources(seedIsGarden bool) (deployer component.DeployWaiter, err error) {
	centralLoggingConfigurations := []component.CentralLoggingConfiguration{
		// seed system components
		extensions.CentralLoggingConfiguration,
		dependencywatchdog.CentralLoggingConfiguration,
		alertmanager.CentralLoggingConfiguration,
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
	if gardenlethelper.IsEventLoggingEnabled(&r.Config) {
		centralLoggingConfigurations = append(centralLoggingConfigurations, eventlogger.CentralLoggingConfiguration)
	}

	return sharedcomponent.NewFluentOperatorCustomResources(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		gardenlethelper.IsLoggingEnabled(&r.Config),
		"",
		centralLoggingConfigurations,
		customresources.GetDynamicClusterOutput(map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource}),
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(settings *gardencorev1beta1.SeedSettings, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewVerticalPodAutoscaler(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		secretsManager,
		vpaEnabled(settings),
		v1beta1constants.SecretNameCASeed,
		v1beta1constants.PriorityClassNameSeedSystem800,
		v1beta1constants.PriorityClassNameSeedSystem700,
		v1beta1constants.PriorityClassNameSeedSystem700,
	)
}

func (r *Reconciler) newHVPA() (component.DeployWaiter, error) {
	return sharedcomponent.NewHVPA(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		hvpaEnabled(),
		r.SeedVersion,
		v1beta1constants.PriorityClassNameSeedSystem700,
	)
}

func (r *Reconciler) newEtcdDruid() (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		r.ComponentImageVectors,
		r.Config.ETCDConfig,
		v1beta1constants.PriorityClassNameSeedSystem800,
	)
}

func (r *Reconciler) newKubeStateMetrics() (component.DeployWaiter, error) {
	return sharedcomponent.NewKubeStateMetrics(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		v1beta1constants.PriorityClassNameSeedSystem600,
	)
}

func (r *Reconciler) newPrometheusOperator() (component.DeployWaiter, error) {
	return sharedcomponent.NewPrometheusOperator(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		v1beta1constants.PriorityClassNameSeedSystem600,
	)
}

func (r *Reconciler) newFluentOperator() (component.DeployWaiter, error) {
	return sharedcomponent.NewFluentOperator(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		gardenlethelper.IsLoggingEnabled(&r.Config),
		v1beta1constants.PriorityClassNameSeedSystem600,
	)
}

func (r *Reconciler) newFluentBit() (component.DeployWaiter, error) {
	return sharedcomponent.NewFluentBit(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		gardenlethelper.IsLoggingEnabled(&r.Config),
		v1beta1constants.PriorityClassNameSeedSystem600,
	)
}

func (r *Reconciler) newClusterAutoscaler() component.DeployWaiter {
	return clusterautoscaler.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace)
}

func (r *Reconciler) newMachineControllerManager() component.DeployWaiter {
	return machinecontrollermanager.NewBootstrapper(r.SeedClientSet.Client(), r.GardenNamespace)
}

func (r *Reconciler) newClusterIdentity(seed *gardencorev1beta1.Seed) component.DeployWaiter {
	return clusteridentity.NewForSeed(r.SeedClientSet.Client(), r.GardenNamespace, *seed.Status.ClusterIdentity)
}

func (r *Reconciler) newNginxIngressController(seed *seedpkg.Seed, istioDefaultLabels map[string]string) (component.DeployWaiter, error) {
	providerConfig, err := getConfig(seed.GetInfo())
	if err != nil {
		return nil, err
	}

	return sharedcomponent.NewNginxIngress(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.GardenNamespace,
		r.SeedVersion,
		providerConfig,
		seed.GetLoadBalancerServiceAnnotations(),
		nil,
		v1beta1constants.PriorityClassNameSeedSystem600,
		true,
		true,
		component.ClusterTypeSeed,
		"",
		v1beta1constants.SeedNginxIngressClass,
		[]string{seed.GetIngressFQDN("*")},
		istioDefaultLabels,
	)
}

func (r *Reconciler) newKubeAPIServerService(wildCardCertSecret *corev1.Secret) component.Deployer {
	c := kubeapiserverexposure.NewInternalNameService(r.SeedClientSet.Client(), r.GardenNamespace)
	if wildCardCertSecret == nil {
		c = component.OpDestroy(c)
	}

	return c
}

func (r *Reconciler) newKubeAPIServerIngress(seed *seedpkg.Seed, wildCardCertSecret *corev1.Secret) component.Deployer {
	values := kubeapiserverexposure.IngressValues{}
	if wildCardCertSecret != nil {
		values = kubeapiserverexposure.IngressValues{
			Host:             seed.GetIngressFQDN("api-seed"),
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			ServiceName:      v1beta1constants.DeploymentNameKubeAPIServer,
			TLSSecretName:    &wildCardCertSecret.Name,
		}
	}

	c := kubeapiserverexposure.NewIngress(r.SeedClientSet.Client(), r.GardenNamespace, values)
	if wildCardCertSecret == nil {
		c = component.OpDestroy(c)
	}

	return c
}
