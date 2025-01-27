// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorv1alpha1conversion "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/conversion"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	extensioncrds "github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/extensions/extension"
	runtimegardensystem "github.com/gardener/gardener/pkg/component/garden/system/runtime"
	virtualgardensystem "github.com/gardener/gardener/pkg/component/garden/system/virtual"
	gardeneraccess "github.com/gardener/gardener/pkg/component/gardener/access"
	gardeneradmissioncontroller "github.com/gardener/gardener/pkg/component/gardener/admissioncontroller"
	gardenerapiserver "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	gardenercontrollermanager "github.com/gardener/gardener/pkg/component/gardener/controllermanager"
	gardenerdashboard "github.com/gardener/gardener/pkg/component/gardener/dashboard"
	"github.com/gardener/gardener/pkg/component/gardener/dashboard/terminal"
	gardenerdiscoveryserver "github.com/gardener/gardener/pkg/component/gardener/discoveryserver"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	gardenerscheduler "github.com/gardener/gardener/pkg/component/gardener/scheduler"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	kubeapiserverexposure "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/observability/logging"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	gardenblackboxexporter "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/gardenermetricsexporter"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	longtermprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/longterm"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
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
	etcdCRD       component.Deployer
	vpaCRD        component.Deployer
	istioCRD      component.Deployer
	fluentCRD     component.Deployer
	extensionCRD  component.Deployer
	prometheusCRD component.DeployWaiter

	gardenerResourceManager component.DeployWaiter
	runtimeSystem           component.DeployWaiter
	verticalPodAutoscaler   component.DeployWaiter
	etcdDruid               component.DeployWaiter
	istio                   istio.Interface
	nginxIngressController  component.DeployWaiter

	extensions extension.Interface

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

	gardenerDiscoveryServer component.DeployWaiter

	gardenerDashboard         gardenerdashboard.Interface
	terminalControllerManager component.DeployWaiter

	gardenerMetricsExporter       component.DeployWaiter
	kubeStateMetrics              component.DeployWaiter
	fluentOperator                component.DeployWaiter
	fluentBit                     component.DeployWaiter
	fluentOperatorCustomResources component.DeployWaiter
	plutono                       plutono.Interface
	vali                          component.Deployer
	prometheusOperator            component.DeployWaiter
	alertManager                  alertmanager.Interface
	prometheusGarden              prometheus.Interface
	prometheusLongTerm            prometheus.Interface
	blackboxExporter              component.DeployWaiter
}

