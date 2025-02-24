// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/extensions"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	kubeapiserverexposure "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	kubernetesdashboard "github.com/gardener/gardener/pkg/component/kubernetes/dashboard"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	kubescheduler "github.com/gardener/gardener/pkg/component/kubernetes/scheduler"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	vpnauthzserver "github.com/gardener/gardener/pkg/component/networking/vpn/authzserver"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	"github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/nodemanagement/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/component/observability/logging"
	"github.com/gardener/gardener/pkg/component/observability/logging/eventlogger"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/metricsserver"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/nodeexporter"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	aggregateprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	cacheprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
	seedprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/helper"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

type components struct {
	machineCRD    component.Deployer
	extensionCRD  component.Deployer
	etcdCRD       component.Deployer
	istioCRD      component.Deployer
	vpaCRD        component.Deployer
	fluentCRD     component.Deployer
	prometheusCRD component.DeployWaiter

	clusterIdentity          component.DeployWaiter
	gardenerResourceManager  component.DeployWaiter
	system                   component.DeployWaiter
	istio                    component.DeployWaiter
	istioDefaultLabels       map[string]string
	istioDefaultNamespace    string
	nginxIngressController   component.DeployWaiter
	verticalPodAutoscaler    component.DeployWaiter
	etcdDruid                component.DeployWaiter
	clusterAutoscaler        component.DeployWaiter
	machineControllerManager component.DeployWaiter
	dwdWeeder                component.DeployWaiter
	dwdProber                component.DeployWaiter

	// TODO(Wieneo): Remove this after Gardener v1.117 was released
	vpnAuthzServer component.DeployWaiter

	kubeAPIServerService component.Deployer
	kubeAPIServerIngress component.Deployer
	ingressDNSRecord     component.DeployWaiter

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
	isManagedSeed bool,
) (
	c components,
	err error,
) {
	// crds
	c.machineCRD, err = machinecontrollermanager.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	if err != nil {
		return
	}
	c.extensionCRD, err = extensioncrds.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier(), !seedIsGarden, true)
	if err != nil {
		return
	}
	c.etcdCRD = etcd.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	c.istioCRD = istio.NewCRD(r.SeedClientSet.ChartApplier())
	c.vpaCRD = vpa.NewCRD(r.SeedClientSet.Applier(), nil)
	c.fluentCRD, err = fluentoperator.NewCRDs(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	if err != nil {
		return
	}
	c.prometheusCRD, err = prometheusoperator.NewCRDs(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
	if err != nil {
		return
	}

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
	c.verticalPodAutoscaler, err = r.newVerticalPodAutoscaler(seed.GetInfo().Spec.Settings, secretsManager, seedIsGarden)
	if err != nil {
		return
	}
	c.etcdDruid, err = r.newEtcdDruid(secretsManager)
	if err != nil {
		return
	}
	c.clusterAutoscaler = r.newClusterAutoscaler()
	c.machineControllerManager = r.newMachineControllerManager()
	c.dwdWeeder, c.dwdProber, err = r.newDependencyWatchdogs(seed.GetInfo().Spec.Settings)
	if err != nil {
		return
	}

	// TODO(Wieneo): Remove this after Gardener v1.117 was released
	c.vpnAuthzServer, err = r.newVPNAuthzServer()
	if err != nil {
		return
	}

	c.kubeAPIServerService = r.newKubeAPIServerService(wildCardCertSecret)
	c.kubeAPIServerIngress = r.newKubeAPIServerIngress(seed, wildCardCertSecret, c.istioDefaultLabels, c.istioDefaultNamespace)
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
	c.vali, err = r.newVali()
	if err != nil {
		return
	}
	c.plutono, err = r.newPlutono(seed, secretsManager, globalMonitoringSecretSeed, wildCardCertSecret)
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
	c.cachePrometheus, err = r.newCachePrometheus(log, seed, isManagedSeed)
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
	c.aggregatePrometheus, err = r.newAggregatePrometheus(log, seed, seedIsGarden, secretsManager, globalMonitoringSecretSeed, wildCardCertSecret, alertingSMTPSecret)
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

	return sharedcomponent.NewRuntimeGardenerResourceManager(r.SeedClientSet.Client(), r.GardenNamespace, secretsManager, resourcemanager.Values{
		DefaultSeccompProfileEnabled:              features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		DefaultNotReadyToleration:                 defaultNotReadyTolerationSeconds,
		DefaultUnreachableToleration:              defaultUnreachableTolerationSeconds,
		EndpointSliceHintsEnabled:                 v1beta1helper.SeedSettingTopologyAwareRoutingEnabled(seed.Spec.Settings),
		LogLevel:                                  r.Config.LogLevel,
		LogFormat:                                 r.Config.LogFormat,
		NetworkPolicyAdditionalNamespaceSelectors: additionalNetworkPolicyNamespaceSelectors,
		PriorityClassName:                         v1beta1constants.PriorityClassNameSeedSystemCritical,
		SecretNameServerCA:                        v1beta1constants.SecretNameCASeed,
		Zones:                                     seed.Spec.Provider.Zones,
	})
}

func (r *Reconciler) newIstio(ctx context.Context, seed *seedpkg.Seed, isGardenCluster bool) (component.DeployWaiter, map[string]string, string, error) {
	labels := sharedcomponent.GetIstioZoneLabels(r.Config.SNI.Ingress.Labels, nil)

	servicePorts := []corev1.ServicePort{
		{Name: "tcp", Port: 443, TargetPort: intstr.FromInt32(9443)},
		{Name: "tls-tunnel", Port: vpnseedserver.GatewayPort, TargetPort: intstr.FromInt32(vpnseedserver.GatewayPort)},
	}

	proxyProtocolEnabled := !features.DefaultFeatureGate.Enabled(features.RemoveAPIServerProxyLegacyPort)
	if proxyProtocolEnabled {
		servicePorts = append(servicePorts, corev1.ServicePort{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt32(8443)})
	}

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
		servicePorts,
		proxyProtocolEnabled,
		seed.GetLoadBalancerServiceProxyProtocolTermination(),
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
				seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone),
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
			seed.GetLoadBalancerServiceProxyProtocolTermination(),
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
					seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone),
				); err != nil {
					return nil, nil, "", err
				}
			}
		}
	}

	return istioDeployer, labels, istioDeployer.GetValues().IngressGateway[0].Namespace, nil
}

