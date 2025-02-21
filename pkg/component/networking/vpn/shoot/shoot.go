// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	labelValue = "vpn-shoot"

	managedResourceName = "shoot-core-vpn-shoot"
	name                = "vpn-shoot"
	deploymentName      = "vpn-shoot"
	containerName       = "vpn-shoot"
	initContainerName   = "vpn-shoot-init"
	serviceName         = "vpn-shoot"

	volumeName          = "vpn-shoot"
	volumeNameTLSAuth   = "vpn-shoot-tlsauth"
	volumeNameDevNetTun = "dev-net-tun"

	volumeMountPathSecret    = "/srv/secrets/vpn-client" // #nosec G101 -- No credential.
	volumeMountPathSecretTLS = "/srv/secrets/tlsauth"    // #nosec G101 -- No credential.
	volumeMountPathDevNetTun = "/dev/net/tun"
)

// ReversedVPNValues contains the configuration values for the ReversedVPN.
type ReversedVPNValues struct {
	// Header is the header for the ReversedVPN.
	Header string
	// Endpoint is the endpoint for the ReversedVPN.
	Endpoint string
	// OpenVPNPort is the port for the ReversedVPN.
	OpenVPNPort int32
	// IPFamilies are the IPFamilies of the shoot.
	IPFamilies []gardencorev1beta1.IPFamily
}

// Values is a set of configuration values for the VPNShoot component.
type Values struct {
	// Image is the container image used for vpnShoot.
	Image string
	// PodAnnotations is the set of additional annotations to be used for the pods.
	PodAnnotations map[string]string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// VPAUpdateDisabled indicates whether the vertical pod autoscaler update should be disabled.
	VPAUpdateDisabled bool
	// ReversedVPN contains the configuration values for the ReversedVPN.
	ReversedVPN ReversedVPNValues
	// SeedPodNetwork is the pod CIDR of the seed.
	SeedPodNetwork string
	// HighAvailabilityEnabled marks whether HA is enabled for VPN.
	HighAvailabilityEnabled bool
	// HighAvailabilityNumberOfSeedServers is the number of VPN seed servers used for HA.
	HighAvailabilityNumberOfSeedServers int
	// HighAvailabilityNumberOfShootClients is the number of VPN shoot clients used for HA.
	HighAvailabilityNumberOfShootClients int
	// DisableNewVPN disable new VPN implementation.
	// TODO(MartinWeindel) Remove after feature gate `NewVPN` gets promoted to GA.
	DisableNewVPN bool
}