func (r *Reconciler) instantiateComponents(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
	applier kubernetes.Applier,
	wildcardCertSecret *corev1.Secret,
	enableSeedAuthorizer bool,
) (
	c components,
	err error,
) {
	var wildcardCertSecretName *string
	if wildcardCertSecret != nil {
		wildcardCertSecretName = ptr.To(wildcardCertSecret.GetName())
	}

	// crds
	c.etcdCRD = etcd.NewCRD(r.RuntimeClientSet.Client(), applier)
	c.vpaCRD = vpa.NewCRD(applier, nil)
	c.istioCRD = istio.NewCRD(r.RuntimeClientSet.ChartApplier())
	c.fluentCRD = fluentoperator.NewCRDs(applier)
	c.prometheusCRD, err = prometheusoperator.NewCRDs(r.RuntimeClientSet.Client(), applier)
	if err != nil {
		return
	}
	c.extensionCRD = extensioncrds.NewCRD(applier, true, false)

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
	c.etcdDruid, err = r.newEtcdDruid(secretsManager)
	if err != nil {
		return
	}
	c.istio, err = r.newIstio(ctx, garden)
	if err != nil {
		return
	}
	c.nginxIngressController, err = r.newNginxIngressController(garden, c.istio.GetValues().IngressGateway)
	if err != nil {
		return
	}

	// garden extensions
	c.extensions, err = r.newExtensions(ctx, log, garden)
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

	primaryIngressDomain := garden.Spec.RuntimeCluster.Ingress.Domains[0]

	c.virtualSystem = r.newVirtualSystem(enableSeedAuthorizer)
	c.virtualGardenGardenerAccess = r.newGardenerAccess(garden, secretsManager)

	// gardener control plane components
	discoveryServerDomain := discoveryServerDomain(garden)
	workloadIdentityTokenIssuer := workloadIdentityTokenIssuerURL(garden)
	c.gardenerAPIServer, err = r.newGardenerAPIServer(ctx, garden, secretsManager, workloadIdentityTokenIssuer)
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
	c.gardenerDashboard, err = r.newGardenerDashboard(garden, secretsManager, wildcardCertSecretName)
	if err != nil {
		return
	}
	c.terminalControllerManager, err = r.newTerminalControllerManager(garden, secretsManager)
	if err != nil {
		return
	}
	c.gardenerDiscoveryServer, err = r.newGardenerDiscoveryServer(secretsManager, discoveryServerDomain, wildcardCertSecretName, workloadIdentityTokenIssuer)
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
	c.vali, err = r.newVali()
	if err != nil {
		return
	}
	c.plutono, err = r.newPlutono(garden, secretsManager, primaryIngressDomain.Name, wildcardCertSecretName)
	if err != nil {
		return
	}
	c.prometheusOperator, err = r.newPrometheusOperator()
	if err != nil {
		return
	}
	c.alertManager, err = r.newAlertmanager(log, garden, secretsManager, primaryIngressDomain.Name, wildcardCertSecretName)
	if err != nil {
		return
	}
	c.prometheusGarden, err = r.newPrometheusGarden(log, garden, secretsManager, primaryIngressDomain.Name, wildcardCertSecretName)
	if err != nil {
		return
	}
	c.prometheusLongTerm, err = r.newPrometheusLongTerm(log, garden, secretsManager, primaryIngressDomain.Name, wildcardCertSecretName)
	if err != nil {
		return
	}
	c.blackboxExporter, err = r.newBlackboxExporter(garden, secretsManager, wildcardCertSecretName)
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
	// TODO(rfranzke): Consider removing this two-step deployment once we only support Kubernetes 1.32+ (in this
	//  version, the structured authorization feature has been promoted to GA). We already use structured authz for
	//  1.30+ clusters. See https://github.com/gardener/gardener/pull/10682#discussion_r1816324389 for more information.
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

	return sharedcomponent.NewRuntimeGardenerResourceManager(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, resourcemanager.Values{
		DefaultSeccompProfileEnabled:              features.DefaultFeatureGate.Enabled(features.DefaultSeccompProfile),
		DefaultNotReadyToleration:                 defaultNotReadyTolerationSeconds,
		DefaultUnreachableToleration:              defaultUnreachableTolerationSeconds,
		EndpointSliceHintsEnabled:                 helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
		LogLevel:                                  r.Config.LogLevel,
		LogFormat:                                 r.Config.LogFormat,
		ManagedResourceLabels:                     map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)},
		NetworkPolicyAdditionalNamespaceSelectors: r.Config.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
		PriorityClassName:                         v1beta1constants.PriorityClassNameGardenSystemCritical,
		SecretNameServerCA:                        operatorv1alpha1.SecretNameCARuntime,
		Zones:                                     garden.Spec.RuntimeCluster.Provider.Zones,
	})
}

func (r *Reconciler) newVirtualGardenGardenerResourceManager(secretsManager secretsmanager.Interface) (resourcemanager.Interface, error) {
	return sharedcomponent.NewTargetGardenerResourceManager(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, resourcemanager.Values{
		IsWorkerless:       true,
		LogLevel:           r.Config.LogLevel,
		LogFormat:          r.Config.LogFormat,
		NamePrefix:         namePrefix,
		PriorityClassName:  v1beta1constants.PriorityClassNameGardenSystem400,
		SecretNameServerCA: operatorv1alpha1.SecretNameCARuntime,
		TargetNamespaces:   []string{v1beta1constants.GardenNamespace, metav1.NamespaceSystem, gardencorev1beta1.GardenerShootIssuerNamespace, gardencorev1beta1.GardenerSystemPublicNamespace},
	})
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
		true,
	)
}

