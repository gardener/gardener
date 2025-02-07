// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-core-node-local-dns"

	labelKey = "k8s-app"
	// portServiceServer is the service port used for the DNS server.
	portServiceServer = 53
	// portServer is the target port used for the DNS server.
	portServer = 8053
	// prometheus configuration for node-local-dns
	prometheusPort      = 9253
	prometheusScrape    = true
	prometheusErrorPort = 9353

	containerName        = "node-cache"
	metricsPortName      = "metrics"
	errorMetricsPortName = "errormetrics"

	domain            = gardencorev1beta1.DefaultDomain
	serviceName       = "kube-dns-upstream"
	livenessProbePort = 8099
	configDataKey     = "Corefile"
)

// Interface contains functions for a NodeLocalDNS deployer.
type Interface interface {
	component.DeployWaiter
	SetClusterDNS([]string)
	SetDNSServers([]string)
	SetIPFamilies([]gardencorev1beta1.IPFamily)
}

// Values is a set of configuration values for the node-local-dns component.
type Values struct {
	// Image is the container image used for node-local-dns.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
	// Config is the node local configuration for the shoot spec
	Config *gardencorev1beta1.NodeLocalDNS
	// ClusterDNS are the ClusterIPs of kube-system/coredns Service
	ClusterDNS []string
	// DNSServer are the ClusterIPs of kube-system/coredns Service
	DNSServers []string
	// KubernetesVersion is the Kubernetes version of the Shoot.
	KubernetesVersion *semver.Version
	// IPFamilies specifies the IP protocol versions to use for node local dns.
	IPFamilies []gardencorev1beta1.IPFamily
}

// New creates a new instance of DeployWaiter for node-local-dns.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &nodeLocalDNS{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeLocalDNS struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nodeLocalDNS) Deploy(ctx context.Context) error {
	scrapeConfig := n.emptyScrapeConfig("")
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"node-local-dns",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:              monitoringv1alpha1.KubernetesRolePod,
				PodNamePrefix:     "node-local",
				ContainerName:     containerName,
				ContainerPortName: metricsPortName,
			},
			"coredns_build_info",
			"coredns_cache_entries",
			"coredns_cache_hits_total",
			"coredns_cache_misses_total",
			"coredns_dns_request_duration_seconds_count",
			"coredns_dns_request_duration_seconds_bucket",
			"coredns_dns_requests_total",
			"coredns_dns_responses_total",
			"coredns_forward_requests_total",
			"coredns_forward_responses_total",
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

	scrapeConfigErrors := n.emptyScrapeConfig("-errors")
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, scrapeConfigErrors, func() error {
		metav1.SetMetaDataLabel(&scrapeConfigErrors.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfigErrors.Spec = shoot.ClusterComponentScrapeConfigSpec(
			"node-local-dns-errors",
			shoot.KubernetesServiceDiscoveryConfig{
				Role:              monitoringv1alpha1.KubernetesRolePod,
				PodNamePrefix:     "node-local",
				ContainerName:     containerName,
				ContainerPortName: errorMetricsPortName,
			},
			"coredns_nodecache_setup_errors_total",
		)
		return nil
	}); err != nil {
		return err
	}

	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, n.client, n.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nodeLocalDNS) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, n.client,
		n.emptyScrapeConfig(""),
		n.emptyScrapeConfig("-errors"),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nodeLocalDNS) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *nodeLocalDNS) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *nodeLocalDNS) computeResourcesData() (map[string][]byte, error) {
	if n.getHealthAddress() == "" {
		return nil, errors.New("empty IPVSAddress")
	}

	var (
		registry       = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey: nodelocaldnsconstants.LabelValue,
				},
			},
			Data: map[string]string{
				configDataKey: domain + `:53 {
    errors
    cache {
            success 9984 30
            denial 9984 5
    }
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + n.getHealthAddress() + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + strings.Join(n.values.ClusterDNS, " ") + ` {
            ` + n.forceTcpToClusterDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
.:53 {
    errors
    cache 30
    reload
    loop
    bind ` + n.bindIP() + `
    forward . ` + n.upstreamDNSAddress() + ` {
            ` + n.forceTcpToUpstreamDNS() + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
			},
		}
	)

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					"k8s-app": "kube-dns-upstream",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"k8s-app": "kube-dns"},
				Ports: []corev1.ServicePort{
					{
						Name:       "dns",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolUDP,
					},
					{
						Name:       "dns-tcp",
						Port:       int32(portServiceServer),
						TargetPort: intstr.FromInt32(portServer),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}

		maxUnavailable       = intstr.FromString("10%")
		hostPathFileOrCreate = corev1.HostPathFileOrCreate
		daemonSet            = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
				Labels: map[string]string{
					labelKey:                                    nodelocaldnsconstants.LabelValue,
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleSystemComponent,
					managedresources.LabelKeyOrigin:             managedresources.LabelValueGardener,
					v1beta1constants.LabelNodeCriticalComponent: "true",
				},
			},
			Spec: appsv1.DaemonSetSpec{
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &maxUnavailable,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						labelKey: nodelocaldnsconstants.LabelValue,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							labelKey:                                    nodelocaldnsconstants.LabelValue,
							v1beta1constants.LabelNetworkPolicyToDNS:    "allowed",
							v1beta1constants.LabelNodeCriticalComponent: "true",
						},
						Annotations: map[string]string{
							"prometheus.io/port":   strconv.Itoa(prometheusPort),
							"prometheus.io/scrape": strconv.FormatBool(prometheusScrape),
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  "system-node-critical",
						ServiceAccountName: serviceAccount.Name,
						HostNetwork:        true,
						DNSPolicy:          corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Tolerations: []corev1.Toleration{
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							},
							{
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoSchedule,
							},
						},
						NodeSelector: map[string]string{
							v1beta1constants.LabelNodeLocalDNS: "true",
						},
						Containers: []corev1.Container{
							{
								Name:  containerName,
								Image: n.values.Image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("25m"),
										corev1.ResourceMemory: resource.MustParse("25Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("200Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN"},
									},
								},
								Args: []string{
									"-localip",
									n.containerArg(),
									"-conf",
									"/etc/Corefile",
									"-upstreamsvc",
									serviceName,
									"-health-port",
									strconv.Itoa(livenessProbePort),
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: int32(53),
										Name:          "dns",
										Protocol:      corev1.ProtocolUDP,
									},
									{
										ContainerPort: int32(53),
										Name:          "dns-tcp",
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusPort),
										Name:          metricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										ContainerPort: int32(prometheusErrorPort),
										Name:          errorMetricsPortName,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Host: n.getIPVSAddress(),
											Path: "/health",
											Port: intstr.FromInt32(livenessProbePort),
										},
									},
									InitialDelaySeconds: int32(60),
									TimeoutSeconds:      int32(5),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										MountPath: "/run/xtables.lock",
										Name:      "xtables-lock",
										ReadOnly:  false,
									},
									{
										MountPath: "/etc/coredns",
										Name:      "config-volume",
									},
									{
										MountPath: "/etc/kube-dns",
										Name:      "kube-dns-config",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "xtables-lock",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/run/xtables.lock",
										Type: &hostPathFileOrCreate,
									},
								},
							},
							{
								Name: "kube-dns-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-dns",
										},
										Optional: ptr.To(true),
									},
								},
							},
							{
								Name: "config-volume",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
										Items: []corev1.KeyToPath{
											{
												Key:  configDataKey,
												Path: "Corefile.base",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	utilruntime.Must(references.InjectAnnotations(daemonSet))

	if n.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-local-dns",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       daemonSet.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					}},
				},
			},
		}
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		configMap,
		service,
		daemonSet,
		vpa,
	)
}