// New creates a new instance of DeployWaiter for vpnshoot
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &vpnShoot{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type vpnShoot struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

type vpnSecret struct {
	name       string
	volumeName string
	mountPath  string
	secret     *corev1.Secret
}

func (v *vpnShoot) Deploy(ctx context.Context) error {
	scrapeConfig := v.emptyScrapeConfig()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			MetricsPath: ptr.To("/probe"),
			Params:      map[string][]string{"module": {"http_apiserver"}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRolePod,
				APIServer:  ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer),
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: shoot.AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				// This is needed because we do not fetch the correct cluster CA bundle right now
				TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					TargetLabel: "type",
					Replacement: ptr.To("seed"),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_name"},
					Action:       "keep",
					Regex:        `vpn-shoot-(0|.+-.+);vpn-shoot-init`,
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_name"},
					TargetLabel:  "__param_target",
					Regex:        `(.+);(.+)`,
					Replacement:  ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port) + `/api/v1/namespaces/kube-system/pods/${1}/log?container=${2}&tailLines=1`),
					Action:       "replace",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__param_target"},
					TargetLabel:  "instance",
					Action:       "replace",
				},
				{
					TargetLabel: "__address__",
					Replacement: ptr.To("blackbox-exporter:9115"),
					Action:      "replace",
				},
				{
					Action:      "replace",
					Replacement: ptr.To("tunnel-probe-apiserver-proxy"),
					TargetLabel: "job",
				},
			},
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
				"probe_http_status_code",
				"probe_success",
			),
		}
		return nil
	}); err != nil {
		return err
	}

	prometheusRule := v.emptyPrometheusRule()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, v.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "vpn.rules",
				Rules: []monitoringv1.Rule{
					{
						Alert: "VPNShootNoPods",
						Expr:  intstr.FromString(`kube_deployment_status_replicas_available{deployment="` + deploymentName + `"} == 0`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    "vpn",
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"description": "vpn-shoot deployment in Shoot cluster has 0 available pods. VPN won't work.",
							"summary":     "VPN Shoot deployment no pods",
						},
					},
					{
						Alert: "VPNHAShootNoPods",
						Expr:  intstr.FromString(`kube_statefulset_status_replicas_ready{statefulset="` + deploymentName + `"} == 0`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    "vpn",
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"description": "vpn-shoot statefulset in HA Shoot cluster has 0 available pods. VPN won't work.",
							"summary":     "VPN HA Shoot statefulset no pods",
						},
					},
					{
						Alert: "VPNProbeAPIServerProxyFailed",
						Expr:  intstr.FromString(`absent(probe_success{job="tunnel-probe-apiserver-proxy"}) == 1 or probe_success{job="tunnel-probe-apiserver-proxy"} == 0 or probe_http_status_code{job="tunnel-probe-apiserver-proxy"} != 200`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    "vpn-test",
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "all",
						},
						Annotations: map[string]string{
							"description": "The API Server proxy functionality is not working. Probably the vpn connection from an API Server pod to the vpn-shoot endpoint on the Shoot workers does not work.",
							"summary":     "API Server Proxy not usable",
						},
					},
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	var (
		config = &secretsutils.CertificateSecretConfig{
			Name:                        "vpn-shoot-client",
			CommonName:                  "vpn-shoot-client",
			CertType:                    secretsutils.ClientCert,
			SkipPublishingCACertificate: true,
		}
		signingCA = v1beta1constants.SecretNameCAVPN
	)

	secretCA, found := v.secretsManager.Get(signingCA)
	if !found {
		return fmt.Errorf("secret %q not found", signingCA)
	}

	var secrets []vpnSecret
	if !v.values.HighAvailabilityEnabled {
		secret, err := v.secretsManager.Generate(ctx, config, secretsmanager.SignedByCA(signingCA), secretsmanager.Rotate(secretsmanager.InPlace))
		if err != nil {
			return err
		}

		secrets = append(secrets, vpnSecret{
			name:       config.Name,
			volumeName: volumeName,
			mountPath:  volumeMountPathSecret,
			secret:     secret,
		})
	} else {
		for i := 0; i < v.values.HighAvailabilityNumberOfShootClients; i++ {
			config.Name = fmt.Sprintf("vpn-shoot-client-%d", i)
			config.CommonName = config.Name
			secret, err := v.secretsManager.Generate(ctx, config, secretsmanager.SignedByCA(signingCA), secretsmanager.Rotate(secretsmanager.InPlace))
			if err != nil {
				return err
			}
			secrets = append(secrets, vpnSecret{
				name:       config.Name,
				volumeName: fmt.Sprintf("%s-%d", volumeName, i),
				mountPath:  fmt.Sprintf("%s-%d", volumeMountPathSecret, i),
				secret:     secret,
			})
		}
	}

	data, err := v.computeResourcesData(secretCA, secrets)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, v.client, v.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (v *vpnShoot) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, v.client,
		v.emptyScrapeConfig(),
		v.emptyPrometheusRule(),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, v.client, v.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (v *vpnShoot) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *vpnShoot) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *vpnShoot) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("tunnel-probe-apiserver-proxy", v.namespace, shoot.Label)}
}

func (v *vpnShoot) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta("tunnel-probe-apiserver-proxy", v.namespace, shoot.Label)}
}

