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

package garden

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/gardensystem"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

type components struct {
	etcdCRD  component.Deployer
	vpaCRD   component.Deployer
	hvpaCRD  component.Deployer
	istioCRD component.DeployWaiter

	gardenerResourceManager component.DeployWaiter
	system                  component.DeployWaiter
	verticalPodAutoscaler   component.DeployWaiter
	hvpaController          component.DeployWaiter
	etcdDruid               component.DeployWaiter
	istio                   istio.Interface
	nginxIngressController  component.DeployWaiter

	etcdMain                             etcd.Interface
	etcdEvents                           etcd.Interface
	kubeAPIServerService                 component.DeployWaiter
	kubeAPIServerSNI                     component.Deployer
	kubeAPIServer                        kubeapiserver.Interface
	kubeControllerManager                kubecontrollermanager.Interface
	virtualGardenGardenerResourceManager resourcemanager.Interface
	virtualGardenGardenerAccess          component.Deployer

	kubeStateMetrics component.DeployWaiter
}

func (r *Reconciler) instantiateComponents(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
	applier kubernetes.Applier,
) (
	c components,
	err error,
) {
	// crds
	c.etcdCRD = etcd.NewCRD(r.RuntimeClientSet.Client(), applier)
	c.vpaCRD = vpa.NewCRD(applier, nil)
	c.hvpaCRD = hvpa.NewCRD(applier)
	if !hvpaEnabled() {
		c.hvpaCRD = component.OpDestroy(c.hvpaCRD)
	}
	c.istioCRD = istio.NewCRD(r.RuntimeClientSet.ChartApplier())

	// garden system components
	c.gardenerResourceManager, err = r.newGardenerResourceManager(garden, secretsManager)
	if err != nil {
		return
	}
	c.system = r.newSystem()
	c.verticalPodAutoscaler, err = r.newVerticalPodAutoscaler(garden, secretsManager)
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
	c.istio, err = r.newIstio(garden)
	if err != nil {
		return
	}
	c.nginxIngressController, err = r.newNginxIngressController()
	if err != nil {
		return
	}

	// virtual garden control plane components
	c.etcdMain, err = r.newEtcd(log, garden, secretsManager, v1beta1constants.ETCDRoleMain, etcd.ClassImportant)
	if err != nil {
		return
	}
	c.etcdEvents, err = r.newEtcd(log, garden, secretsManager, v1beta1constants.ETCDRoleEvents, etcd.ClassNormal)
	if err != nil {
		return
	}
	c.kubeAPIServerService, err = r.newKubeAPIServerService(log, garden, c.istio.GetValues().IngressGateway)
	if err != nil {
		return
	}
	c.kubeAPIServerSNI, err = r.newSNI(garden, c.istio.GetValues().IngressGateway)
	if err != nil {
		return
	}
	c.kubeAPIServer, err = r.newKubeAPIServer(ctx, garden, secretsManager, targetVersion)
	if err != nil {
		return
	}
	c.kubeControllerManager, err = r.newKubeControllerManager(log, garden, secretsManager, targetVersion)
	if err != nil {
		return
	}
	c.virtualGardenGardenerResourceManager, err = r.newVirtualGardenGardenerResourceManager(secretsManager)
	if err != nil {
		return
	}

	var accessDomain string
	if domains := garden.Spec.VirtualCluster.DNS.Domains; len(domains) > 0 {
		accessDomain = domains[0]
	} else {
		accessDomain = *garden.Spec.VirtualCluster.DNS.Domain
	}
	c.virtualGardenGardenerAccess = r.newGardenerAccess(secretsManager, accessDomain)

	// observability components
	c.kubeStateMetrics, err = r.newKubeStateMetrics()
	if err != nil {
		return
	}

	return c, nil
}

const namePrefix = "virtual-garden-"

