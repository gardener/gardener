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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Masterminds/semver/v3"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1conversion "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/gardeneradmissioncontroller"
	"github.com/gardener/gardener/pkg/component/gardenerapiserver"
	"github.com/gardener/gardener/pkg/component/gardenercontrollermanager"
	"github.com/gardener/gardener/pkg/component/gardenermetricsexporter"
	"github.com/gardener/gardener/pkg/component/gardenerscheduler"
	runtimegardensystem "github.com/gardener/gardener/pkg/component/gardensystem/runtime"
	virtualgardensystem "github.com/gardener/gardener/pkg/component/gardensystem/virtual"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/logging"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	controllermanagerv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

type components struct {
	etcdCRD   component.Deployer
	vpaCRD    component.Deployer
	hvpaCRD   component.Deployer
	istioCRD  component.DeployWaiter
	fluentCRD component.DeployWaiter

	gardenerResourceManager component.DeployWaiter
	runtimeSystem           component.DeployWaiter
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
	virtualGardenGardenerAccess          component.DeployWaiter
	virtualSystem                        component.DeployWaiter

	gardenerAPIServer           gardenerapiserver.Interface
	gardenerAdmissionController component.DeployWaiter
	gardenerControllerManager   component.DeployWaiter
	gardenerScheduler           component.DeployWaiter

	gardenerMetricsExporter       component.DeployWaiter
	kubeStateMetrics              component.DeployWaiter
	fluentOperator                component.DeployWaiter
	fluentBit                     component.DeployWaiter
	fluentOperatorCustomResources component.DeployWaiter
	plutono                       plutono.Interface
	vali                          component.Deployer
}

func (r *Reconciler) instantiateComponents(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
	applier kubernetes.Applier,
	wildcardCert *corev1.Secret,
	enableSeedAuthorizer bool,
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
	c.fluentCRD = fluentoperator.NewCRDs(applier)

	// garden system components
	c.gardenerResourceManager, err = r.newGardenerResourceManager(garden, secretsManager)
	if err != nil {
		return
	}
	c.runtimeSystem = r.newRuntimeSystem()
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
	c.nginxIngressController, err = r.newNginxIngressController(garden)
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
	c.kubeAPIServer, err = r.newKubeAPIServer(ctx, garden, secretsManager, targetVersion, enableSeedAuthorizer)
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
	c.virtualSystem = r.newVirtualSystem(enableSeedAuthorizer)
	c.virtualGardenGardenerAccess = r.newGardenerAccess(garden, secretsManager)

	// gardener control plane components
	c.gardenerAPIServer, err = r.newGardenerAPIServer(ctx, garden, secretsManager)
	if err != nil {
		return
	}
	c.gardenerAdmissionController, err = r.newGardenerAdmissionController(garden, secretsManager, enableSeedAuthorizer)
	if err != nil {
		return
	}
	c.gardenerControllerManager, err = r.newGardenerControllerManager(garden, secretsManager)
	if err != nil {
		return
	}
	c.gardenerScheduler, err = r.newGardenerScheduler(garden, secretsManager)
	if err != nil {
		return
	}

	// observability components
	c.gardenerMetricsExporter, err = r.newGardenerMetricsExporter(secretsManager)
	if err != nil {
		return
	}
	c.kubeStateMetrics, err = r.newKubeStateMetrics()
	if err != nil {
		return
	}
	c.fluentOperator, err = r.newFluentOperator()
	if err != nil {
		return
	}
	c.fluentBit, err = r.newFluentBit()
	if err != nil {
		return
	}
	c.fluentOperatorCustomResources, err = r.newFluentCustomResources()
	if err != nil {
		return
	}
	c.vali, err = r.newVali(garden)
	if err != nil {
		return
	}

	c.plutono, err = r.newPlutono(secretsManager, garden.Spec.RuntimeCluster.Ingress.Domain, wildcardCert)
	if err != nil {
		return
	}

	return c, nil
}

