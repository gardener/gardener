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

package vpnseedserver_test

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VpnSeedServer", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		sm            secretsmanager.Interface
		vpnSeedServer Interface

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"
		vpnImage  = "eu.gcr.io/gardener-project/gardener/vpn-seed-server:v1.2.3"
		values    = Values{
			ImageAPIServerProxy: "envoyproxy/envoy:v4.5.6",
			ImageVPNSeedServer:  vpnImage,
			KubeAPIServerHost:   pointer.String("foo.bar"),
			Network: NetworkValues{
				PodCIDR:     "10.0.1.0/24",
				ServiceCIDR: "10.0.0.0/24",
				NodeCIDR:    "10.0.2.0/24",
			},
			Replicas:                             1,
			HighAvailabilityEnabled:              false,
			HighAvailabilityNumberOfSeedServers:  2,
			HighAvailabilityNumberOfShootClients: 1,
			SeedVersion:                          semver.MustParse("1.22.1"),
		}

		istioNamespace     = "istio-foo"
		istioNamespaceFunc = func() string { return istioNamespace }

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		namespaceUID     = types.UID("123456")

		secretNameDH     = "vpn-seed-server-dh"
		secretChecksumDH = "9012"
		secretDataDH     = map[string][]byte{"dh2048.pem": []byte("baz")}
		secrets          = Secrets{
			DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH, Data: secretDataDH},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpn-seed-server-envoy-config",
				Namespace: namespace,
			},
			Data: map[string]string{
				"envoy.yaml": `static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: 9443
    listener_filters:
    - name: "envoy.filters.listener.tls_inspector"
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.listener.tls_inspector.v3.TlsInspector
    filter_chains:
    - transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          common_tls_context:
            tls_certificates:
            - certificate_chain: { filename: "/srv/secrets/vpn-server/tls.crt" }
              private_key: { filename: "/srv/secrets/vpn-server/tls.key" }
            validation_context:
              trusted_ca:
                filename: /srv/secrets/vpn-server/ca.crt
      filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.stdout
            filter:
              or_filter:
                filters:
                - status_code_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 500
                        runtime_key: "null"
                - duration_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 1000
                        runtime_key: "null"
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
              log_format:
                text_format_source:
                  inline_string: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n"
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster: dynamic_forward_proxy_cluster
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config: {}
          http_filters:
          - name: envoy.filters.http.dynamic_forward_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.dynamic_forward_proxy.v3.FilterConfig
              dns_cache_config:
                name: dynamic_forward_proxy_cache_config
                dns_lookup_family: V4_ONLY
                max_hosts: 8192
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT
  - name: metrics_listener
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 15000
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: stats_server
          route_config:
            virtual_hosts:
            - name: admin_interface
              domains:
              - "*"
              routes:
              - match:
                  prefix: "/metrics"
                  headers:
                  - name: ":method"
                    string_match:
                      exact: GET
                route:
                  cluster: prometheus_stats
                  prefix_rewrite: "/stats/prometheus"
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
  clusters:
  - name: dynamic_forward_proxy_cluster
    connect_timeout: 20s
    circuitBreakers:
      thresholds:
      - maxConnections: 8192
    lb_policy: CLUSTER_PROVIDED
    cluster_type:
      name: envoy.clusters.dynamic_forward_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.clusters.dynamic_forward_proxy.v3.ClusterConfig
        dns_cache_config:
          name: dynamic_forward_proxy_cache_config
          dns_lookup_family: V4_ONLY
          max_hosts: 8192
  - name: prometheus_stats
    connect_timeout: 0.25s
    type: static
    load_assignment:
      cluster_name: prometheus_stats
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              pipe:
                path: /home/nonroot/envoy.admin
admin:
  address:
    pipe:
      path: /home/nonroot/envoy.admin`,
			},
		}

		secretDH = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-server-dh", Namespace: namespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       secretDataDH,
		}
	)

	Expect(kubernetesutils.MakeUnique(configMap)).To(Succeed())
	Expect(kubernetesutils.MakeUnique(secretDH)).To(Succeed())

	var (
		deploymentObjectMeta = &metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: namespace,
			Labels: map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				v1beta1constants.LabelApp:   DeploymentName,
			},
		}

		template = func(nodeNetwork string, highAvailability bool) *corev1.PodTemplateSpec {
			hostPathCharDev := corev1.HostPathCharDev
			template := &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1beta1constants.GardenRole:                                     v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:                                       DeploymentName,
						v1beta1constants.LabelNetworkPolicyToShootNetworks:              v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToDNS:                        v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToPrivateNetworks:            v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: pointer.Bool(false),
					PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane300,
					DNSPolicy:                    corev1.DNSDefault, // make sure to not use the coredns for DNS resolution.
					Containers: []corev1.Container{
						{
							Name:            DeploymentName,
							Image:           vpnImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "tcp-tunnel",
									ContainerPort: 1194,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"NET_ADMIN",
										"NET_RAW",
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SERVICE_NETWORK",
									Value: values.Network.ServiceCIDR,
								},
								{
									Name:  "POD_NETWORK",
									Value: values.Network.PodCIDR,
								},
								{
									Name:  "NODE_NETWORK",
									Value: nodeNetwork,
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
										Port: intstr.FromInt(1194),
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(1194),
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
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
								{
									Name:      "dh",
									MountPath: "/srv/secrets/dh",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: pointer.Int64(30),
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
									DefaultMode: pointer.Int32(420),
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
									SecretName:  "vpn-seed-server-tlsauth-a1d0aa00",
									DefaultMode: pointer.Int32(0400),
								},
							},
						},
						{
							Name: "dh",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretDH.Name,
									DefaultMode: pointer.Int32(0400),
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
						Name:  "CLIENT_TO_CLIENT",
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
					{
						Name:  "HA_VPN_CLIENTS",
						Value: "2",
					},
				}...)
				template.Spec.Containers[0].VolumeMounts = append(template.Spec.Containers[0].VolumeMounts, mount)
				template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
					Name:            "openvpn-exporter",
					Image:           vpnImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"/openvpn-exporter",
						"-openvpn.status_paths",
						"/srv/status/openvpn.status",
						"-web.listen-address",
						":15000",
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 15000,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(15000),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(15000),
							},
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"all",
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("20m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{mount},
				})
				template.Spec.Volumes = append(template.Spec.Volumes, corev1.Volume{
					Name: "openvpn-status",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
			} else {
				template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
					Name:            "envoy-proxy",
					Image:           values.ImageAPIServerProxy,
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{
								"all",
							},
						},
					},
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
								Port: intstr.FromInt(9443),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(9443),
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("20m"),
							corev1.ResourceMemory: resource.MustParse("20Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("850M"),
						},
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
								Name: configMap.Name,
							},
						},
					},
				})
			}
			return template
		}

		deployment = func(nodeNetwork string) *appsv1.Deployment {
			maxSurge := intstr.FromInt(100)
			maxUnavailable := intstr.FromInt(0)
			deploy := &appsv1.Deployment{
				ObjectMeta: *deploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas:             pointer.Int32(values.Replicas),
					RevisionHistoryLimit: pointer.Int32(1),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
						v1beta1constants.LabelApp: DeploymentName,
					}},
					Strategy: appsv1.DeploymentStrategy{
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxUnavailable: &maxUnavailable,
							MaxSurge:       &maxSurge,
						},
						Type: appsv1.RollingUpdateDeploymentStrategyType,
					},
					Template: *template(nodeNetwork, false),
				},
			}

			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}

		statefulSet = func(nodeNetwork string) *appsv1.StatefulSet {
			sts := &appsv1.StatefulSet{
				ObjectMeta: *deploymentObjectMeta,
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy:  appsv1.ParallelPodManagement,
					Replicas:             pointer.Int32(3),
					RevisionHistoryLimit: pointer.Int32(1),
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
						v1beta1constants.LabelApp: DeploymentName,
					}},
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
					},
					Template: *template(nodeNetwork, true),
				},
			}

			Expect(references.InjectAnnotations(sts)).To(Succeed())
			return sts
		}

		destinationRule = func() *networkingv1beta1.DestinationRule {
			return &networkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: namespace},
				Spec: istionetworkingv1beta1.DestinationRule{
					ExportTo: []string{"*"},
					Host:     fmt.Sprintf("%s.%s.svc.cluster.local", DeploymentName, namespace),
					TrafficPolicy: &istionetworkingv1beta1.TrafficPolicy{
						ConnectionPool: &istionetworkingv1beta1.ConnectionPoolSettings{
							Tcp: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
								MaxConnections: 5000,
								TcpKeepalive: &istionetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
									Interval: &durationpb.Duration{
										Seconds: 75,
									},
									Time: &durationpb.Duration{
										Seconds: 7200,
									},
								},
							},
						},
						LoadBalancer: &istionetworkingv1beta1.LoadBalancerSettings{
							LocalityLbSetting: &istionetworkingv1beta1.LocalityLoadBalancerSetting{
								Enabled:          &wrapperspb.BoolValue{Value: true},
								FailoverPriority: []string{"topology.kubernetes.io/zone"},
							},
						},
						OutlierDetection: &istionetworkingv1beta1.OutlierDetection{
							MinHealthPercent: 0,
						},
						Tls: &istionetworkingv1beta1.ClientTLSSettings{
							Mode: istionetworkingv1beta1.ClientTLSSettings_DISABLE,
						},
					},
				},
			}
		}

		indexedDestinationRule = func(idx int) *networkingv1beta1.DestinationRule {
			destRule := destinationRule()
			destRule.Name = fmt.Sprintf("%s-%d", DeploymentName, idx)
			destRule.Spec.Host = fmt.Sprintf("%s.%s.svc.cluster.local", destRule.Name, namespace)
			return destRule
		}

		maxUnavailable      = intstr.FromInt(1)
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DeploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"app": "vpn-seed-server",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &maxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpn-seed-server",
					},
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceName,
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.istio.io/exportTo":                                           "*",
					"networking.resources.gardener.cloud/namespace-selectors":                `[{"matchLabels":{"gardener.cloud/role":"istio-ingress"}},{"matchExpressions":[{"key":"handler.exposureclass.gardener.cloud/name","operator":"Exists"}]}]`,
					"networking.resources.gardener.cloud/pod-label-selector-namespace-alias": "all-shoots",
					"networking.resources.gardener.cloud/from-policy-pod-label-selector":     "all-scrape-targets",
					"networking.resources.gardener.cloud/from-policy-allowed-ports":          `[{"protocol":"TCP","port":15000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       DeploymentName,
						Port:       1194,
						TargetPort: intstr.FromInt(1194),
					},
					{
						Name:       "http-proxy",
						Port:       9443,
						TargetPort: intstr.FromInt(9443),
					},
					{
						Name:       "metrics",
						Port:       15000,
						TargetPort: intstr.FromInt(15000),
					},
				},
				Selector: map[string]string{
					v1beta1constants.LabelApp: DeploymentName,
				},
			},
		}

		indexedService = func(idx int) *corev1.Service {
			svc := *service
			svc.Name = fmt.Sprintf("%s-%d", ServiceName, idx)
			svc.Spec.Selector = map[string]string{
				"statefulset.kubernetes.io/pod-name": svc.Name,
			}
			return &svc
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName + "-vpa", Namespace: namespace},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       DeploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: DeploymentName,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
							ControlledValues: &controlledValues,
						},
						{
							ContainerName: "envoy-proxy",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		sm = fakesecretsmanager.New(c, namespace)

		By("Ensure secrets managed outside of this package for whose secretsmanager.Get() will be called")
		c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, "ca-vpn"), gomock.AssignableToTypeOf(&corev1.Secret{})).AnyTimes()

		vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, values)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the DH secret information is not provided", func() {
				vpnSeedServer.SetSecrets(Secrets{})
				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing DH secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				vpnSeedServer.SetSecrets(secrets)
				vpnSeedServer.SetSeedNamespaceObjectUID(namespaceUID)
			})

			It("should fail because the server secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) error {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
						return fakeErr
					}),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the tls auth secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) error {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
						return fakeErr
					}),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the configmap cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the dh secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the destinationRule cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()),

					c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),

					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy all resources (w/o node network)", func() {
				copy := values
				copy.Network.NodeCIDR = ""
				vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, copy)
				vpnSeedServer.SetSecrets(secrets)
				vpnSeedServer.SetSeedNamespaceObjectUID(namespaceUID)

				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(secretDH))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment("")))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(BeComparableTo(destinationRule(), test.CmpOptsForDestinationRule()))
						}),

					c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),

					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)
				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy all resources (w/o node network and high availability)", func() {
				haValues := values
				haValues.Network.NodeCIDR = ""
				haValues.Replicas = 3
				haValues.HighAvailabilityEnabled = true
				haValues.HighAvailabilityNumberOfSeedServers = 3
				haValues.HighAvailabilityNumberOfShootClients = 2

				vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, haValues)
				vpnSeedServer.SetSecrets(secrets)
				vpnSeedServer.SetSeedNamespaceObjectUID(namespaceUID)

				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(secretDH))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.StatefulSet{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(statefulSet("")))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(podDisruptionBudget))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName+"-0"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(indexedService(0)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-0"), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(BeComparableTo(indexedDestinationRule(0), test.CmpOptsForDestinationRule()))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName+"-1"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(indexedService(1)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-1"), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(BeComparableTo(indexedDestinationRule(1), test.CmpOptsForDestinationRule()))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName+"-2"), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(indexedService(2)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-2"), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(BeComparableTo(indexedDestinationRule(2), test.CmpOptsForDestinationRule()))
						}),

					c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),

					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)
				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy all resources (w/ node network)", func() {
				vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, values)
				vpnSeedServer.SetSecrets(secrets)
				vpnSeedServer.SetSeedNamespaceObjectUID(namespaceUID)

				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMap))
						}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(secretDH))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment(values.Network.NodeCIDR)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(BeComparableTo(destinationRule(), test.CmpOptsForDestinationRule()))
						}),

					c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
					c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),
					c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),

					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the statefulset cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the destinationRule cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the envoy filter cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully destroy all resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName + "-0"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName + "-1"}}),
			)
			Expect(vpnSeedServer.Destroy(ctx)).To(Succeed())
		})

		It("should successfully destroy all resources (w/ high availability)", func() {
			haValues := values
			haValues.Replicas = 2
			haValues.HighAvailabilityEnabled = true
			haValues.HighAvailabilityNumberOfSeedServers = 2
			haValues.HighAvailabilityNumberOfShootClients = 1
			vpnSeedServer = New(c, namespace, sm, istioNamespaceFunc, haValues)

			gomock.InOrder(
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-0"}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName + "-0"}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-1"}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName + "-1"}}),
			)

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