func (r *Reconciler) newEtcdDruid(secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	return sharedcomponent.NewEtcdDruid(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		r.ComponentImageVectors,
		r.Config.Controllers.Garden.ETCDConfig,
		secretsManager,
		operatorv1alpha1.SecretNameCARuntime,
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
		evictionRequirement           *string
		defragmentationScheduleFormat string
		storageClassName              *string
		storageCapacity               string
		minAllowed                    corev1.ResourceList
	)

	switch role {
	case v1beta1constants.ETCDRoleMain:
		evictionRequirement = ptr.To(v1beta1constants.EvictionRequirementNever)
		defragmentationScheduleFormat = "%d %d * * *" // defrag main etcd daily in the maintenance window
		storageCapacity = "25Gi"
		minAllowed = helper.GetMinAllowedForETCDMain(garden.Spec.VirtualCluster.ETCD)
		if etcd := garden.Spec.VirtualCluster.ETCD; etcd != nil && etcd.Main != nil && etcd.Main.Storage != nil {
			storageClassName = etcd.Main.Storage.ClassName
			if etcd.Main.Storage.Capacity != nil {
				storageCapacity = etcd.Main.Storage.Capacity.String()
			}
		}

	case v1beta1constants.ETCDRoleEvents:
		evictionRequirement = ptr.To(v1beta1constants.EvictionRequirementInMaintenanceWindowOnly)
		defragmentationScheduleFormat = "%d %d */3 * *"
		storageCapacity = "10Gi"
		minAllowed = helper.GetMinAllowedForETCDEvents(garden.Spec.VirtualCluster.ETCD)
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

	replicas := ptr.To[int32](1)
	if highAvailabilityEnabled {
		replicas = ptr.To[int32](3)
	}

	return etcd.New(
		log,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		etcd.Values{
			NamePrefix:                  namePrefix,
			Role:                        role,
			Class:                       class,
			Replicas:                    replicas,
			Autoscaling:                 etcd.AutoscalingConfig{MinAllowed: minAllowed},
			StorageCapacity:             storageCapacity,
			StorageClassName:            storageClassName,
			DefragmentationSchedule:     &defragmentationSchedule,
			CARotationPhase:             helper.GetCARotationPhase(garden.Status.Credentials),
			MaintenanceTimeWindow:       garden.Spec.VirtualCluster.Maintenance.TimeWindow,
			EvictionRequirement:         evictionRequirement,
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
		authorizationWebhookConfigs  []kubeapiserver.AuthorizationWebhook
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

		authorizationWebhookConfigs = append(authorizationWebhookConfigs, kubeapiserver.AuthorizationWebhook{
			Name:       "seed-authorizer",
			Kubeconfig: kubeconfig,
			WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
				// Set TTL to a very low value since it cannot be set to 0 because of defaulting.
				// See https://github.com/kubernetes/apiserver/blob/3658357fea9fa8b36173d072f2d548f135049e05/pkg/apis/apiserver/v1beta1/defaults.go#L29-L36
				AuthorizedTTL:                            metav1.Duration{Duration: 1 * time.Nanosecond},
				UnauthorizedTTL:                          metav1.Duration{Duration: 1 * time.Nanosecond},
				Timeout:                                  metav1.Duration{Duration: 10 * time.Second},
				FailurePolicy:                            apiserverv1beta1.FailurePolicyDeny,
				SubjectAccessReviewVersion:               "v1",
				MatchConditionSubjectAccessReviewVersion: "v1",
				MatchConditions: []apiserverv1beta1.WebhookMatchCondition{{
					// only intercept request from gardenlets and service accounts from seed namespaces
					Expression: fmt.Sprintf("'%s' in request.groups || request.groups.exists(e, e.startsWith('%s%s'))", v1beta1constants.SeedsGroup, serviceaccount.ServiceAccountGroupPrefix, gardenerutils.SeedNamespaceNamePrefix),
				}},
			},
		})
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
		kubeapiserver.VPNConfig{Enabled: false},
		v1beta1constants.PriorityClassNameGardenSystem500,
		true,
		auditWebhookConfig,
		authenticationWebhookConfig,
		authorizationWebhookConfigs,
		resourcesToStoreInETCDEvents,
	)
}

func defaultAPIServerAutoscalingConfig(garden *operatorv1alpha1.Garden) apiserver.AutoscalingConfig {
	minReplicas := int32(2)
	if helper.HighAvailabilityEnabled(garden) {
		minReplicas = 3
	}

	return apiserver.AutoscalingConfig{
		APIServerResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("600m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		MinReplicas:       minReplicas,
		MaxReplicas:       6,
		ScaleDownDisabled: false,
		MinAllowed:        helper.GetMinAllowedForKubeAPIServer(garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer),
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
		certificateSigningDuration = ptr.To(controllerManager.CertificateSigningDuration.Duration)
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
		false,
		certificateSigningDuration,
		kubecontrollermanager.ControllerWorkers{
			GarbageCollector:    ptr.To(250),
			Namespace:           ptr.To(100),
			ResourceQuota:       ptr.To(100),
			ServiceAccountToken: ptr.To(0),
		},
		kubecontrollermanager.ControllerSyncPeriods{
			ResourceQuota: ptr.To(time.Minute),
		},
		map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)},
	)
}