func (r *Reconciler) newGardenerResourceManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	var defaultNotReadyTolerationSeconds, defaultUnreachableTolerationSeconds *int64
	if nodeToleration := r.Config.NodeToleration; nodeToleration != nil {
		defaultNotReadyTolerationSeconds = nodeToleration.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = nodeToleration.DefaultUnreachableTolerationSeconds
	}

	return sharedcomponent.NewRuntimeGardenerResourceManager(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		r.Config.LogLevel, r.Config.LogFormat,
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystemCritical,
		defaultNotReadyTolerationSeconds,
		defaultUnreachableTolerationSeconds,
		features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		r.Config.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
		garden.Spec.RuntimeCluster.Provider.Zones,
	)
}

func (r *Reconciler) newVirtualGardenGardenerResourceManager(secretsManager secretsmanager.Interface) (resourcemanager.Interface, error) {
	return sharedcomponent.NewTargetGardenerResourceManager(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.ImageVector,
		secretsManager,
		nil,
		nil,
		nil,
		r.RuntimeVersion,
		r.Config.LogLevel, r.Config.LogFormat,
		namePrefix,
		false,
		v1beta1constants.PriorityClassNameGardenSystem400,
		nil,
		operatorv1alpha1.SecretNameCARuntime,
		nil,
		false,
		nil,
		true,
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewVerticalPodAutoscaler(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		secretsManager,
		vpaEnabled(garden.Spec.RuntimeCluster.Settings),
		operatorv1alpha1.SecretNameCARuntime,
		v1beta1constants.PriorityClassNameGardenSystem300,
		v1beta1constants.PriorityClassNameGardenSystem200,
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newHVPA() (component.DeployWaiter, error) {
	return sharedcomponent.NewHVPA(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		hvpaEnabled(),
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newEtcdDruid() (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		r.ComponentImageVectors,
		r.Config.Controllers.Garden.ETCDConfig,
		v1beta1constants.PriorityClassNameGardenSystem300,
	)
}

func (r *Reconciler) newSystem() component.DeployWaiter {
	return gardensystem.New(r.RuntimeClientSet.Client(), r.GardenNamespace)
}

func (r *Reconciler) newEtcd(
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	role string,
	class etcd.Class,
) (
	etcd.Interface,
	error,
) {
	var (
		hvpaScaleDownUpdateMode       *string
		defragmentationScheduleFormat string
		storageClassName              *string
		storageCapacity               string
	)

	switch role {
	case v1beta1constants.ETCDRoleMain:
		hvpaScaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeOff)
		defragmentationScheduleFormat = "%d %d * * *" // defrag main etcd daily in the maintenance window
		storageCapacity = "25Gi"
		if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Main != nil && etcd.Main.Storage != nil {
			storageClassName = etcd.Main.Storage.ClassName
			if etcd.Main.Storage.Capacity != nil {
				storageCapacity = etcd.Main.Storage.Capacity.String()
			}
		}

	case v1beta1constants.ETCDRoleEvents:
		hvpaScaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeMaintenanceWindow)
		defragmentationScheduleFormat = "%d %d */3 * *"
		storageCapacity = "10Gi"
		if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Events != nil && etcd.Events.Storage != nil {
			storageClassName = etcd.Events.Storage.ClassName
			if etcd.Events.Storage.Capacity != nil {
				storageCapacity = etcd.Events.Storage.Capacity.String()
			}
		}
	}

	defragmentationSchedule, err := timewindow.DetermineSchedule(
		defragmentationScheduleFormat,
		garden.Spec.VirtualCluster.Maintenance.TimeWindow.Begin,
		garden.Spec.VirtualCluster.Maintenance.TimeWindow.End,
		garden.UID,
		garden.CreationTimestamp,
		timewindow.RandomizeWithinTimeWindow,
	)
	if err != nil {
		return nil, err
	}

	highAvailabilityEnabled := helper.HighAvailabilityEnabled(garden)

	replicas := pointer.Int32(1)
	if highAvailabilityEnabled {
		replicas = pointer.Int32(3)
	}

	return etcd.New(
		log,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		etcd.Values{
			NamePrefix:               namePrefix,
			Role:                     role,
			Class:                    class,
			Replicas:                 replicas,
			StorageCapacity:          storageCapacity,
			StorageClassName:         storageClassName,
			DefragmentationSchedule:  &defragmentationSchedule,
			CARotationPhase:          helper.GetCARotationPhase(garden.Status.Credentials),
			RuntimeKubernetesVersion: r.RuntimeVersion,
			HvpaConfig: &etcd.HVPAConfig{
				Enabled:               hvpaEnabled(),
				MaintenanceTimeWindow: garden.Spec.VirtualCluster.Maintenance.TimeWindow,
				ScaleDownUpdateMode:   hvpaScaleDownUpdateMode,
			},
			PriorityClassName:           v1beta1constants.PriorityClassNameGardenSystem500,
			HighAvailabilityEnabled:     highAvailabilityEnabled,
			TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		},
	), nil
}

func (r *Reconciler) newKubeAPIServerService(log logr.Logger, garden *operatorv1alpha1.Garden, ingressGatewayValues []istio.IngressGatewayValues) (component.DeployWaiter, error) {
	if len(ingressGatewayValues) != 1 {
		return nil, fmt.Errorf("exactly one Istio Ingress Gateway is required for the SNI config")
	}

	var annotations map[string]string
	if settings := garden.Spec.RuntimeCluster.Settings; settings != nil && settings.LoadBalancerServices != nil {
		annotations = settings.LoadBalancerServices.Annotations
	}

	var clusterIP string
	if os.Getenv("GARDENER_OPERATOR_LOCAL") == "true" {
		clusterIP = "10.2.10.2"
	}

	serviceType := corev1.ServiceTypeLoadBalancer

	return kubeapiserverexposure.NewService(
		log,
		r.RuntimeClientSet.Client(),
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return annotations },
			TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
			RuntimeKubernetesVersion:    r.RuntimeVersion,
			ServiceType:                 &serviceType,
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: namePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: r.GardenNamespace}
		},
		func() client.ObjectKey {
			return client.ObjectKey{Name: v1beta1constants.DefaultSNIIngressServiceName, Namespace: ingressGatewayValues[0].Namespace}
		},
		nil,
		nil,
		nil,
		clusterIP,
	), nil
}