func (v *vpnShoot) computeResourcesData(secretCAVPN *corev1.Secret, secretsVPNShoot []vpnSecret) (map[string][]byte, error) {
	var (
		secretVPNSeedServerTLSAuth *corev1.Secret
		found                      bool
	)

	secretVPNSeedServerTLSAuth, found = v.secretsManager.Get(vpnseedserver.SecretNameTLSAuth)
	if !found {
		return nil, fmt.Errorf("secret %q not found", vpnseedserver.SecretNameTLSAuth)
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		secretCA = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot-ca",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretCAVPN.Data,
		}
		secretTLSAuth = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot-tlsauth",
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: secretVPNSeedServerTLSAuth.Data,
		}
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	utilruntime.Must(kubernetesutils.MakeUnique(secretCA))
	utilruntime.Must(kubernetesutils.MakeUnique(secretTLSAuth))

	for i, item := range secretsVPNShoot {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.name,
				Namespace: metav1.NamespaceSystem,
			},
			Type: corev1.SecretTypeOpaque,
			Data: item.secret.Data,
		}
		utilruntime.Must(kubernetesutils.MakeUnique(secret))
		secretsVPNShoot[i].secret = secret
	}

	var (
		vpa *vpaautoscalingv1.VerticalPodAutoscaler

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-vpn",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Allows the VPN to communicate with shoot components and makes " +
						"the VPN reachable from the seed.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{}},
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
			},
		}

		labels = map[string]string{
			v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
			v1beta1constants.LabelApp:       labelValue,
			managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
		}
		template = v.podTemplate(serviceAccount, secretsVPNShoot, secretCA, secretTLSAuth)

		networkPolicyFromSeed = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-from-seed",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress from the control plane to "+
						"pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyShootFromSeed, v1beta1constants.LabelNetworkPolicyAllowed),
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyShootFromSeed: v1beta1constants.LabelNetworkPolicyAllowed}},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: template.Labels,
						},
					}},
				}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			},
		}
		deploymentOrStatefulSet client.Object
	)

	if !v.values.HighAvailabilityEnabled {
		deploymentOrStatefulSet = v.deployment(labels, template)
	} else {
		deploymentOrStatefulSet = v.statefulSet(labels, template)
	}

	utilruntime.Must(references.InjectAnnotations(deploymentOrStatefulSet))

	if v.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		if v.values.VPAUpdateDisabled {
			vpaUpdateMode = vpaautoscalingv1.UpdateModeOff
		}
		controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		kind := "Deployment"
		if _, ok := deploymentOrStatefulSet.(*appsv1.StatefulSet); ok {
			kind = "StatefulSet"
		}
		containerNames := []string{containerName}
		if v.values.HighAvailabilityEnabled {
			containerNames = nil
			for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
				containerNames = append(containerNames, fmt.Sprintf("%s-s%d", containerName, i))
			}
			containerNames = append(containerNames, "tunnel-controller")
		}
		var containerPolicies []vpaautoscalingv1.ContainerResourcePolicy
		for _, name := range containerNames {
			containerPolicies = append(containerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName: name,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("10Mi"),
				},
				ControlledValues: &controlledValues,
			})
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-shoot",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       kind,
					Name:       deploymentOrStatefulSet.GetName(),
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: containerPolicies,
				},
			},
		}
	}

	var objects []client.Object
	for _, item := range secretsVPNShoot {
		objects = append(objects, item.secret)
	}

	if v.values.HighAvailabilityEnabled {
		objects = append(objects, v.podDisruptionBudget())
	}

	objects = append(objects,
		secretCA,
		secretTLSAuth,
		serviceAccount,
		networkPolicy,
		networkPolicyFromSeed,
		deploymentOrStatefulSet,
		clusterRole,
		clusterRoleBinding,
		vpa,
	)

	return registry.AddAllAndSerialize(objects...)
}

func (v *vpnShoot) podDisruptionBudget() client.Object {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    getLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
			Selector:                   &metav1.LabelSelector{MatchLabels: getLabels()},
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		},
	}

	return pdb
}