func (r *Reconciler) newDependencyWatchdogs(seedSettings *gardencorev1beta1.SeedSettings) (dwdWeeder component.DeployWaiter, dwdProber component.DeployWaiter, err error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameDependencyWatchdog, imagevectorutils.RuntimeVersion(r.SeedVersion.String()), imagevectorutils.TargetVersion(r.SeedVersion.String()))
	if err != nil {
		return nil, nil, err
	}

	var (
		dwdWeederValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleWeeder, Image: image.String()}
		dwdProberValues = dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProber, Image: image.String()}
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
				KubeConfigSecretName:   dependencywatchdog.KubeConfigSecretName,
				ProbeInterval:          &metav1.Duration{Duration: dependencywatchdog.DefaultProbeInterval},
				DependentResourceInfos: make([]proberapi.DependentResourceInfo, 0, len(dependencyWatchdogProberConfigurationFuncs)),
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

// TODO(Wieneo): Remove this after Gardener v1.117 was released
func (r *Reconciler) newVPNAuthzServer() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameExtAuthzServer, imagevectorutils.RuntimeVersion(r.SeedVersion.String()), imagevectorutils.TargetVersion(r.SeedVersion.String()))
	if err != nil {
		return nil, err
	}

	return vpnauthzserver.New(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		image.String(),
	), nil
}

func (r *Reconciler) newSystem(seed *gardencorev1beta1.Seed) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePauseContainer)
	if err != nil {
		return nil, err
	}

	var replicasExcessCapacityReservation int32 = 2
	if numberOfZones := len(seed.Spec.Provider.Zones); numberOfZones > 1 {
		replicasExcessCapacityReservation = int32(numberOfZones) // #nosec G115 -- `len(seed.Spec.Provider.Zones)` cannot be higher than max int32. Zones come from shoot spec and there is a validation that there cannot be more zones than worker.Maximum which is int32.
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

func (r *Reconciler) newVali() (component.Deployer, error) {
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
		v1beta1helper.SeedSettingVerticalPodAutoscalerEnabled(seed.GetInfo().Spec.Settings),
		wildcardCertName,
	)
}