func (r *Reconciler) newKubeStateMetrics() (component.DeployWaiter, error) {
	return sharedcomponent.NewKubeStateMetrics(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		r.RuntimeVersion,
		v1beta1constants.PriorityClassNameGardenSystem100,
		kubestatemetrics.SuffixRuntime,
	)
}

func (r *Reconciler) newIstio(ctx context.Context, garden *operatorv1alpha1.Garden) (istio.Interface, error) {
	var annotations map[string]string
	if settings := garden.Spec.RuntimeCluster.Settings; settings != nil && settings.LoadBalancerServices != nil {
		annotations = settings.LoadBalancerServices.Annotations
	}

	return sharedcomponent.NewIstio(
		ctx,
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
		nil,
		false,
		garden.Spec.RuntimeCluster.Provider.Zones,
		false,
	)
}

func (r *Reconciler) newSNI(garden *operatorv1alpha1.Garden, ingressGatewayValues []istio.IngressGatewayValues) (component.Deployer, error) {
	if len(ingressGatewayValues) != 1 {
		return nil, fmt.Errorf("exactly one Istio Ingress Gateway is required for the SNI config")
	}

	domains := toDomainNames(getAPIServerDomains(garden.Spec.VirtualCluster.DNS.Domains))
	return kubeapiserverexposure.NewSNI(
		r.RuntimeClientSet.Client(),
		namePrefix+v1beta1constants.DeploymentNameKubeAPIServer,
		r.GardenNamespace,
		func() *kubeapiserverexposure.SNIValues {
			return &kubeapiserverexposure.SNIValues{
				Hosts: domains,
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
			ServerInCluster:       fmt.Sprintf("%s%s.%s.svc.cluster.local", namePrefix, v1beta1constants.DeploymentNameKubeAPIServer, r.GardenNamespace),
			ServerOutOfCluster:    gardenerutils.GetAPIServerDomain(garden.Spec.VirtualCluster.DNS.Domains[0].Name),
			ManagedResourceLabels: map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)},
		},
	)
}

const gardenerDNSNamePrefix = "gardener."

func toDomainNames(domains []operatorv1alpha1.DNSDomain) []string {
	domainNames := make([]string, 0, len(domains))
	for _, domain := range domains {
		domainNames = append(domainNames, domain.Name)
	}
	return domainNames
}

func getAPIServerDomains(domains []operatorv1alpha1.DNSDomain) []operatorv1alpha1.DNSDomain {
	apiServerDomains := make([]operatorv1alpha1.DNSDomain, 0, len(domains)*2)
	for _, domain := range domains {
		apiServerDomains = append(apiServerDomains,
			operatorv1alpha1.DNSDomain{
				Name:     gardenerutils.GetAPIServerDomain(domain.Name),
				Provider: domain.Provider,
			},
			operatorv1alpha1.DNSDomain{
				Name:     gardenerDNSNamePrefix + domain.Name,
				Provider: domain.Provider,
			})
	}
	return apiServerDomains
}

func getIngressWildcardDomains(domains []operatorv1alpha1.DNSDomain) []operatorv1alpha1.DNSDomain {
	wildcardDomains := make([]operatorv1alpha1.DNSDomain, 0, len(domains))
	for _, domain := range domains {
		wildcardDomains = append(wildcardDomains,
			operatorv1alpha1.DNSDomain{
				Name:     "*." + domain.Name,
				Provider: domain.Provider,
			})
	}
	return wildcardDomains
}

