// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"context"
	"net"
	"regexp"
	"strconv"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// DeploymentName is the name of the coredns Deployment.
	DeploymentName = "coredns"

	clusterProportionalAutoscalerDeploymentName = "coredns-autoscaler"
	clusterProportionalDNSAutoscalerLabelValue  = "coredns-autoscaler"
	managedResourceName                         = "shoot-core-coredns"

	containerName = "coredns"
	serviceName   = "kube-dns" // this is due to legacy reasons

	portNameMetrics = "metrics"
	portMetrics     = 9153

	configDataKey               = "Corefile"
	volumeNameConfig            = "config-volume"
	volumeNameConfigCustom      = "custom-config-volume"
	volumeMountPathConfig       = "/etc/coredns"
	volumeMountPathConfigCustom = "/etc/coredns/custom"
)

// Interface contains functions for a CoreDNS deployer.
type Interface interface {
	component.DeployWaiter
	SetPodAnnotations(map[string]string)
	SetNodeNetworkCIDRs([]net.IPNet)
	SetPodNetworkCIDRs([]net.IPNet)
	SetClusterIPs([]net.IP)
	SetIPFamilies([]gardencorev1beta1.IPFamily)
}

// Values is a set of configuration values for the coredns component.
type Values struct {
	// APIServerHost is the host of the kube-apiserver.
	APIServerHost *string
	// ClusterDomain is the domain used for cluster-wide DNS records handled by CoreDNS.
	ClusterDomain string
	// ClusterIPs is the IP address which should be used as `.spec.clusterIP` in the Service spec.
	ClusterIPs []net.IP
	// Image is the container image used for CoreDNS.
	Image string
	// PodAnnotations is the set of additional annotations to be used for the pods.
	PodAnnotations map[string]string
	// PodNetworkCIDRs are the CIDR of the pod network.
	PodNetworkCIDRs []net.IPNet
	// NodeNetworkCIDRs are the CIDR of the node network.
	NodeNetworkCIDRs []net.IPNet
	// AutoscalingMode indicates whether cluster proportional autoscaling is enabled.
	AutoscalingMode gardencorev1beta1.CoreDNSAutoscalingMode
	// ClusterProportionalAutoscalerImage is the container image used for the cluster proportional autoscaler.
	ClusterProportionalAutoscalerImage string
	// WantsVerticalPodAutoscaler indicates whether vertical autoscaler should be used.
	WantsVerticalPodAutoscaler bool
	// SearchPathRewriteCommonSuffixes contains common suffixes to be rewritten when SearchPathRewritesEnabled is set.
	SearchPathRewriteCommonSuffixes []string
	// IPFamilies specifies the IP protocol versions to use for core dns.
	IPFamilies []gardencorev1beta1.IPFamily
}

