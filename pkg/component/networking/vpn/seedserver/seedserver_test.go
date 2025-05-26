// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedserver_test

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"strings"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	comptest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	tplNameEnvoy = "envoy.yaml.tpl"
	//go:embed templates/envoy.yaml.tpl
	tplContentEnvoy string
	tplEnvoy        *template.Template
)

func init() {
	var err error
	tplEnvoy, err = template.
		New(tplNameEnvoy).
		Parse(tplContentEnvoy)
	utilruntime.Must(err)
}

var _ = Describe("VpnSeedServer", func() {
	var (
		c             client.Client
		sm            secretsmanager.Interface
		vpnSeedServer Interface

		ctx                 = context.TODO()
		namespace           = "shoot--foo--bar"
		vpnSeedServerImage  = "some-image:some-tag"
		apiServerProxyImage = "some-image2:some-tag2"
		values              = Values{}

		istioNamespace     = "istio-foo"
		istioNamespaceFunc = func() string { return istioNamespace }

		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto

		secretNameTLSAuth = "vpn-seed-server-tlsauth-a1d0aa00"

		listenAddress        = "0.0.0.0"
		listenAddressV6      = "::"
		dnsLookUpFamily      = "ALL"
		volumeMountPathCerts = "/srv/secrets/vpn-server"
		fileNameCABundle     = "ca.crt"
		metricsPort          = 15000

		expectedConfigMap *corev1.ConfigMap
	)

	var (
		template = func(nodeNetworks []net.IPNet, highAvailability, disableNewVPN bool) *corev1.PodTemplateSpec {
			hostPathCharDev := corev1.HostPathCharDev
			nodes := ""
			if len(nodeNetworks) > 0 {
				nodes = nodeNetworks[0].String()
			}
			template := &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1beta1constants.GardenRole:                                     v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:                                       "vpn-seed-server",
						v1beta1constants.LabelNetworkPolicyToDNS:                        v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks:            v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: ptr.To(false),
					PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
					DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
					Containers: []corev1.Container{
						{
							Name:            "vpn-seed-server",
							Image:           vpnSeedServerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "tcp-tunnel",
									ContainerPort: 1194,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "IP_FAMILIES",
									Value: string(values.Network.IPFamilies[0]),
								},
								{
									Name:  "SERVICE_NETWORK",
									Value: values.Network.ServiceCIDRs[0].String(),
								},
								{
									Name:  "POD_NETWORK",
									Value: values.Network.PodCIDRs[0].String(),
								},
								{
									Name:  "NODE_NETWORK",
									Value: nodes,
								},
								{
									Name:  "SERVICE_NETWORKS",
									Value: values.Network.ServiceCIDRs[0].String(),
								},
								{
									Name:  "POD_NETWORKS",
									Value: values.Network.PodCIDRs[0].String(),
								},
								{
									Name:  "NODE_NETWORKS",
									Value: nodes,
								},
								{
									Name:  "SEED_POD_NETWORK",
									Value: values.SeedPodNetwork,
								},
								{
									Name: "LOCAL_NODE_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.hostIP",
										},
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(1194),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(1194),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("7.5M"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
										"NET_RAW",
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dev-net-tun",
									MountPath: "/dev/net/tun",
								},
								{
									Name:      "certs",
									MountPath: "/srv/secrets/vpn-server",
								},
								{
									Name:      "tlsauth",
									MountPath: "/srv/secrets/tlsauth",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: ptr.To[int64](30),
					Volumes: []corev1.Volume{
						{
							Name: "dev-net-tun",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/dev/net/tun",
									Type: &hostPathCharDev,
								},
							},
						},
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: ptr.To[int32](420),
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "ca-vpn",
												},
												Items: []corev1.KeyToPath{{
													Key:  "bundle.crt",
													Path: "ca.crt",
												}},
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "vpn-seed-server",
												},
												Items: []corev1.KeyToPath{
													{
														Key:  "tls.crt",
														Path: "tls.crt",
													},
													{
														Key:  "tls.key",
														Path: "tls.key",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: "tlsauth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameTLSAuth,
									DefaultMode: ptr.To[int32](0400),
								},
							},
						},
					},
				},
			}
			if highAvailability {
				mount := corev1.VolumeMount{
					Name:      "openvpn-status",
					MountPath: "/srv/status",
				}
				template.Spec.Containers[0].Env = append(template.Spec.Containers[0].Env, []corev1.EnvVar{
					{
						Name:  "OPENVPN_STATUS_PATH",
						Value: "/srv/status/openvpn.status",
					},
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name:  "IS_HA",
						Value: "true",
					},
					{
						Name:  "HA_VPN_CLIENTS",
						Value: "2",
					},
				}...)
				template.Spec.Containers[0].VolumeMounts = append(template.Spec.Containers[0].VolumeMounts, mount)
				exporterContainer := corev1.Container{
					Name:            "openvpn-exporter",
					Image:           vpnSeedServerImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"/bin/vpn-server",
						"exporter",
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OPENVPN_STATUS_PATH",
							Value: "/srv/status/openvpn.status",
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 15000,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"all",
							},
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(15000),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(15000),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("20M"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{mount},
				}
				if disableNewVPN {
					exporterContainer.Command = []string{
						"/openvpn-exporter",
						"-openvpn.status_paths",
						"/srv/status/openvpn.status",
						"-web.listen-address",
						":15000",
					}
					exporterContainer.Env = nil
				}
				template.Spec.Containers = append(template.Spec.Containers, exporterContainer)
				template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
					Name: "openvpn-status",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
			} else {
				template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
					Name:            "envoy-proxy",
					Image:           apiServerProxyImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"envoy",
						"--concurrency",
						"2",
						"-c",
						"/etc/envoy/envoy.yaml",
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(9443),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(9443),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("25M"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"all",
							},
						},
						RunAsUser:    ptr.To(int64(v1beta1constants.EnvoyNonRootUserId)),
						RunAsNonRoot: ptr.To(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "certs",
							MountPath: "/srv/secrets/vpn-server",
						},
						{
							Name:      "envoy-config",
							MountPath: "/etc/envoy",
						},
					},
				})
				template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
					Name: "envoy-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: expectedConfigMap.Name,
							},
						},
					},
				})
			}

			if !disableNewVPN {
				template.Spec.InitContainers = []corev1.Container{
					{
						Name:            "setup",
						Image:           vpnSeedServerImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command: []string{
							"/bin/vpn-server",
							"setup",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: ptr.To(true),
						},
					},
				}
			}

			return template
		}

		deployment = func(nodeNetworks []net.IPNet) *appsv1.Deployment {
			maxSurge := intstr.FromInt32(100)
			maxUnavailable := intstr.FromInt32(0)
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-seed-server",
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   "vpn-seed-server",
						"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To(values.Replicas),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
						v1beta1constants.LabelApp: "vpn-seed-server",
					}},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxUnavailable: &maxUnavailable,
							MaxSurge:       &maxSurge,
						},
						Type: appsv1.RollingUpdateDeploymentStrategyType,
					},
					Template: *template(nodeNetworks, false, false),
				},
			}

			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}

		statefulSet = func(nodeNetworks []net.IPNet, disableNewVPN bool) *appsv1.StatefulSet {
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-seed-server",
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   "vpn-seed-server",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy:  appsv1.ParallelPodManagement,
					Replicas:             ptr.To[int32](3),
					RevisionHistoryLimit: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
						v1beta1constants.LabelApp: "vpn-seed-server",
					}},
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
					},
					Template: *template(nodeNetworks, true, disableNewVPN),
				},
			}

			Expect(references.InjectAnnotations(sts)).To(Succeed())
			return sts
		}

		destinationRule = func() *istionetworkingv1beta1.DestinationRule {
			return &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-server", Namespace: namespace, ResourceVersion: "1"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					ExportTo: []string{"*"},
					Host:     fmt.Sprintf("%s.%s.svc.cluster.local", "vpn-seed-server", namespace),
					TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
						ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
							Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
								MaxConnections: 5000,
								TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
									Interval: &durationpb.Duration{
										Seconds: 75,
									},
									Time: &durationpb.Duration{
										Seconds: 7200,
									},
								},
							},
						},
						LoadBalancer: &istioapinetworkingv1beta1.LoadBalancerSettings{
							LocalityLbSetting: &istioapinetworkingv1beta1.LocalityLoadBalancerSetting{
								Enabled:          &wrapperspb.BoolValue{Value: true},
								FailoverPriority: []string{"topology.kubernetes.io/zone"},
							},
						},
						OutlierDetection: &istioapinetworkingv1beta1.OutlierDetection{
							MinHealthPercent: 0,
						},
						Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
							Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
						},
					},
				},
			}
		}

		indexedDestinationRule = func(idx int) *istionetworkingv1beta1.DestinationRule {
			destRule := destinationRule()
			destRule.Name = fmt.Sprintf("%s-%d", "vpn-seed-server", idx)
			destRule.Spec.Host = fmt.Sprintf("%s.%s.svc.cluster.local", destRule.Name, namespace)
			return destRule
		}

		maxUnavailable              = intstr.FromInt32(1)
		expectedPodDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "vpn-seed-server",
				},
				ResourceVersion: "1",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpn-seed-server",
					},
				},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		expectedService = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceName,
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.istio.io/exportTo":                                              "*",
					"networking.resources.gardener.cloud/namespace-selectors":                   `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]}]`,
					"networking.resources.gardener.cloud/pod-label-selector-namespace-alias":    "all-shoots",
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":15000}]`,
				},
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "vpn-seed-server",
						Port:       1194,
						TargetPort: intstr.FromInt32(1194),
					},
					{
						Name:       "http-proxy",
						Port:       9443,
						TargetPort: intstr.FromInt32(9443),
					},
					{
						Name:       "metrics",
						Port:       15000,
						TargetPort: intstr.FromInt32(15000),
					},
				},
				Selector: map[string]string{
					v1beta1constants.LabelApp: "vpn-seed-server",
				},
			},
		}

		scrapeConfig = func(highAvailabilityEnabled bool) *monitoringv1alpha1.ScrapeConfig {
			jobName, serviceNameRegexSuffix := "reversed-vpn-envoy-side-car", ""
			allowedMetrics := []string{
				"envoy_cluster_external_upstream_rq",
				"envoy_cluster_external_upstream_rq_completed",
				"envoy_cluster_external_upstream_rq_xx",
				"envoy_cluster_lb_healthy_panic",
				"envoy_cluster_original_dst_host_invalid",
				"envoy_cluster_upstream_cx_active",
				"envoy_cluster_upstream_cx_connect_attempts_exceeded",
				"envoy_cluster_upstream_cx_connect_fail",
				"envoy_cluster_upstream_cx_connect_timeout",
				"envoy_cluster_upstream_cx_max_requests",
				"envoy_cluster_upstream_cx_none_healthy",
				"envoy_cluster_upstream_cx_overflow",
				"envoy_cluster_upstream_cx_pool_overflow",
				"envoy_cluster_upstream_cx_protocol_error",
				"envoy_cluster_upstream_cx_rx_bytes_total",
				"envoy_cluster_upstream_cx_total",
				"envoy_cluster_upstream_cx_tx_bytes_total",
				"envoy_cluster_upstream_rq",
				"envoy_cluster_upstream_rq_completed",
				"envoy_cluster_upstream_rq_max_duration_reached",
				"envoy_cluster_upstream_rq_pending_overflow",
				"envoy_cluster_upstream_rq_per_try_timeout",
				"envoy_cluster_upstream_rq_retry",
				"envoy_cluster_upstream_rq_retry_limit_exceeded",
				"envoy_cluster_upstream_rq_retry_overflow",
				"envoy_cluster_upstream_rq_rx_reset",
				"envoy_cluster_upstream_rq_timeout",
				"envoy_cluster_upstream_rq_total",
				"envoy_cluster_upstream_rq_tx_reset",
				"envoy_cluster_upstream_rq_xx",
				"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_attempt",
				"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_failure",
				"envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_success",
				"envoy_dns_cache_dynamic_forward_proxy_cache_config_host_overflow",
				"envoy_dns_cache_dynamic_forward_proxy_cache_config_num_hosts",
				"envoy_http_downstream_cx_rx_bytes_total",
				"envoy_http_downstream_cx_total",
				"envoy_http_downstream_cx_tx_bytes_total",
				"envoy_http_downstream_rq_xx",
				"envoy_http_no_route",
				"envoy_http_rq_total",
				"envoy_listener_http_downstream_rq_xx",
				"envoy_server_memory_allocated",
				"envoy_server_memory_heap_size",
				"envoy_server_memory_physical_size",
				"envoy_cluster_upstream_cx_connect_ms_bucket",
				"envoy_cluster_upstream_cx_connect_ms_sum",
				"envoy_cluster_upstream_cx_length_ms_bucket",
				"envoy_cluster_upstream_cx_length_ms_sum",
				"envoy_http_downstream_cx_length_ms_bucket",
				"envoy_http_downstream_cx_length_ms_sum",
			}

			if highAvailabilityEnabled {
				jobName, serviceNameRegexSuffix = "openvpn-server-exporter", "-[0-2]"
				allowedMetrics = []string{
					"openvpn_server_client_received_bytes_total",
					"openvpn_server_client_sent_bytes_total",
					"openvpn_server_route_last_reference_time_seconds",
					"openvpn_status_update_time_seconds",
					"openvpn_up",
				}
			}

			scrapeConfig := &monitoringv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "shoot-vpn-seed-server",
					Namespace:       namespace,
					Labels:          map[string]string{"prometheus": "shoot"},
					ResourceVersion: "1",
				},
				Spec: monitoringv1alpha1.ScrapeConfigSpec{
					KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
						Role:       "Service",
						Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
					}},
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To(jobName),
							TargetLabel: "job",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_service_port_name"},
							Action:       "keep",
							Regex:        "vpn-seed-server" + serviceNameRegexSuffix + `;metrics`,
						},
					},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
				},
			}

			if highAvailabilityEnabled {
				scrapeConfig.Spec.MetricRelabelConfigs = append(scrapeConfig.Spec.MetricRelabelConfigs,
					monitoringv1.RelabelConfig{
						SourceLabels: []monitoringv1.LabelName{"instance"},
						Action:       "replace",
						Regex:        `([^.]+).+`,
						TargetLabel:  "service",
					},
					monitoringv1.RelabelConfig{
						SourceLabels: []monitoringv1.LabelName{"real_address"},
						Action:       "replace",
						Regex:        `([^:]+).+`,
						TargetLabel:  "real_ip",
					},
					monitoringv1.RelabelConfig{
						Regex:  "username",
						Action: "labeldrop",
					},
				)
			}

			return scrapeConfig
		}

		indexedService = func(idx int) *corev1.Service {
			svc := expectedService.DeepCopy()
			svc.Name = fmt.Sprintf("%s-%d", ServiceName, idx)
			svc.Spec.Selector = map[string]string{
				"statefulset.kubernetes.io/pod-name": svc.Name,
			}
			return svc
		}

		expectedVPAFor = func(highAvailabilityEnabled bool, updateMode *vpaautoscalingv1.UpdateMode) *vpaautoscalingv1.VerticalPodAutoscaler {
			targetKindRef := "Deployment"
			if highAvailabilityEnabled {
				targetKindRef = "StatefulSet"
			}

			containerPolicies := []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: "vpn-seed-server",
					Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
				},
			}

			if highAvailabilityEnabled {
				containerPolicies = append(containerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
					ContainerName: "openvpn-exporter",
					Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
				})
			} else {
				containerPolicies = append(containerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
					ContainerName:    "envoy-proxy",
					ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
				})
			}

			return &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "vpn-seed-server" + "-vpa",
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       targetKindRef,
						Name:       "vpn-seed-server",
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: updateMode,
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: containerPolicies,
					},
				},
			}
		}
	)

	BeforeEach(func() {
		values = Values{
			ImageAPIServerProxy: apiServerProxyImage,
			ImageVPNSeedServer:  vpnSeedServerImage,
			KubeAPIServerHost:   ptr.To("foo.bar"),
			Network: NetworkValues{
				PodCIDRs:     []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}},
				ServiceCIDRs: []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}},
				NodeCIDRs:    []net.IPNet{{IP: net.ParseIP("10.0.2.0"), Mask: net.CIDRMask(24, 32)}},
				IPFamilies:   []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
			},
			Replicas:                             1,
			HighAvailabilityEnabled:              false,
			HighAvailabilityNumberOfSeedServers:  2,
			HighAvailabilityNumberOfShootClients: 1,
		}

		expectedConfigMap = seedConfigMap(listenAddress, listenAddressV6, dnsLookUpFamily, volumeMountPathCerts, fileNameCABundle, namespace, metricsPort)
	})

	JustBeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-vpn", Namespace: namespace}})).To(Succeed())

		vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, values)
	})

	Describe("#Deploy", func() {
		Context("secret information available", func() {
			JustBeforeEach(func() {
				statefulSet := statefulSet(values.Network.NodeCIDRs, false)
				statefulSet.ResourceVersion = ""
				Expect(c.Create(ctx, statefulSet)).To(Succeed())

				for i := 0; i < 2; i++ {
					destinationRule := indexedDestinationRule(i)
					destinationRule.ResourceVersion = ""
					Expect(c.Create(ctx, destinationRule)).To(Succeed())

					service := indexedService(i)
					service.ResourceVersion = ""
					Expect(c.Create(ctx, service)).To(Succeed())
				}

				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())

				actualSecretServer := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, actualSecretServer)).To(Succeed())
				Expect(actualSecretServer.Immutable).To(PointTo(BeTrue()))
				Expect(actualSecretServer.Data).NotTo(BeEmpty())

				actualSecretTLSAuth := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretNameTLSAuth}, actualSecretTLSAuth)).To(Succeed())
				Expect(actualSecretTLSAuth.Immutable).To(PointTo(BeTrue()))
				Expect(actualSecretTLSAuth.Data).NotTo(BeEmpty())

				actualService := &corev1.Service{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedService.Namespace, Name: expectedService.Name}, actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(expectedService))

				actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
				expectedScrapeConfig := scrapeConfig(values.HighAvailabilityEnabled)
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedScrapeConfig.Namespace, Name: expectedScrapeConfig.Name}, actualScrapeConfig)).To(Succeed())
				Expect(actualScrapeConfig).To(DeepEqual(expectedScrapeConfig))

				actualConfigMap := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedConfigMap.Namespace, Name: expectedConfigMap.Name}, actualConfigMap)).To(Succeed())
				Expect(actualConfigMap).To(DeepEqual(expectedConfigMap))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
				updateMode := vpaUpdateMode
				if values.VPAUpdateDisabled || values.HighAvailabilityEnabled {
					updateMode = vpaautoscalingv1.UpdateModeOff
				}
				expectedVPA := expectedVPAFor(values.HighAvailabilityEnabled, &updateMode)
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVPA.Namespace, Name: expectedVPA.Name}, actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(expectedVPA))

				actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
				expectedDestinationRule := destinationRule()
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, actualDestinationRule)).To(Succeed())
				Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, comptest.CmpOptsForDestinationRule()))

				actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, actualPodDisruptionBudget)).To(BeNotFoundError())

				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, &appsv1.StatefulSet{})).To(BeNotFoundError())
				for i := 0; i < 2; i++ {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(indexedDestinationRule(i)), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(indexedService(i)), &corev1.Service{})).To(BeNotFoundError())
				}
			})

			Context("w/o node network", func() {
				BeforeEach(func() {
					values.Network.NodeCIDRs = nil
				})

				It("should successfully deploy all resources", func() {
					actualDeployment := &appsv1.Deployment{}
					expectedDeployment := deployment(nil)
					Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, actualDeployment)).To(Succeed())
					Expect(actualDeployment).To(DeepEqual(expectedDeployment))
				})
			})

			Context("w/ node network", func() {
				It("should successfully deploy all resources", func() {
					actualDeployment := &appsv1.Deployment{}
					expectedDeployment := deployment(values.Network.NodeCIDRs)
					Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, actualDeployment)).To(Succeed())
					Expect(actualDeployment).To(DeepEqual(expectedDeployment))
				})

				Context("IPv6", func() {
					BeforeEach(func() {
						listenAddress = "0.0.0.0"
						listenAddressV6 = "::"
						dnsLookUpFamily = "ALL"
						networkConfig := NetworkValues{
							PodCIDRs:     []net.IPNet{{IP: net.ParseIP("2001:db8:1::"), Mask: net.CIDRMask(48, 128)}},
							ServiceCIDRs: []net.IPNet{{IP: net.ParseIP("2001:db8:3::"), Mask: net.CIDRMask(48, 128)}},
							IPFamilies:   []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6},
						}
						values.Network = networkConfig
					})

					It("should successfully deploy all resources", func() {
						actualDeployment := &appsv1.Deployment{}
						expectedDeployment := deployment(values.Network.NodeCIDRs)
						Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDeployment.Namespace, Name: expectedDeployment.Name}, actualDeployment)).To(Succeed())
						Expect(actualDeployment).To(DeepEqual(expectedDeployment))
					})
				})
			})

			Context("With VPA update mode set to off", func() {
				BeforeEach(func() {
					values.VPAUpdateDisabled = true
				})

				It("should successfully deploy vpa with update mode set to off", func() {
					actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
					expectedVPA := expectedVPAFor(values.HighAvailabilityEnabled, ptr.To(vpaautoscalingv1.UpdateModeOff))
					Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVPA.Namespace, Name: expectedVPA.Name}, actualVPA)).To(Succeed())
					Expect(actualVPA).To(DeepEqual(expectedVPA))
				})
			})
		})

		Context("High availability (w/o node network)", func() {
			BeforeEach(func() {
				values.Network.NodeCIDRs = nil
				values.Replicas = 3
				values.HighAvailabilityEnabled = true
				values.HighAvailabilityNumberOfSeedServers = 3
				values.HighAvailabilityNumberOfShootClients = 2
			})

			It("should successfully deploy all resources", func() {
				deployment := deployment(values.Network.NodeCIDRs)
				deployment.ResourceVersion = ""
				Expect(c.Create(ctx, deployment)).To(Succeed())

				dr := destinationRule()
				dr.ResourceVersion = ""
				Expect(c.Create(ctx, dr)).To(Succeed())

				svc := expectedService.DeepCopy()
				svc.ResourceVersion = ""
				Expect(c.Create(ctx, svc)).To(Succeed())

				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())

				actualSecretServer := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, actualSecretServer)).To(Succeed())
				Expect(actualSecretServer.Immutable).To(PointTo(BeTrue()))
				Expect(actualSecretServer.Data).NotTo(BeEmpty())

				actualSecretTLSAuth := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretNameTLSAuth}, actualSecretTLSAuth)).To(Succeed())
				Expect(actualSecretTLSAuth.Immutable).To(PointTo(BeTrue()))
				Expect(actualSecretTLSAuth.Data).NotTo(BeEmpty())

				for i := 0; i < 2; i++ {
					actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
					expectedDestinationRule := indexedDestinationRule(i)
					Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, actualDestinationRule)).To(Succeed())
					Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, comptest.CmpOptsForDestinationRule()))

					actualService := &corev1.Service{}
					expectedService := indexedService(i)
					Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedService.Namespace, Name: expectedService.Name}, actualService)).To(Succeed())
					Expect(actualService).To(DeepEqual(expectedService))
				}

				actualConfigMap := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedConfigMap.Namespace, Name: expectedConfigMap.Name}, actualConfigMap)).To(Succeed())
				Expect(actualConfigMap).To(DeepEqual(expectedConfigMap))

				actualPodDisruptionBudget := &policyv1.PodDisruptionBudget{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedPodDisruptionBudget.Namespace, Name: expectedPodDisruptionBudget.Name}, actualPodDisruptionBudget)).To(Succeed())
				Expect(actualPodDisruptionBudget).To(DeepEqual(expectedPodDisruptionBudget))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
				updateMode := vpaUpdateMode
				if values.VPAUpdateDisabled || values.HighAvailabilityEnabled {
					updateMode = vpaautoscalingv1.UpdateModeOff
				}
				expectedVPA := expectedVPAFor(values.HighAvailabilityEnabled, &updateMode)
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVPA.Namespace, Name: expectedVPA.Name}, actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(expectedVPA))

				actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
				expectedScrapeConfig := scrapeConfig(values.HighAvailabilityEnabled)
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedScrapeConfig.Namespace, Name: expectedScrapeConfig.Name}, actualScrapeConfig)).To(Succeed())
				Expect(actualScrapeConfig).To(DeepEqual(expectedScrapeConfig))

				actualStatefulSet := &appsv1.StatefulSet{}
				expectedStatefulSet := statefulSet(values.Network.NodeCIDRs, false)
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedStatefulSet.Namespace, Name: expectedStatefulSet.Name}, actualStatefulSet)).To(Succeed())
				Expect(actualStatefulSet).To(DeepEqual(expectedStatefulSet))

				Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(destinationRule()), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedService), &corev1.Service{})).To(BeNotFoundError())
			})
		})
	})

	Describe("#Destroy", func() {
		JustBeforeEach(func() {
			statefulSet := statefulSet(values.Network.NodeCIDRs, false)
			statefulSet.ResourceVersion = ""
			Expect(c.Create(ctx, statefulSet)).To(Succeed())

			for i := 0; i < 2; i++ {
				destinationRule := indexedDestinationRule(i)
				destinationRule.ResourceVersion = ""
				Expect(c.Create(ctx, destinationRule)).To(Succeed())

				service := indexedService(i)
				service.ResourceVersion = ""
				Expect(c.Create(ctx, service)).To(Succeed())
			}

			deployment := deployment(values.Network.NodeCIDRs)
			deployment.ResourceVersion = ""
			Expect(c.Create(ctx, deployment)).To(Succeed())

			dr := destinationRule()
			dr.ResourceVersion = ""
			Expect(c.Create(ctx, dr)).To(Succeed())

			svc := expectedService.DeepCopy()
			svc.ResourceVersion = ""
			Expect(c.Create(ctx, svc)).To(Succeed())

			sc := scrapeConfig(values.HighAvailabilityEnabled)
			sc.ResourceVersion = ""
			Expect(c.Create(ctx, sc)).To(Succeed())

			vpa := expectedVPAFor(values.HighAvailabilityEnabled, &vpaUpdateMode).DeepCopy()
			vpa.ResourceVersion = ""
			Expect(c.Create(ctx, vpa)).To(Succeed())

			envoyFilter := &networkingv1alpha3.EnvoyFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namespace + "-vpn",
					Namespace: istioNamespace,
				},
			}
			Expect(c.Create(ctx, envoyFilter)).To(Succeed())
		})

		JustAfterEach(func() {
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, &appsv1.StatefulSet{})).To(BeNotFoundError())
			for i := 0; i < 2; i++ {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(indexedDestinationRule(i)), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(indexedService(i)), &corev1.Service{})).To(BeNotFoundError())
			}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "vpn-seed-server"}, &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(destinationRule()), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedService), &corev1.Service{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig(values.HighAvailabilityEnabled)), &monitoringv1alpha1.ScrapeConfig{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedVPAFor(values.HighAvailabilityEnabled, &vpaUpdateMode)), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: istioNamespace, Name: namespace + "-vpn"}, &networkingv1alpha3.EnvoyFilter{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedPodDisruptionBudget), &policyv1.PodDisruptionBudget{})).To(BeNotFoundError())
		})

		It("should successfully destroy all resources", func() {
			vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, values)

			Expect(vpnSeedServer.Destroy(ctx)).To(Succeed())
		})

		It("should successfully destroy all resources (w/ high availability)", func() {
			haValues := values
			haValues.Replicas = 2
			haValues.HighAvailabilityEnabled = true
			haValues.HighAvailabilityNumberOfSeedServers = 2
			haValues.HighAvailabilityNumberOfShootClients = 1
			vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, haValues)

			Expect(vpnSeedServer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(vpnSeedServer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(vpnSeedServer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func seedConfigMap(listenAddress, listenAddressV6, dnsLookUpFamily, volumeMountPathCerts, fileNameCABundle, namespace string, metricsPort int) *corev1.ConfigMap {
	values := map[string]any{
		"listenAddress":   listenAddress,
		"listenAddressV6": listenAddressV6,
		"dnsLookupFamily": dnsLookUpFamily,
		"envoyPort":       EnvoyPort,
		"certChain":       volumeMountPathCerts + `/` + secretsutils.DataKeyCertificate,
		"privateKey":      volumeMountPathCerts + `/` + secretsutils.DataKeyPrivateKey,
		"caCert":          volumeMountPathCerts + `/` + fileNameCABundle,
		"metricsPort":     metricsPort,
	}

	var envoyConfig strings.Builder
	err := tplEnvoy.Execute(&envoyConfig, values)
	Expect(err).ToNot(HaveOccurred())

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "vpn-seed-server-envoy-config",
			Namespace:       namespace,
			ResourceVersion: "1",
		},
		Data: map[string]string{
			"envoy.yaml": envoyConfig.String(),
		},
	}
	Expect(kubernetesutils.MakeUnique(configMap)).To(Succeed())
	return configMap
}