func (r *Reconciler) newNginxIngressController(garden *operatorv1alpha1.Garden, ingressGatewayValues []istio.IngressGatewayValues) (component.DeployWaiter, error) {
	providerConfig, err := getNginxIngressConfig(garden)
	if err != nil {
		return nil, err
	}

	if len(ingressGatewayValues) != 1 {
		return nil, fmt.Errorf("exactly one Istio Ingress Gateway is required for the SNI config")
	}

	ingressDomains := toDomainNames(getIngressWildcardDomains(garden.Spec.RuntimeCluster.Ingress.Domains))

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
		component.ClusterTypeSeed,
		"",
		v1beta1constants.SeedNginxIngressClass,
		ingressDomains,
		ingressGatewayValues[0].Labels,
	)
}

func (r *Reconciler) newGardenerMetricsExporter(secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerMetricsExporter)
	if err != nil {
		return nil, err
	}

	return gardenermetricsexporter.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, gardenermetricsexporter.Values{Image: image.String()}), nil
}

func (r *Reconciler) newPlutono(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, ingressDomain string, wildcardCertSecretName *string) (plutono.Interface, error) {
	return sharedcomponent.NewPlutono(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		component.ClusterTypeSeed,
		1,
		"",
		"plutono-garden."+ingressDomain,
		v1beta1constants.PriorityClassNameGardenSystem100,
		false,
		false,
		true,
		false,
		vpaEnabled(garden.Spec.RuntimeCluster.Settings),
		wildcardCertSecretName,
	)
}

func getNginxIngressConfig(garden *operatorv1alpha1.Garden) (map[string]string, error) {
	var (
		defaultConfig = map[string]any{
			"enable-vts-status":            "false",
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
			"allow-snippet-annotations":    "false",
			// This is needed to override the default which is "High" starting from nginx-ingress-controller v1.12.0
			// and we use the nginx.ingress.kubernetes.io/server-snippet annotation in our plutono and alertmanager ingresses.
			// This is acceptable for the seed as we control the ingress resources solely and no malicious configuration can be injected by users.
			// See https://github.com/gardener/gardener/pull/11087 for more details.
			"annotations-risk-level": "Critical",
		}
		providerConfig = map[string]any{}
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

func (r *Reconciler) newGardenerAPIServer(ctx context.Context, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, workloadIdentityTokenIssuer string) (gardenerapiserver.Interface, error) {
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
		workloadIdentityTokenIssuer,
	)
}

func (r *Reconciler) newGardenerAdmissionController(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, enableSeedRestriction bool) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerAdmissionController)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	values := gardeneradmissioncontroller.Values{
		Image:                       image.String(),
		LogLevel:                    logger.InfoLevel,
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
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerControllerManager)
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
			values.Quotas = append(values.Quotas, controllermanagerconfigv1alpha1.QuotaConfiguration{
				Config:          defaultProjectQuota.Config,
				ProjectSelector: defaultProjectQuota.ProjectSelector,
			})
		}
	}

	return gardenercontrollermanager.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values), nil
}

func (r *Reconciler) newGardenerScheduler(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerScheduler)
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

func (r *Reconciler) newGardenerDashboard(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, wildcardCertSecretName *string) (gardenerdashboard.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerDashboard)
	if err != nil {
		return nil, err
	}

	values := gardenerdashboard.Values{
		Image:            image.String(),
		LogLevel:         logger.InfoLevel,
		APIServerURL:     gardenerutils.GetAPIServerDomain(garden.Spec.VirtualCluster.DNS.Domains[0].Name),
		EnableTokenLogin: true,
		Ingress: gardenerdashboard.IngressValues{
			Domains:                domainNames(garden.Spec.RuntimeCluster.Ingress.Domains),
			WildcardCertSecretName: wildcardCertSecretName,
		},
	}

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI != nil {
		values.APIServerURL = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI.DomainPatterns[0]
	}

	if config := garden.Spec.VirtualCluster.Gardener.Dashboard; config != nil {
		if config.LogLevel != nil {
			values.LogLevel = *config.LogLevel
		}

		if config.EnableTokenLogin != nil {
			values.EnableTokenLogin = *config.EnableTokenLogin
		}

		if config.Terminal != nil {
			values.Terminal = &gardenerdashboard.TerminalValues{DashboardTerminal: *config.Terminal}
		}

		if config.OIDCConfig != nil {
			issuerURL := config.OIDCConfig.IssuerURL
			if issuerURL == nil {
				issuerURL = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL
			}

			clientIDPublic := config.OIDCConfig.ClientIDPublic
			if clientIDPublic == nil {
				clientIDPublic = garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.OIDCConfig.ClientID
			}

			values.OIDC = &gardenerdashboard.OIDCValues{
				DashboardOIDC:  *config.OIDCConfig,
				IssuerURL:      *issuerURL,
				ClientIDPublic: *clientIDPublic,
			}
		}

		values.GitHub = config.GitHub

		if config.FrontendConfigMapRef != nil {
			values.FrontendConfigMapName = &config.FrontendConfigMapRef.Name
		}
		if config.AssetsConfigMapRef != nil {
			values.AssetsConfigMapName = &config.AssetsConfigMapRef.Name
		}
	}

	return gardenerdashboard.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values), nil
}