func (r *Reconciler) newCachePrometheus(log logr.Logger, seed *seedpkg.Seed, isManagedSeed bool) (component.DeployWaiter, error) {
	additionalScrapeConfigs, err := cacheprometheus.AdditionalScrapeConfigs(isManagedSeed)
	if err != nil {
		return nil, fmt.Errorf("failed getting additional scrape configs: %w", err)
	}

	return sharedcomponent.NewPrometheus(log, r.SeedClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "cache",
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   resource.MustParse(seed.GetValidVolumeSize("10Gi")),
		Replicas:          1,
		Retention:         ptr.To(monitoringv1.Duration("1d")),
		RetentionSize:     "5GB",
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
		},
		CentralConfigs: prometheus.CentralConfigs{
			AdditionalScrapeConfigs: additionalScrapeConfigs,
			ServiceMonitors:         cacheprometheus.CentralServiceMonitors(),
			PrometheusRules:         cacheprometheus.CentralPrometheusRules(),
		},
		AdditionalResources: []client.Object{
			cacheprometheus.NetworkPolicyToNodeExporter(r.GardenNamespace, seed.GetNodeCIDR()),
			cacheprometheus.NetworkPolicyToKubelet(r.GardenNamespace, seed.GetNodeCIDR()),
		},
	})
}

func (r *Reconciler) newSeedPrometheus(log logr.Logger, seed *seedpkg.Seed) (component.DeployWaiter, error) {
	return sharedcomponent.NewPrometheus(log, r.SeedClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "seed",
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   resource.MustParse(seed.GetValidVolumeSize("100Gi")),
		Replicas:          1,
		RetentionSize:     "85GB",
		VPAMinAllowed:     &corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("400Mi")},
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets:            v1beta1constants.LabelNetworkPolicyAllowed,
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
	})
}

func (r *Reconciler) newAggregatePrometheus(log logr.Logger, seed *seedpkg.Seed, seedIsGarden bool, secretsManager secretsmanager.Interface, globalMonitoringSecret, wildcardCertSecret, alertingSMTPSecret *corev1.Secret) (component.DeployWaiter, error) {
	values := prometheus.Values{
		Name:              "aggregate",
		PriorityClassName: v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:   resource.MustParse(seed.GetValidVolumeSize("20Gi")),
		Replicas:          1,
		Retention:         ptr.To(monitoringv1.Duration("30d")),
		RetentionSize:     "15GB",
		ExternalLabels:    map[string]string{"seed": seed.GetInfo().Name},
		VPAMinAllowed:     &corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1000M")},
		CentralConfigs: prometheus.CentralConfigs{
			PrometheusRules: aggregateprometheus.CentralPrometheusRules(seedIsGarden),
			ScrapeConfigs:   aggregateprometheus.CentralScrapeConfigs(),
			ServiceMonitors: aggregateprometheus.CentralServiceMonitors(),
		},
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets:                                                                       v1beta1constants.LabelNetworkPolicyAllowed,
			"networking.resources.gardener.cloud/to-" + v1beta1constants.IstioSystemNamespace + "-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets:                         v1beta1constants.LabelNetworkPolicyAllowed,
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyIstioIngressNamespaceAlias + "-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyShootNamespaceAlias+"-prometheus-shoot", 9090):                                                     v1beta1constants.LabelNetworkPolicyAllowed,
		},
		Ingress: &prometheus.IngressValues{
			Host:           seed.GetIngressFQDN(v1beta1constants.IngressDomainPrefixPrometheusAggregate),
			SecretsManager: secretsManager,
			SigningCA:      v1beta1constants.SecretNameCASeed,
		},
	}

	if globalMonitoringSecret != nil {
		values.Ingress.AuthSecretName = globalMonitoringSecret.Name
	}

	if wildcardCertSecret != nil {
		values.Ingress.WildcardCertSecretName = ptr.To(wildcardCertSecret.GetName())
	}

	if alertingSMTPSecret != nil {
		values.Alerting = &prometheus.AlertingValues{Alertmanagers: []*prometheus.Alertmanager{{Name: "alertmanager-seed"}}}
	}

	return sharedcomponent.NewPrometheus(log, r.SeedClientSet.Client(), r.GardenNamespace, values)
}