func (r *Reconciler) newKubeAPIServer(
	ctx context.Context,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
) (
	kubeapiserver.Interface,
	error,
) {
	var (
		err                          error
		apiServerConfig              *gardencorev1beta1.KubeAPIServerConfig
		auditWebhookConfig           *kubeapiserver.AuditWebhook
		authenticationWebhookConfig  *kubeapiserver.AuthenticationWebhook
		authorizationWebhookConfig   *kubeapiserver.AuthorizationWebhook
		resourcesToStoreInETCDEvents []schema.GroupResource
		minReplicas                  int32 = 2
	)

	if garden.Spec.VirtualCluster.ControlPlane != nil && garden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil {
		minReplicas = 3
	}

	if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil {
		apiServerConfig = apiServer.KubeAPIServerConfig

		auditWebhookConfig, err = r.computeKubeAPIServerAuditWebhookConfig(ctx, apiServer.AuditWebhook)
		if err != nil {
			return nil, err
		}

		authenticationWebhookConfig, err = r.computeKubeAPIServerAuthenticationWebhookConfig(ctx, apiServer.Authentication)
		if err != nil {
			return nil, err
		}

		authorizationWebhookConfig, err = r.computeKubeAPIServerAuthorizationWebhookConfig(ctx, apiServer.Authorization)
		if err != nil {
			return nil, err
		}

		for _, gr := range apiServer.ResourcesToStoreInETCDEvents {
			resourcesToStoreInETCDEvents = append(resourcesToStoreInETCDEvents, schema.GroupResource{Group: gr.Group, Resource: gr.Resource})
		}
	}

	return sharedcomponent.NewKubeAPIServer(
		ctx,
		r.RuntimeClientSet,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		metav1.ObjectMeta{Namespace: r.GardenNamespace, Name: garden.Name},
		r.RuntimeVersion,
		targetVersion,
		r.ImageVector,
		secretsManager,
		namePrefix,
		apiServerConfig,
		kubeapiserver.AutoscalingConfig{
			APIServerResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("600m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			HVPAEnabled:               hvpaEnabled(),
			MinReplicas:               minReplicas,
			MaxReplicas:               6,
			UseMemoryMetricForHvpaHPA: true,
			ScaleDownDisabledForHvpa:  false,
		},
		garden.Spec.VirtualCluster.Networking.Services,
		kubeapiserver.VPNConfig{Enabled: false},
		v1beta1constants.PriorityClassNameGardenSystem500,
		true,
		pointer.Bool(false),
		auditWebhookConfig,
		authenticationWebhookConfig,
		authorizationWebhookConfig,
		resourcesToStoreInETCDEvents,
	)
}

func (r *Reconciler) computeKubeAPIServerAuditWebhookConfig(ctx context.Context, config *operatorv1alpha1.AuditWebhook) (*kubeapiserver.AuditWebhook, error) {
	if config == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	return &kubeapiserver.AuditWebhook{
		Kubeconfig:   kubeconfig,
		BatchMaxSize: config.BatchMaxSize,
		Version:      config.Version,
	}, nil
}

func (r *Reconciler) computeKubeAPIServerAuthenticationWebhookConfig(ctx context.Context, config *operatorv1alpha1.Authentication) (*kubeapiserver.AuthenticationWebhook, error) {
	if config == nil || config.Webhook == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.Webhook.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	var cacheTTL *time.Duration
	if config.Webhook.CacheTTL != nil {
		cacheTTL = &config.Webhook.CacheTTL.Duration
	}

	return &kubeapiserver.AuthenticationWebhook{
		Kubeconfig: kubeconfig,
		CacheTTL:   cacheTTL,
		Version:    config.Webhook.Version,
	}, nil
}

func (r *Reconciler) computeKubeAPIServerAuthorizationWebhookConfig(ctx context.Context, config *operatorv1alpha1.Authorization) (*kubeapiserver.AuthorizationWebhook, error) {
	if config == nil || config.Webhook == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.Webhook.KubeconfigSecretName}
	kubeconfig, err := fetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	var cacheAuthorizedTTL, cacheUnauthorizedTTL *time.Duration
	if config.Webhook.CacheAuthorizedTTL != nil {
		cacheAuthorizedTTL = &config.Webhook.CacheAuthorizedTTL.Duration
	}
	if config.Webhook.CacheUnauthorizedTTL != nil {
		cacheUnauthorizedTTL = &config.Webhook.CacheUnauthorizedTTL.Duration
	}

	return &kubeapiserver.AuthorizationWebhook{
		Kubeconfig:           kubeconfig,
		CacheAuthorizedTTL:   cacheAuthorizedTTL,
		CacheUnauthorizedTTL: cacheUnauthorizedTTL,
		Version:              config.Webhook.Version,
	}, nil
}