func (v *vpnShoot) podTemplate(serviceAccount *corev1.ServiceAccount, secrets []vpnSecret, secretCA, secretTLSAuth *corev1.Secret) *corev1.PodTemplateSpec {
	template := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
				v1beta1constants.LabelApp:       labelValue,
				managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				"type":                          "tunnel",
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           serviceAccount.Name,
			PriorityClassName:            "system-cluster-critical",
			DNSPolicy:                    corev1.DNSDefault,
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			InitContainers: v.getInitContainers(),
			Volumes:        v.getVolumes(secrets, secretCA, secretTLSAuth),
		},
	}

	if !v.values.HighAvailabilityEnabled {
		template.Spec.Containers = []corev1.Container{*v.container(secrets, nil)}
	} else {
		for i := 0; i < v.values.HighAvailabilityNumberOfSeedServers; i++ {
			template.Spec.Containers = append(template.Spec.Containers, *v.container(secrets, &i))
		}
		if !v.values.DisableNewVPN {
			template.Spec.Containers = append(template.Spec.Containers, *v.tunnelControllerContainer())
		}
	}

	return template
}

func (v *vpnShoot) container(secrets []vpnSecret, index *int) *corev1.Container {
	name := containerName
	if index != nil {
		name = fmt.Sprintf("%s-s%d", containerName, *index)
	}
	return &corev1.Container{
		Name:            name,
		Image:           v.values.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             v.getEnvVars(index),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: v.getResourceLimits(),
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged:               ptr.To(false),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
		VolumeMounts: v.getVolumeMounts(secrets),
	}
}

func (v *vpnShoot) tunnelControllerContainer() *corev1.Container {
	return &corev1.Container{
		Name:            "tunnel-controller",
		Image:           v.values.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/tunnel-controller"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("20Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged:               ptr.To(false),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
	}
}

func (v *vpnShoot) deployment(labels map[string]string, template *corev1.PodTemplateSpec) *appsv1.Deployment {
	var (
		intStrMax  = intstr.FromString("100%")
		intStrZero = intstr.FromString("0%")
		replicas   = 1
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: ptr.To[int32](2),
			Replicas:             ptr.To(int32(replicas)),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &intStrMax,
					MaxUnavailable: &intStrZero,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: *template,
		},
	}
}

func (v *vpnShoot) statefulSet(labels map[string]string, template *corev1.PodTemplateSpec) *appsv1.StatefulSet {
	replicas := v.values.HighAvailabilityNumberOfShootClients
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: metav1.NamespaceSystem,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			RevisionHistoryLimit: ptr.To[int32](2),
			Replicas:             ptr.To(int32(replicas)), // #nosec: G115 - There is a validation for `replicas` in `Deployments` and `StatefulSets` which limits their value range.
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: *template,
		},
	}
}

func getLabels() map[string]string {
	return map[string]string{v1beta1constants.LabelApp: labelValue}
}

func (v *vpnShoot) indexedReversedHeader(index *int) string {
	if index == nil {
		return v.values.ReversedVPN.Header
	}
	return strings.Replace(v.values.ReversedVPN.Header, "vpn-seed-server", fmt.Sprintf("vpn-seed-server-%d", *index), 1)
}

func (v *vpnShoot) getEnvVars(index *int) []corev1.EnvVar {
	var (
		envVariables []corev1.EnvVar
		ipFamilies   []string
	)
	for _, v := range v.values.ReversedVPN.IPFamilies {
		ipFamilies = append(ipFamilies, string(v))
	}
	envVariables = append(envVariables,
		corev1.EnvVar{
			Name:  "IP_FAMILIES",
			Value: strings.Join(ipFamilies, ","),
		},
		corev1.EnvVar{
			Name:  "ENDPOINT",
			Value: v.values.ReversedVPN.Endpoint,
		},
		corev1.EnvVar{
			Name:  "OPENVPN_PORT",
			Value: strconv.Itoa(int(v.values.ReversedVPN.OpenVPNPort)),
		},
		corev1.EnvVar{
			Name:  "REVERSED_VPN_HEADER",
			Value: v.indexedReversedHeader(index),
		},
		corev1.EnvVar{
			Name:  "IS_SHOOT_CLIENT",
			Value: "true",
		},
		corev1.EnvVar{
			Name:  "SEED_POD_NETWORK",
			Value: v.values.SeedPodNetwork,
		},
	)

	if index != nil {
		envVariables = append(envVariables,
			[]corev1.EnvVar{
				{
					Name:  "IS_HA",
					Value: "true",
				},
				{
					Name:  "VPN_SERVER_INDEX",
					Value: strconv.Itoa(*index),
				},
				{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
			}...)
	}

	if v.values.DisableNewVPN {
		envVariables = append(envVariables,
			corev1.EnvVar{
				Name:  "DO_NOT_CONFIGURE_KERNEL_SETTINGS",
				Value: "true",
			},
		)
	}

	return envVariables
}

func (v *vpnShoot) getResourceLimits() corev1.ResourceList {
	if v.values.VPAEnabled {
		return corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		}
	}
	return corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("120Mi"),
	}
}