func selectIPAddress(addresses []string, preferIPv6 bool) string {
	if len(addresses) == 1 {
		return addresses[0]
	}
	var ipv4, ipv6 string
	for _, addr := range addresses {
		ip := net.ParseIP(addr)
		if ip.To4() != nil {
			ipv4 = addr
		} else {
			ipv6 = addr
		}
	}
	if preferIPv6 {
		return ipv6
	}
	return ipv4
}

func (n *nodeLocalDNS) bindIP() string {
	if len(n.values.DNSServers) > 0 {
		dnsAddress := selectIPAddress(n.values.DNSServers, n.values.IPFamilies[0] != gardencorev1beta1.IPFamilyIPv4)
		return n.getIPVSAddress() + " " + dnsAddress
	}
	return n.getIPVSAddress()
}

func (n *nodeLocalDNS) containerArg() string {
	if len(n.values.DNSServers) > 0 {
		dnsAddress := selectIPAddress(n.values.DNSServers, n.values.IPFamilies[0] != gardencorev1beta1.IPFamilyIPv4)
		return n.getIPVSAddress() + "," + dnsAddress
	}
	return n.getIPVSAddress()
}

func (n *nodeLocalDNS) forceTcpToClusterDNS() string {
	if n.values.Config == nil || n.values.Config.ForceTCPToClusterDNS == nil || *n.values.Config.ForceTCPToClusterDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}

func (n *nodeLocalDNS) forceTcpToUpstreamDNS() string {
	if n.values.Config == nil || n.values.Config.ForceTCPToUpstreamDNS == nil || *n.values.Config.ForceTCPToUpstreamDNS {
		return "force_tcp"
	}
	return "prefer_udp"
}

func (n *nodeLocalDNS) upstreamDNSAddress() string {
	if n.values.Config != nil && ptr.Deref(n.values.Config.DisableForwardToUpstreamDNS, false) {
		return strings.Join(n.values.ClusterDNS, " ")
	}
	return "__PILLAR__UPSTREAM__SERVERS__"
}

func (n *nodeLocalDNS) emptyScrapeConfig(suffix string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta("node-local-dns"+suffix, n.namespace, shoot.Label)}
}

func (n *nodeLocalDNS) SetClusterDNS(dns []string) {
	n.values.ClusterDNS = dns
}

func (n *nodeLocalDNS) SetDNSServers(servers []string) {
	n.values.DNSServers = servers
}

func (n *nodeLocalDNS) SetIPFamilies(ipfamilies []gardencorev1beta1.IPFamily) {
	n.values.IPFamilies = ipfamilies
}

func (n *nodeLocalDNS) getIPVSAddress() (ipvsAddress string) {
	return n.getAddress(false)
}

func (n *nodeLocalDNS) getHealthAddress() (healthAddress string) {
	return n.getAddress(true)
}

func (n *nodeLocalDNS) getAddress(useIPv6Brackets bool) string {
	if n.values.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv4 {
		return nodelocaldnsconstants.IPVSAddress
	}
	if useIPv6Brackets {
		return fmt.Sprintf("[%s]", nodelocaldnsconstants.IPVSIPv6Address)
	}
	return nodelocaldnsconstants.IPVSIPv6Address
}