func (r *Reconciler) newTerminalControllerManager(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameTerminalControllerManager)
	if err != nil {
		return nil, err
	}

	values := terminal.Values{
		Image:                       image.String(),
		TopologyAwareRoutingEnabled: helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings),
	}

	deployer := terminal.New(r.RuntimeClientSet.Client(), r.GardenNamespace, secretsManager, values)
	if config := garden.Spec.VirtualCluster.Gardener.Dashboard; config == nil || config.Terminal == nil {
		deployer = component.OpDestroyAndWait(deployer)
	}

	return deployer, nil
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
		fluentcustomresources.GetStaticClusterOutput(map[string]string{v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource}),
	)
}

func (r *Reconciler) newVali() (component.Deployer, error) {
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
	)
}

func (r *Reconciler) newPrometheusOperator() (component.DeployWaiter, error) {
	return sharedcomponent.NewPrometheusOperator(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		v1beta1constants.PriorityClassNameGardenSystem100,
	)
}

func (r *Reconciler) newAlertmanager(log logr.Logger, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, ingressDomain string, wildcardCertSecretName *string) (alertmanager.Interface, error) {
	return sharedcomponent.NewAlertmanager(log, r.RuntimeClientSet.Client(), r.GardenNamespace, alertmanager.Values{
		Name:              "garden",
		ClusterType:       component.ClusterTypeSeed,
		PriorityClassName: v1beta1constants.PriorityClassNameGardenSystem100,
		StorageCapacity:   resource.MustParse(getValidVolumeSize(garden.Spec.RuntimeCluster.Volume, "1Gi")),
		Replicas:          2,
		RuntimeVersion:    r.RuntimeVersion,
		Ingress: &alertmanager.IngressValues{
			Host:                   "alertmanager-garden." + ingressDomain,
			SecretsManager:         secretsManager,
			SigningCA:              operatorv1alpha1.SecretNameCARuntime,
			WildcardCertSecretName: wildcardCertSecretName,
		},
	})
}

func (r *Reconciler) newPrometheusGarden(log logr.Logger, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, ingressDomain string, wildcardCertSecretName *string) (prometheus.Interface, error) {
	return sharedcomponent.NewPrometheus(log, r.RuntimeClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "garden",
		PriorityClassName: v1beta1constants.PriorityClassNameGardenSystem100,
		StorageCapacity:   resource.MustParse(getValidVolumeSize(garden.Spec.RuntimeCluster.Volume, "200Gi")),
		Replicas:          2,
		Retention:         ptr.To(monitoringv1.Duration("10d")),
		RetentionSize:     "190GB",
		ScrapeTimeout:     "50s", // This is intentionally smaller than the scrape interval of 1m.
		RuntimeVersion:    r.RuntimeVersion,
		ExternalLabels:    map[string]string{"landscape": garden.Spec.VirtualCluster.Gardener.ClusterIdentity},
		AdditionalPodLabels: map[string]string{
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:                                                v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                               v1beta1constants.LabelNetworkPolicyAllowed,
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyGardenScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyExtensionsNamespaceAlias + "-" + v1beta1constants.LabelNetworkPolicyGardenScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
		},
		CentralConfigs: prometheus.CentralConfigs{
			AdditionalScrapeConfigs: gardenprometheus.AdditionalScrapeConfigs(),
			PrometheusRules:         gardenprometheus.CentralPrometheusRules(garden.Spec.VirtualCluster.Gardener.DiscoveryServer != nil),
			ServiceMonitors:         gardenprometheus.CentralServiceMonitors(),
		},
		Alerting: &prometheus.AlertingValues{Alertmanagers: []*prometheus.Alertmanager{{Name: "alertmanager-garden"}}},
		AdditionalAlertRelabelConfigs: []monitoringv1.RelabelConfig{
			{
				SourceLabels: []monitoringv1.LabelName{"project", "name"},
				Regex:        "(.+);(.+)",
				Action:       "replace",
				Replacement:  ptr.To("https://dashboard." + ingressDomain + "/namespace/garden-$1/shoots/$2"),
				TargetLabel:  "shoot_dashboard_url",
			},
			{
				SourceLabels: []monitoringv1.LabelName{"project", "name"},
				Regex:        "garden;(.+)",
				Action:       "replace",
				Replacement:  ptr.To("https://dashboard." + ingressDomain + "/namespace/garden/shoots/$1"),
				TargetLabel:  "shoot_dashboard_url",
			},
		},
		Ingress: &prometheus.IngressValues{
			Host:                   "prometheus-garden." + ingressDomain,
			SecretsManager:         secretsManager,
			SigningCA:              operatorv1alpha1.SecretNameCARuntime,
			WildcardCertSecretName: wildcardCertSecretName,
		},
		TargetCluster: &prometheus.TargetClusterValues{ServiceAccountName: gardenprometheus.ServiceAccountName},
	})
}