func (v *vpnShoot) getVolumeMounts(secrets []vpnSecret) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{}
	for _, item := range secrets {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      item.volumeName,
			MountPath: item.mountPath,
		})
	}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameTLSAuth,
		MountPath: volumeMountPathSecretTLS,
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameDevNetTun,
		MountPath: volumeMountPathDevNetTun,
	})

	return volumeMounts
}

func (v *vpnShoot) getVolumes(secret []vpnSecret, secretCA, secretTLSAuth *corev1.Secret) []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, item := range secret {
		volumes = append(volumes, corev1.Volume{
			Name: item.volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0400),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretCA.Name,
								},
								Items: []corev1.KeyToPath{{
									Key:  secretsutils.DataKeyCertificateBundle,
									Path: "ca.crt",
								}},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: item.secret.Name,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  secretsutils.DataKeyCertificate,
										Path: secretsutils.DataKeyCertificate,
									},
									{
										Key:  secretsutils.DataKeyPrivateKey,
										Path: secretsutils.DataKeyPrivateKey,
									},
								},
							},
						},
					},
				},
			},
		})
	}
	volumes = append(volumes, corev1.Volume{
		Name: volumeNameTLSAuth,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secretTLSAuth.Name,
				DefaultMode: ptr.To[int32](0400),
			},
		},
	})
	hostPathCharDev := corev1.HostPathCharDev
	volumes = append(volumes, corev1.Volume{
		Name: volumeNameDevNetTun,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: volumeMountPathDevNetTun,
				Type: &hostPathCharDev,
			},
		},
	})
	return volumes
}

func (v *vpnShoot) getInitContainers() []corev1.Container {
	var ipFamilies []string
	for _, v := range v.values.ReversedVPN.IPFamilies {
		ipFamilies = append(ipFamilies, string(v))
	}

	container := corev1.Container{
		Name:            initContainerName,
		Image:           v.values.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/vpn-client", "setup"},
		Env: []corev1.EnvVar{
			{
				Name:  "IP_FAMILIES",
				Value: strings.Join(ipFamilies, ","),
			},
			{
				Name:  "IS_SHOOT_CLIENT",
				Value: "true",
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("30m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
		},
	}

	if v.values.HighAvailabilityEnabled {
		container.Env = append(container.Env, []corev1.EnvVar{
			{
				Name:  "IS_HA",
				Value: "true",
			},
			{
				Name:  "HA_VPN_SERVERS",
				Value: strconv.Itoa(v.values.HighAvailabilityNumberOfSeedServers),
			},
			{
				Name:  "HA_VPN_CLIENTS",
				Value: strconv.Itoa(v.values.HighAvailabilityNumberOfShootClients),
			},
		}...)
	}

	if v.values.DisableNewVPN {
		container.Command = nil
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS",
				Value: "true",
			})

		if v.values.HighAvailabilityEnabled {
			container.Env = append(container.Env,
				corev1.EnvVar{
					Name:  "CONFIGURE_BONDING",
					Value: "true",
				},
			)
		}
	}

	return []corev1.Container{container}
}