func fetchKubeconfigFromSecret(ctx context.Context, c client.Client, key client.ObjectKey) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok || len(kubeconfig) == 0 {
		return nil, errors.New("the secret's field 'kubeconfig' is empty")
	}

	return kubeconfig, nil
}

func (r *Reconciler) newKubeControllerManager(
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
) (
	kubecontrollermanager.Interface,
	error,
) {
	var (
		config                     *gardencorev1beta1.KubeControllerManagerConfig
		certificateSigningDuration *time.Duration
	)

	if controllerManager := garden.Spec.VirtualCluster.Kubernetes.KubeControllerManager; controllerManager != nil {
		config = controllerManager.KubeControllerManagerConfig
		certificateSigningDuration = pointer.Duration(controllerManager.CertificateSigningDuration.Duration)
	}

	_, services, err := net.ParseCIDR(garden.Spec.VirtualCluster.Networking.Services)
	if err != nil {
		return nil, fmt.Errorf("cannot parse service network CIDR: %w", err)
	}

	return sharedcomponent.NewKubeControllerManager(
		log,
		r.RuntimeClientSet,
		r.GardenNamespace,
		r.RuntimeVersion,
		targetVersion,
		r.ImageVector,
		secretsManager,
		namePrefix,
		config,
		v1beta1constants.PriorityClassNameGardenSystem300,
		true,
		&kubecontrollermanager.HVPAConfig{Enabled: hvpaEnabled()},
		nil,
		services,
		certificateSigningDuration,
		kubecontrollermanager.ControllerWorkers{
			GarbageCollector:    pointer.Int(250),
			Namespace:           pointer.Int(100),
			ResourceQuota:       pointer.Int(100),
			ServiceAccountToken: pointer.Int(0),
		},
		kubecontrollermanager.ControllerSyncPeriods{
			ResourceQuota: pointer.Duration(time.Minute),
		},
	)
}