// New creates a new instance of DeployWaiter for coredns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &coreDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type coreDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (c *coreDNS) Deploy(ctx context.Context) error {
	scrapeConfig := c.emptyScrapeConfig()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"coredns",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:             monitoringv1alpha1.KubernetesRoleEndpoint,
				ServiceName:      serviceName,
				EndpointPortName: portNameMetrics,
			},
			"coredns_build_info",
			"coredns_cache_entries",
			"coredns_cache_hits_total",
			"coredns_cache_misses_total",
			"coredns_dns_request_duration_seconds_count",
			"coredns_dns_request_duration_seconds_bucket",
			"coredns_dns_requests_total",
			"coredns_dns_responses_total",
			"coredns_proxy_request_duration_seconds_count",
			"coredns_kubernetes_dns_programming_duration_seconds_bucket",
			"coredns_kubernetes_dns_programming_duration_seconds_count",
			"coredns_kubernetes_dns_programming_duration_seconds_sum",
			"process_max_fds",
			"process_open_fds",
		)
		return nil
	}); err != nil {
		return err
	}

	prometheusRule := c.emptyPrometheusRule()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "coredns.rules",
				Rules: []monitoringv1.Rule{
					{
						Alert: "CoreDNSDown",
						Expr:  intstr.FromString(`absent(up{job="coredns"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("20m")),
						Labels: map[string]string{
							"service":    serviceName,
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "all",
						},
						Annotations: map[string]string{
							"description": "CoreDNS could not be found. Cluster DNS resolution will not work.",
							"summary":     "CoreDNS is down",
						},
					},
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := c.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (c *coreDNS) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, c.client,
		c.emptyScrapeConfig(),
		c.emptyPrometheusRule(),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, c.client, c.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *coreDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, managedResourceName)
}

func (c *coreDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, managedResourceName)
}

func (c *coreDNS) computeResourcesData() (map[string][]byte, error) {
	var (
		portAPIServer       = intstr.FromInt32(kubeapiserverconstants.Port)
		portDNSServerHost   = intstr.FromInt32(53)
		portDNSServer       = intstr.FromInt32(corednsconstants.PortServer)
		portMetricsEndpoint = intstr.FromInt32(portMetrics)
		protocolTCP         = corev1.ProtocolTCP
		protocolUDP         = corev1.ProtocolUDP

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:coredns",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints", "services", "pods", "namespaces"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"discovery.k8s.io"},
					Resources: []string{"endpointslices"},
					Verbs:     []string{"list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "system:coredns",
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}

		// We don't need to make this ConfigMap immutable since CoreDNS provides the "reload" plugins which does an
		// auto-reload if the config changes.
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				configDataKey: `.:` + strconv.Itoa(corednsconstants.PortServer) + ` {  
  health {
      lameduck 15s
  }
  ready` + getSearchPathRewrites(c.values.ClusterDomain, c.values.SearchPathRewriteCommonSuffixes) + `
  kubernetes ` + c.values.ClusterDomain + ` in-addr.arpa ip6.arpa {
      pods insecure
      fallthrough in-addr.arpa ip6.arpa
      ttl 30
  }
  prometheus :` + strconv.Itoa(portMetrics) + `

  import custom/*.override

  errors
  log . {
      class error
  }
  forward . /etc/resolv.conf
  loop
  reload
  loadbalance round_robin
  cache 30
}
import custom/*.server
`,
			},
		}

		configMapCustom = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "coredns-custom",
				Namespace:   metav1.NamespaceSystem,
				Annotations: map[string]string{resourcesv1alpha1.Ignore: "true"},
			},
			Data: map[string]string{
				"changeme.server":   "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/networking/custom-dns-config.md",
				"changeme.override": "# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/networking/custom-dns-config.md",
			},
		}

		ipFamilyPolicy = getIPFamilyPolicy(c.values.IPFamilies)

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					corednsconstants.LabelKey:       corednsconstants.LabelValue,
					"kubernetes.io/cluster-service": "true",
					"kubernetes.io/name":            "CoreDNS",
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: c.values.ClusterIPs[0].String(),
				Selector:  map[string]string{corednsconstants.LabelKey: corednsconstants.LabelValue},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(corednsconstants.PortServiceServer),
						TargetPort: intstr.FromInt32(corednsconstants.PortServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(corednsconstants.PortServiceServer),
						TargetPort: intstr.FromInt32(corednsconstants.PortServer),
						Protocol:   corev1.ProtocolTCP,
					},
					{
						Name:       "metrics",
						Port:       int32(portMetrics),
						TargetPort: intstr.FromInt32(portMetrics),
					},
				},
				IPFamilyPolicy: &ipFamilyPolicy,
			},
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud--allow-dns",
				Namespace: metav1.NamespaceSystem,
				Annotations: map[string]string{
					v1beta1constants.GardenerDescription: "Allows CoreDNS to lookup DNS records, talk to the API Server. " +
						"Also allows CoreDNS to be reachable via its service and its metrics endpoint.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      corednsconstants.LabelKey,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{corednsconstants.LabelValue},
					}},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &portAPIServer, Protocol: &protocolTCP},     // Allow communication to API Server
						{Port: &portDNSServerHost, Protocol: &protocolTCP}, // Lookup DNS due to cache miss
						{Port: &portDNSServerHost, Protocol: &protocolUDP}, // Lookup DNS due to cache miss
					},
				}},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &portMetricsEndpoint, Protocol: &protocolTCP}, // CoreDNS metrics port
						{Port: &portDNSServer, Protocol: &protocolTCP},       // CoreDNS server port
						{Port: &portDNSServer, Protocol: &protocolUDP},       // CoreDNS server port
					},
					From: []networkingv1.NetworkPolicyPeer{
						{NamespaceSelector: &metav1.LabelSelector{}, PodSelector: &metav1.LabelSelector{}},
					},
				}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DeploymentName,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](2),
				RevisionHistoryLimit: ptr.To[int32](2),
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       ptr.To(intstr.FromInt32(1)),
						MaxUnavailable: ptr.To(intstr.FromInt32(0)),
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corednsconstants.LabelKey: corednsconstants.LabelValue},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: c.values.PodAnnotations,
						Labels:      getLabels(),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-cluster-critical",
						ServiceAccountName: serviceAccount.Name,
						DNSPolicy:          corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot:       ptr.To(true),
							RunAsUser:          ptr.To[int64](65534),
							FSGroup:            ptr.To[int64](1),
							SupplementalGroups: []int64{1},
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           c.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{"" +
								"-conf",
								volumeMountPathConfig + "/" + configDataKey,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "dns-udp",
									Protocol:      protocolUDP,
									ContainerPort: corednsconstants.PortServer,
								},
								{
									Name:          "dns-tcp",
									Protocol:      protocolTCP,
									ContainerPort: corednsconstants.PortServer,
								},
								{
									Name:          portNameMetrics,
									Protocol:      protocolTCP,
									ContainerPort: portMetrics,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Scheme: corev1.URISchemeHTTP,
										Port:   intstr.FromInt32(8080),
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    5,
								InitialDelaySeconds: 60,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/ready",
										Scheme: corev1.URISchemeHTTP,
										Port:   intstr.FromInt32(8181),
									},
								},
								SuccessThreshold:    1,
								FailureThreshold:    1,
								InitialDelaySeconds: 30,
								TimeoutSeconds:      2,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("15Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1500Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"NET_BIND_SERVICE"}, // TODO(marc1404): When updating coredns to v1.13.x check if the NET_BIND_SERVICE capability can be removed.
									Drop: []corev1.Capability{"all"},
								},
								ReadOnlyRootFilesystem: ptr.To(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      volumeNameConfig,
									MountPath: volumeMountPathConfig,
									ReadOnly:  true,
								},
								{
									Name:      volumeNameConfigCustom,
									MountPath: volumeMountPathConfigCustom,
									ReadOnly:  true,
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
										Items: []corev1.KeyToPath{{
											Key:  configDataKey,
											Path: configDataKey,
										}},
									},
								},
							},
							{
								Name: volumeNameConfigCustom,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapCustom.Name,
										},
										DefaultMode: ptr.To[int32](420),
										Optional:    ptr.To(true),
									},
								},
							},
						},
					},
				},
			},
		}

		clusterProportionalDNSAutoscalerServiceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns-autoscaler",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterProportionalDNSAutoscalerClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:coredns-autoscaler",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"replicationcontrollers/scale"},
					Verbs:     []string{"get", "update"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"deployments/scale", "replicasets/scale"},
					Verbs:     []string{"get", "update"},
				},
				// Remove the configmaps rule once below issue is fixed:
				// kubernetes-incubator/cluster-proportional-autoscaler#16
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"get", "create"},
				},
			},
		}

		clusterProportionalDNSAutoscalerClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:coredns-autoscaler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterProportionalDNSAutoscalerClusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      clusterProportionalDNSAutoscalerServiceAccount.Name,
				Namespace: clusterProportionalDNSAutoscalerServiceAccount.Namespace,
			}},
		}

		clusterProportionalDNSAutoscalerDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterProportionalAutoscalerDeploymentName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getClusterProportionalDNSAutoscalerLabels(),
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{corednsconstants.LabelKey: clusterProportionalDNSAutoscalerLabelValue},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: getClusterProportionalDNSAutoscalerLabels(),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-cluster-critical",
						ServiceAccountName: clusterProportionalDNSAutoscalerServiceAccount.Name,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot:       ptr.To(true),
							RunAsUser:          ptr.To[int64](65534),
							SupplementalGroups: []int64{65534},
							FSGroup:            ptr.To[int64](65534),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{{
							Name:            "autoscaler",
							Image:           c.values.ClusterProportionalAutoscalerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/cluster-proportional-autoscaler",
								"--namespace=" + metav1.NamespaceSystem,
								"--configmap=coredns-autoscaler",
								"--target=deployment/" + deployment.Name,
								`--default-params={"linear":{"coresPerReplica":256,"nodesPerReplica":16,"min":2,"preventSinglePointFailure":true,"includeUnschedulableNodes":true}}`,
								"--logtostderr=true",
								"--v=2",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("70Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
								},
								ReadOnlyRootFilesystem: ptr.To(true),
							},
						}},
					},
				},
			},
		}

		clusterProportionalDNSAutoscalerVPA = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: clusterProportionalAutoscalerDeploymentName, Namespace: metav1.NamespaceSystem},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       clusterProportionalAutoscalerDeploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
				Labels:    map[string]string{corednsconstants.LabelKey: corednsconstants.LabelValue},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
				Selector:                   deployment.Spec.Selector,
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		horizontalPodAutoscaler = &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
				},
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: ptr.To[int32](2),
				MaxReplicas: 5,
				Metrics: []autoscalingv2.MetricSpec{{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To[int32](70),
						},
					},
				}},
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
			},
		}
	)

	managedObjects := []client.Object{
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		configMap,
		configMapCustom,
		service,
		networkPolicy,
		deployment,
		podDisruptionBudget,
	}

	if c.values.APIServerHost != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: *c.values.APIServerHost,
		})
	}

	for _, cidr := range append(c.values.NodeNetworkCIDRs, c.values.PodNetworkCIDRs...) {
		networkPolicy.Spec.Ingress[0].From = append(networkPolicy.Spec.Ingress[0].From, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{CIDR: cidr.String()},
		})
	}

	for _, ip := range c.values.ClusterIPs {
		service.Spec.ClusterIPs = append(service.Spec.ClusterIPs, ip.String())
	}

	if c.values.AutoscalingMode == gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional {
		managedObjects = append(managedObjects,
			clusterProportionalDNSAutoscalerServiceAccount,
			clusterProportionalDNSAutoscalerClusterRole,
			clusterProportionalDNSAutoscalerClusterRoleBinding,
			clusterProportionalDNSAutoscalerDeployment,
		)
		if c.values.WantsVerticalPodAutoscaler {
			managedObjects = append(managedObjects, clusterProportionalDNSAutoscalerVPA)
		}
		// Replicas are managed by the cluster-proportional autoscaler and not the high-availability webhook
		delete(deployment.Labels, resourcesv1alpha1.HighAvailabilityConfigType)
		deployment.Spec.Replicas = nil
	} else {
		managedObjects = append(managedObjects, horizontalPodAutoscaler)
	}

	return registry.AddAllAndSerialize(managedObjects...)
}

func (c *coreDNS) SetPodAnnotations(v map[string]string) {
	c.values.PodAnnotations = v
}

func (c *coreDNS) SetNodeNetworkCIDRs(nodes []net.IPNet) {
	c.values.NodeNetworkCIDRs = nodes
}

func (c *coreDNS) SetPodNetworkCIDRs(pods []net.IPNet) {
	c.values.PodNetworkCIDRs = pods
}

func (c *coreDNS) SetClusterIPs(ips []net.IP) {
	c.values.ClusterIPs = ips
}

func (c *coreDNS) SetIPFamilies(ipfamilies []gardencorev1beta1.IPFamily) {
	c.values.IPFamilies = ipfamilies
}

func (c *coreDNS) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("coredns", c.namespace, shoot.Label)}
}

func (c *coreDNS) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta("coredns", c.namespace, shoot.Label)}
}

func getLabels() map[string]string {
	return map[string]string{
		"origin":                    "gardener",
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent,
		corednsconstants.LabelKey:   corednsconstants.LabelValue,
	}
}

func getClusterProportionalDNSAutoscalerLabels() map[string]string {
	return map[string]string{
		"origin":                        "gardener",
		v1beta1constants.GardenRole:     v1beta1constants.GardenRoleSystemComponent,
		corednsconstants.LabelKey:       clusterProportionalDNSAutoscalerLabelValue,
		"kubernetes.io/cluster-service": "true",
	}
}

func getSearchPathRewrites(clusterDomain string, commonSuffixes []string) string {
	quotedClusterDomain := regexp.QuoteMeta(clusterDomain)
	suffixRewrites := ""
	for _, suffix := range commonSuffixes {
		suffixRewrites = suffixRewrites + `
  rewrite stop {
    name regex (.*)\.` + regexp.QuoteMeta(suffix) + `\.svc\.` + quotedClusterDomain + ` {1}.` + suffix + `
    answer name (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.` + clusterDomain + `
    answer value (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.` + clusterDomain + `
  }`
	}
	return `
  rewrite stop {
    name regex (^(?:[^\.]+\.)+)svc\.` + quotedClusterDomain + `\.svc\.` + quotedClusterDomain + ` {1}svc.` + clusterDomain + `
    answer name (^(?:[^\.]+\.)+)svc\.` + quotedClusterDomain + ` {1}svc.` + clusterDomain + `.svc.` + clusterDomain + `
    answer value (^(?:[^\.]+\.)+)svc\.` + quotedClusterDomain + ` {1}svc.` + clusterDomain + `.svc.` + clusterDomain + `
  }` + suffixRewrites
}

func getIPFamilyPolicy(ipFamilies []gardencorev1beta1.IPFamily) corev1.IPFamilyPolicy {
	ipFamiliesSet := sets.New[gardencorev1beta1.IPFamily](ipFamilies...)
	if ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv4) && ipFamiliesSet.Has(gardencorev1beta1.IPFamilyIPv6) {
		return corev1.IPFamilyPolicyPreferDualStack
	}
	return corev1.IPFamilyPolicySingleStack
}