func (r *Reconciler) enableSeedAuthorizer(ctx context.Context) (bool, error) {
	// The reconcile flow deploys the kube-apiserver of the virtual garden cluster before the gardener-apiserver and
	// gardener-admission-controller (it has to be this way, otherwise the Gardener components cannot start). However,
	// GAC serves an authorization webhook for the SeedAuthorizer feature. We can only configure kube-apiserver to
	// consult this webhook when GAC runs, obviously. This is not possible in the initial Garden deployment (due to
	// above order). Hence, we have to run the flow as second time after the initial Garden creation - this time with
	// the SeedAuthorizer feature getting enabled. From then on, all subsequent reconciliations can always enable it and
	// only one reconciliation is needed.
	if err := r.RuntimeClientSet.Client().Get(ctx, client.ObjectKey{Name: gardenerapiserver.DeploymentName, Namespace: r.GardenNamespace}, &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	if err := r.RuntimeClientSet.Client().Get(ctx, client.ObjectKey{Name: gardeneradmissioncontroller.DeploymentName, Namespace: r.GardenNamespace}, &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
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
		[]string{v1beta1constants.GardenNamespace, metav1.NamespaceSystem},
	)
}

func (r *Reconciler) newVerticalPodAutoscaler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewVerticalPodAutoscaler(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
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
		hvpaEnabled(),
		v1beta1constants.PriorityClassNameGardenSystem200,
	)
}

func (r *Reconciler) newEtcdDruid() (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ComponentImageVectors,
		r.Config.Controllers.Garden.ETCDConfig,
		v1beta1constants.PriorityClassNameGardenSystem300,
	)
}