func (r *Reconciler) newPrometheusLongTerm(log logr.Logger, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, ingressDomain string, wildcardCertSecretName *string) (prometheus.Interface, error) {
	imageCortex, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameCortex)
	if err != nil {
		return nil, err
	}

	return sharedcomponent.NewPrometheus(log, r.RuntimeClientSet.Client(), r.GardenNamespace, prometheus.Values{
		Name:              "longterm",
		PriorityClassName: v1beta1constants.PriorityClassNameGardenSystem100,
		StorageCapacity:   resource.MustParse(getValidVolumeSize(garden.Spec.RuntimeCluster.Volume, "100Gi")),
		Replicas:          2,
		RetentionSize:     "80GB",
		ScrapeTimeout:     "50s", // This is intentionally smaller than the scrape interval of 1m.
		RuntimeVersion:    r.RuntimeVersion,
		AdditionalPodLabels: map[string]string{
			gardenerutils.NetworkPolicyLabel("prometheus-garden", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
		},
		CentralConfigs: prometheus.CentralConfigs{
			PrometheusRules: longtermprometheus.CentralPrometheusRules(),
			ScrapeConfigs:   longtermprometheus.CentralScrapeConfigs(),
		},
		Ingress: &prometheus.IngressValues{
			Host:                   "prometheus-longterm." + ingressDomain,
			SecretsManager:         secretsManager,
			SigningCA:              operatorv1alpha1.SecretNameCARuntime,
			WildcardCertSecretName: wildcardCertSecretName,
		},
		Cortex: &prometheus.CortexValues{
			Image:         imageCortex.String(),
			CacheValidity: 7 * 24 * time.Hour, // 1 week
		},
	})
}