func (r *Reconciler) newKubeStateMetrics() (component.DeployWaiter, error) {
	return sharedcomponent.NewKubeStateMetrics(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ImageVector,
		v1beta1constants.PriorityClassNameGardenSystem100,
	)
}

func (r *Reconciler) newIstio(garden *operatorv1alpha1.Garden) (istio.Interface, error) {
	var annotations map[string]string
	if settings := garden.Spec.RuntimeCluster.Settings; settings != nil && settings.LoadBalancerServices != nil {
		annotations = settings.LoadBalancerServices.Annotations
	}

	return sharedcomponent.NewIstio(
		r.RuntimeClientSet.Client(),
		r.ImageVector,
		r.RuntimeClientSet.ChartRenderer(),
		namePrefix,
		v1beta1constants.DefaultSNIIngressNamespace,
		v1beta1constants.PriorityClassNameGardenSystemCritical,
		true,
		sharedcomponent.GetIstioZoneLabels(nil, nil),
		gardenerutils.NetworkPolicyLabel(r.GardenNamespace+"-"+kubeapiserverconstants.ServiceName(namePrefix), kubeapiserverconstants.Port),
		annotations,
		nil,
		nil,
		[]corev1.ServicePort{
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(9443)},
		},
		false,
		false,
		garden.Spec.RuntimeCluster.Provider.Zones,
	)
}

func (r *Reconciler) newSNI(garden *operatorv1alpha1.Garden, ingressGatewayValues []istio.IngressGatewayValues) (component.Deployer, error) {
	if len(ingressGatewayValues) != 1 {
		return nil, fmt.Errorf("exactly one Istio Ingress Gateway is required for the SNI config")
	}

	var domains []string
	if domain := garden.Spec.VirtualCluster.DNS.Domain; domain != nil {
		domains = append(domains, *domain)
	}
	domains = append(domains, garden.Spec.VirtualCluster.DNS.Domains...)

	return kubeapiserverexposure.NewSNI(
		r.RuntimeClientSet.Client(),
		r.RuntimeClientSet.Applier(),
		namePrefix+v1beta1constants.DeploymentNameKubeAPIServer,
		r.GardenNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				Hosts: getAPIServerDomains(domains),
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: ingressGatewayValues[0].Namespace,
					Labels:    ingressGatewayValues[0].Labels,
				},
			}
		},
	), nil
}

func (r *Reconciler) newGardenerAccess(secretsManager secretsmanager.Interface, domain string) component.Deployer {
	return gardeneraccess.New(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		gardeneraccess.Values{
			ServerInCluster:    fmt.Sprintf("%s%s.%s.svc.cluster.local", namePrefix, v1beta1constants.DeploymentNameKubeAPIServer, r.GardenNamespace),
			ServerOutOfCluster: gardenerutils.GetAPIServerDomain(domain),
		},
	)
}

func getAPIServerDomains(domains []string) []string {
	apiServerDomains := make([]string, 0, len(domains)*2)
	for _, domain := range domains {
		apiServerDomains = append(apiServerDomains, gardenerutils.GetAPIServerDomain(domain))
		apiServerDomains = append(apiServerDomains, "gardener."+domain)
	}
	return apiServerDomains
}

func (r *Reconciler) newNginxIngressController() (component.DeployWaiter, error) {
	return sharedcomponent.NewNginxIngress(
		r.RuntimeClientSet.Client(),
		r.ImageVector,
		r.RuntimeVersion,
		v1beta1constants.SeedNginxIngressClass,
		map[string]string{
			"enable-vts-status":            "false",
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
			"allow-snippet-annotations":    "false",
		},
		nil,
		r.GardenNamespace,
		v1beta1constants.PriorityClassNameGardenSystem300)
}