func (r *Reconciler) newRuntimeSystem() component.DeployWaiter {
	return runtimegardensystem.New(r.RuntimeClientSet.Client(), r.GardenNamespace)
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

	return kubeapiserverexposure.NewService(
		log,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		&kubeapiserverexposure.ServiceValues{
			AnnotationsFunc:             func() map[string]string { return annotations },
			NamePrefix:                  namePrefix,
			TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
			RuntimeKubernetesVersion:    r.RuntimeVersion,
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
	enableSeedAuthorizer bool,
) (
	kubeapiserver.Interface,
	error,
) {
	var (
		err                          error
		apiServerConfig              *gardencorev1beta1.KubeAPIServerConfig
		auditWebhookConfig           *apiserver.AuditWebhook
		authenticationWebhookConfig  *kubeapiserver.AuthenticationWebhook
		authorizationWebhookConfig   *kubeapiserver.AuthorizationWebhook
		resourcesToStoreInETCDEvents []schema.GroupResource
	)

	if apiServer := garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer; apiServer != nil {
		apiServerConfig = apiServer.KubeAPIServerConfig

		auditWebhookConfig, err = r.computeAPIServerAuditWebhookConfig(ctx, apiServer.AuditWebhook)
		if err != nil {
			return nil, err
		}

		authenticationWebhookConfig, err = r.computeKubeAPIServerAuthenticationWebhookConfig(ctx, apiServer.Authentication)
		if err != nil {
			return nil, err
		}

		for _, gr := range apiServer.ResourcesToStoreInETCDEvents {
			resourcesToStoreInETCDEvents = append(resourcesToStoreInETCDEvents, schema.GroupResource{Group: gr.Group, Resource: gr.Resource})
		}
	}

	if enableSeedAuthorizer {
		caSecret, found := secretsManager.Get(operatorv1alpha1.SecretNameCAGardener)
		if !found {
			return nil, fmt.Errorf("secret %q not found", operatorv1alpha1.SecretNameCAGardener)
		}

		kubeconfig, err := runtime.Encode(clientcmdlatest.Codec, kubernetesutils.NewKubeconfig(
			"authorization-webhook",
			clientcmdv1.Cluster{
				Server:                   fmt.Sprintf("https://%s/webhooks/auth/seed", gardeneradmissioncontroller.ServiceName),
				CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle],
			},
			clientcmdv1.AuthInfo{},
		))
		if err != nil {
			return nil, fmt.Errorf("failed generating authorization webhook kubeconfig: %w", err)
		}

		authorizationWebhookConfig = &kubeapiserver.AuthorizationWebhook{
			Kubeconfig:           kubeconfig,
			CacheAuthorizedTTL:   pointer.Duration(0),
			CacheUnauthorizedTTL: pointer.Duration(0),
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
		secretsManager,
		namePrefix,
		apiServerConfig,
		defaultAPIServerAutoscalingConfig(garden),
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

func defaultAPIServerAutoscalingConfig(garden *operatorv1alpha1.Garden) apiserver.AutoscalingConfig {
	minReplicas := int32(2)
	if garden.Spec.VirtualCluster.ControlPlane != nil && garden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil {
		minReplicas = 3
	}

	return apiserver.AutoscalingConfig{
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
	}
}

func (r *Reconciler) computeAPIServerAuditWebhookConfig(ctx context.Context, config *operatorv1alpha1.AuditWebhook) (*apiserver.AuditWebhook, error) {
	if config == nil {
		return nil, nil
	}

	key := client.ObjectKey{Namespace: r.GardenNamespace, Name: config.KubeconfigSecretName}
	kubeconfig, err := gardenerutils.FetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
	if err != nil {
		return nil, fmt.Errorf("failed reading kubeconfig for audit webhook from referenced secret %s: %w", key, err)
	}

	return &apiserver.AuditWebhook{
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
	kubeconfig, err := gardenerutils.FetchKubeconfigFromSecret(ctx, r.RuntimeClientSet.Client(), key)
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
			{Name: "tcp", Port: 443, TargetPort: intstr.FromInt32(9443)},
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

	return kubeapiserverexposure.NewSNI(
		r.RuntimeClientSet.Client(),
		r.RuntimeClientSet.Applier(),
		namePrefix+v1beta1constants.DeploymentNameKubeAPIServer,
		r.GardenNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				Hosts: getAPIServerDomains(garden.Spec.VirtualCluster.DNS.Domains),
				IstioIngressGateway: kubeapiserverexposure.IstioIngressGateway{
					Namespace: ingressGatewayValues[0].Namespace,
					Labels:    ingressGatewayValues[0].Labels,
				},
			}
		},
	), nil
}

func (r *Reconciler) newGardenerAccess(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) component.DeployWaiter {
	return gardeneraccess.New(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		gardeneraccess.Values{
			ServerInCluster:    fmt.Sprintf("%s%s.%s.svc.cluster.local", namePrefix, v1beta1constants.DeploymentNameKubeAPIServer, r.GardenNamespace),
			ServerOutOfCluster: gardenerutils.GetAPIServerDomain(garden.Spec.VirtualCluster.DNS.Domains[0]),
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

func (r *Reconciler) newNginxIngressController(garden *operatorv1alpha1.Garden) (component.DeployWaiter, error) {
	providerConfig, err := getNginxIngressConfig(garden)
	if err != nil {
		return nil, err
	}

	return sharedcomponent.NewNginxIngress(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.GardenNamespace,
		r.RuntimeVersion,
		providerConfig,
		getLoadBalancerServiceAnnotations(garden),
		nil,
		v1beta1constants.PriorityClassNameGardenSystem300,
		true,
		true,
		component.ClusterTypeSeed,
		"",
		v1beta1constants.SeedNginxIngressClass,
	)
}

func (r *Reconciler) newGardenerMetricsExporter(secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerMetricsExporter)
	if err != nil {
		return nil, err
	}

	return gardenermetricsexporter.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, gardenermetricsexporter.Values{Image: image.String()}), nil
}

func (r *Reconciler) newPlutono(secretsManager secretsmanager.Interface, ingressDomain string, wildcardCert *corev1.Secret) (plutono.Interface, error) {
	var wildcardCertName *string
	if wildcardCert != nil {
		wildcardCertName = pointer.String(wildcardCert.GetName())
	}

	return sharedcomponent.NewPlutono(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		component.ClusterTypeSeed,
		1,
		"",
		fmt.Sprintf("%s.%s", "plutono-garden", ingressDomain),
		v1beta1constants.PriorityClassNameGardenSystem100,
		false,
		false,
		true,
		false,
		false,
		false,
		wildcardCertName,
	)
}

func getNginxIngressConfig(garden *operatorv1alpha1.Garden) (map[string]string, error) {
	var (
		defaultConfig = map[string]interface{}{
			"enable-vts-status":            "false",
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
			"allow-snippet-annotations":    "false",
		}
		providerConfig = map[string]interface{}{}
	)

	if garden.Spec.RuntimeCluster.Ingress.Controller.ProviderConfig != nil {
		if err := json.Unmarshal(garden.Spec.RuntimeCluster.Ingress.Controller.ProviderConfig.Raw, &providerConfig); err != nil {
			return nil, err
		}
	}

	return utils.InterfaceMapToStringMap(utils.MergeMaps(defaultConfig, providerConfig)), nil
}

// GetLoadBalancerServiceAnnotations returns the load balancer annotations set for the garden if any.
func getLoadBalancerServiceAnnotations(garden *operatorv1alpha1.Garden) map[string]string {
	if garden.Spec.RuntimeCluster.Settings != nil && garden.Spec.RuntimeCluster.Settings.LoadBalancerServices != nil {
		// return copy of annotations to prevent any accidental mutation by components
		return utils.MergeStringMaps(garden.Spec.RuntimeCluster.Settings.LoadBalancerServices.Annotations)
	}
	return nil
}

func (r *Reconciler) newVirtualSystem(enableSeedAuthorizer bool) component.DeployWaiter {
	return virtualgardensystem.New(r.RuntimeClientSet.Client(), r.GardenNamespace, virtualgardensystem.Values{SeedAuthorizerEnabled: enableSeedAuthorizer})
}

func (r *Reconciler) newGardenerAPIServer(ctx context.Context, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (gardenerapiserver.Interface, error) {
	var (
		err                error
		apiServerConfig    *operatorv1alpha1.GardenerAPIServerConfig
		auditWebhookConfig *apiserver.AuditWebhook
	)

	if apiServer := garden.Spec.VirtualCluster.Gardener.APIServer; apiServer != nil {
		apiServerConfig = apiServer

		auditWebhookConfig, err = r.computeAPIServerAuditWebhookConfig(ctx, apiServer.AuditWebhook)
		if err != nil {
			return nil, err
		}
	}

	return sharedcomponent.NewGardenerAPIServer(
		ctx,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		metav1.ObjectMeta{Namespace: r.GardenNamespace, Name: garden.Name},
		r.RuntimeVersion,
		secretsManager,
		apiServerConfig,
		defaultAPIServerAutoscalingConfig(garden),
		auditWebhookConfig,
		helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		garden.Spec.VirtualCluster.Gardener.ClusterIdentity,
	)
}

func (r *Reconciler) newGardenerAdmissionController(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, enableSeedRestriction bool) (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerAdmissionController)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	values := gardeneradmissioncontroller.Values{
		Image:                       image.String(),
		LogLevel:                    logger.InfoLevel,
		RuntimeVersion:              r.RuntimeVersion,
		SeedRestrictionEnabled:      enableSeedRestriction,
		TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
	}

	if config := garden.Spec.VirtualCluster.Gardener.AdmissionController; config != nil {
		values.ResourceAdmissionConfiguration = operatorv1alpha1conversion.ConvertToAdmissionControllerResourceAdmissionConfiguration(config.ResourceAdmissionConfiguration)
		if config.LogLevel != nil {
			values.LogLevel = *config.LogLevel
		}
	}

	return gardeneradmissioncontroller.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values), nil
}

func (r *Reconciler) newGardenerControllerManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerControllerManager)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	values := gardenercontrollermanager.Values{
		Image:    image.String(),
		LogLevel: logger.InfoLevel,
	}

	if config := garden.Spec.VirtualCluster.Gardener.ControllerManager; config != nil {
		values.FeatureGates = config.FeatureGates
		if config.LogLevel != nil {
			values.LogLevel = *config.LogLevel
		}

		for _, defaultProjectQuota := range config.DefaultProjectQuotas {
			values.Quotas = append(values.Quotas, controllermanagerv1alpha1.QuotaConfiguration{
				Config:          defaultProjectQuota.Config,
				ProjectSelector: defaultProjectQuota.ProjectSelector,
			})
		}
	}

	return gardenercontrollermanager.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values), nil
}