func (r *Reconciler) newAlertmanager(log logr.Logger, seed *seedpkg.Seed, alertingSMTPSecret *corev1.Secret) (component.DeployWaiter, error) {
	c, err := sharedcomponent.NewAlertmanager(log, r.SeedClientSet.Client(), r.GardenNamespace, alertmanager.Values{
		Name:               "seed",
		ClusterType:        component.ClusterTypeSeed,
		PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystem600,
		StorageCapacity:    resource.MustParse(seed.GetValidVolumeSize("1Gi")),
		Replicas:           1,
		AlertingSMTPSecret: alertingSMTPSecret,
	})

	if alertingSMTPSecret == nil {
		return component.OpDestroyAndWait(c), nil
	}

	return c, err
}

func (r *Reconciler) newFluentCustomResources(seedIsGarden bool) (deployer component.DeployWaiter, err error) {
	centralLoggingConfigurations := []component.CentralLoggingConfiguration{
		// seed system components
		extensions.CentralLoggingConfiguration,
		dependencywatchdog.CentralLoggingConfiguration,
		alertmanager.CentralLoggingConfiguration,
		prometheus.CentralLoggingConfiguration,
		plutono.CentralLoggingConfiguration,
		// shoot control plane components
		clusterautoscaler.CentralLoggingConfiguration,
		vpnseedserver.CentralLoggingConfiguration,
		kubescheduler.CentralLoggingConfiguration,
		machinecontrollermanager.CentralLoggingConfiguration,
		// shoot worker components
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

	var output *fluentbitv1alpha2.ClusterOutput
	if gardenlethelper.IsValiEnabled(&r.Config) {
		output = fluentcustomresources.GetDynamicClusterOutput(map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource})
	}

	return sharedcomponent.NewFluentOperatorCustomResources(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		gardenlethelper.IsLoggingEnabled(&r.Config),
		"",
		centralLoggingConfigurations,
		output,
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(settings *gardencorev1beta1.SeedSettings, secretsManager secretsmanager.Interface, isGardenCluster bool) (component.DeployWaiter, error) {
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
		isGardenCluster,
	)
}

func (r *Reconciler) newEtcdDruid(secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		r.ComponentImageVectors,
		r.Config.ETCDConfig,
		secretsManager,
		v1beta1constants.SecretNameCASeed,
		v1beta1constants.PriorityClassNameSeedSystem800,
	)
}

func (r *Reconciler) newKubeStateMetrics() (component.DeployWaiter, error) {
	return sharedcomponent.NewKubeStateMetrics(
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		r.SeedVersion,
		v1beta1constants.PriorityClassNameSeedSystem600,
		kubestatemetrics.SuffixSeed,
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
		gardenlethelper.IsValiEnabled(&r.Config),
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

func (r *Reconciler) newKubeAPIServerIngress(seed *seedpkg.Seed, wildCardCertSecret *corev1.Secret, istioDefaultLabels map[string]string, istioDefaultNamespace string) component.Deployer {
	values := kubeapiserverexposure.IngressValues{ServiceNamespace: metav1.NamespaceDefault}
	if wildCardCertSecret != nil {
		values = kubeapiserverexposure.IngressValues{
			Host: seed.GetIngressFQDN("api-seed"),
			IstioIngressGatewayLabelsFunc: func() map[string]string {
				return istioDefaultLabels
			},
			IstioIngressGatewayNamespaceFunc: func() string {
				return istioDefaultNamespace
			},
			ServiceName:      "kubernetes",
			ServiceNamespace: metav1.NamespaceDefault,
			TLSSecretName:    &wildCardCertSecret.Name,
		}
	}

	c := kubeapiserverexposure.NewIngress(r.SeedClientSet.Client(), r.GardenNamespace, values)
	if wildCardCertSecret == nil {
		c = component.OpDestroy(c)
	}

	return c
}