func (r *Reconciler) newBlackboxExporter(garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface, wildcardCertSecretName *string) (component.DeployWaiter, error) {
	var (
		primaryVirtualDNSDomain = garden.Spec.VirtualCluster.DNS.Domains[0].Name
		primaryIngressDomain    = garden.Spec.RuntimeCluster.Ingress.Domains[0].Name
		kubeAPIServerTargets    = []monitoringv1alpha1.Target{monitoringv1alpha1.Target("https://" + gardenerDNSNamePrefix + primaryVirtualDNSDomain + "/healthz")}
		gardenerDashboardTarget = monitoringv1alpha1.Target("https://dashboard." + primaryIngressDomain + "/healthz")
	)

	if garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer != nil && garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI != nil {
		for _, domainPattern := range garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.SNI.DomainPatterns {
			if !strings.Contains(domainPattern, "*") {
				kubeAPIServerTargets = append(kubeAPIServerTargets, monitoringv1alpha1.Target("https://"+domainPattern+"/healthz"))
			}
		}
	}

	// See function 'gardenerDashboard.ingress(ctx)' in pkg/component/gardener/dashboard/ingress.go
	isDashboardCertificateIssuedByGardener := wildcardCertSecretName == nil

	return sharedcomponent.NewBlackboxExporter(
		r.RuntimeClientSet.Client(),
		secretsManager,
		r.GardenNamespace,
		blackboxexporter.Values{
			ClusterType:     component.ClusterTypeSeed,
			IsGardenCluster: true,
			VPAEnabled:      true,
			PodLabels: map[string]string{
				v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(v1beta1constants.LabelNetworkPolicyIstioIngressNamespaceAlias+"-"+v1beta1constants.DefaultSNIIngressServiceName, 9443): v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(gardenerapiserver.DeploymentName, 8443):                                                                                v1beta1constants.LabelNetworkPolicyAllowed,
				gardenerutils.NetworkPolicyLabel(gardenerdiscoveryserver.ServiceName, 8081):                                                                             v1beta1constants.LabelNetworkPolicyAllowed,
			},
			PriorityClassName: v1beta1constants.PriorityClassNameGardenSystem100,
			Config:            gardenblackboxexporter.Config(isDashboardCertificateIssuedByGardener, garden.Spec.VirtualCluster.Gardener.DiscoveryServer != nil),
			ScrapeConfigs:     gardenblackboxexporter.ScrapeConfig(r.GardenNamespace, kubeAPIServerTargets, gardenerDashboardTarget),
			Replicas:          1,
		},
	)
}

func (r *Reconciler) newGardenerDiscoveryServer(
	secretsManager secretsmanager.Interface,
	domain string,
	wildcardCertSecretName *string,
	workloadIdentityTokenIssuer string,
) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerDiscoveryServer)
	if err != nil {
		return nil, err
	}

	return gardenerdiscoveryserver.New(
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		secretsManager,
		gardenerdiscoveryserver.Values{
			Image:                       image.String(),
			Domain:                      domain,
			TLSSecretName:               wildcardCertSecretName,
			WorkloadIdentityTokenIssuer: workloadIdentityTokenIssuer,
		},
	), nil
}

func domainNames(domains []operatorv1alpha1.DNSDomain) []string {
	names := make([]string, 0, len(domains))
	for _, domain := range domains {
		names = append(names, domain.Name)
	}
	return names
}

func (r *Reconciler) newExtensions(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden) (extension.Interface, error) {
	values := &extension.Values{
		Namespace:  r.GardenNamespace,
		Extensions: make(map[string]extension.Extension),
	}

	// Transform extension definition from Garden to extensionsv1alpha1.Extension resource.
	for _, ext := range garden.Spec.Extensions {
		values.Extensions[ext.Type] = extension.Extension{
			Extension: extensionsv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ext.Type,
					Namespace: r.GardenNamespace,
				},
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type:           ext.Type,
						Class:          ptr.To(extensionsv1alpha1.ExtensionClassGarden),
						ProviderConfig: ext.ProviderConfig,
					},
				},
			},
			Timeout: extension.DefaultTimeout,
		}
	}

	extensions := &operatorv1alpha1.ExtensionList{}
	if err := r.RuntimeClientSet.Client().List(ctx, extensions); err != nil {
		return nil, fmt.Errorf("error calculating extensions: %w", err)
	}

	// Apply resource specific settings from operatorv1alpha1.Extension resource.
	for _, ext := range extensions.Items {
		for _, res := range ext.Spec.Resources {
			wantedExtension, ok := values.Extensions[res.Type]
			if !ok || res.Kind != extensionsv1alpha1.ExtensionResource {
				continue
			}

			wantedExtension.Lifecycle = res.Lifecycle
			if res.ReconcileTimeout != nil {
				wantedExtension.Timeout = res.ReconcileTimeout.Duration
			}

			values.Extensions[res.Type] = wantedExtension
		}
	}

	return extension.New(log, r.RuntimeClientSet.Client(), values, extension.DefaultInterval, extension.DefaultSevereThreshold, extension.DefaultTimeout), nil
}

func discoveryServerDomain(garden *operatorv1alpha1.Garden) string {
	return "discovery." + garden.Spec.RuntimeCluster.Ingress.Domains[0].Name
}

func workloadIdentityTokenIssuerURL(garden *operatorv1alpha1.Garden) string {
	return "https://" + discoveryServerDomain(garden) + "/garden/workload-identity/issuer"
}