func (r *Reconciler) newGardenerScheduler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerScheduler)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	values := gardenerscheduler.Values{
		Image:    image.String(),
		LogLevel: logger.InfoLevel,
	}

	if config := garden.Spec.VirtualCluster.Gardener.Scheduler; config != nil {
		values.FeatureGates = config.FeatureGates
		if config.LogLevel != nil {
			values.LogLevel = *config.LogLevel
		}
	}

	return gardenerscheduler.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values), nil
}

func (r *Reconciler) newFluentOperator() (component.DeployWaiter, error) {
	return sharedcomponent.NewFluentOperator(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		true,
		v1beta1constants.PriorityClassNameGardenSystem100,
	)
}

func (r *Reconciler) newFluentBit() (component.DeployWaiter, error) {
	return sharedcomponent.NewFluentBit(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		true,
		v1beta1constants.PriorityClassNameGardenSystem100,
	)
}

func (r *Reconciler) newFluentCustomResources() (component.DeployWaiter, error) {
	return sharedcomponent.NewFluentOperatorCustomResources(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		true,
		"-garden",
		logging.GardenCentralLoggingConfigurations,
		customresources.GetStaticClusterOutput(map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource}),
	)
}

func (r *Reconciler) newVali(garden *operatorv1alpha1.Garden) (vali.Interface, error) {
	return sharedcomponent.NewVali(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		nil,
		component.ClusterTypeSeed,
		1,
		false,
		v1beta1constants.PriorityClassNameGardenSystem100,
		nil,
		"",
		false,
		hvpaEnabled(),
		&hvpav1alpha1.MaintenanceTimeWindow{
			Begin: garden.Spec.VirtualCluster.Maintenance.TimeWindow.Begin,
			End:   garden.Spec.VirtualCluster.Maintenance.TimeWindow.End,
		},
	)
}
