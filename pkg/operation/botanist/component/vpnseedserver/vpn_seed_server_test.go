// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/google/go-cmp/cmp"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	istionetworkingv1beta1 "istio.io/api/networking/v1beta1"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	networkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("VpnSeedServer", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		sm            secretsmanager.Interface
		vpnSeedServer Interface

		ctx                     = context.TODO()
		fakeErr                 = fmt.Errorf("fake error")
		namespace               = "shoot--foo--bar"
		vpnImage                = "eu.gcr.io/gardener-project/gardener/vpn-seed-server:v1.2.3"
		envoyImage              = "envoyproxy/envoy:v4.5.6"
		kubeAPIServerHost       = "foo.bar"
		serviceNetwork          = "10.0.0.0/24"
		podNetwork              = "10.0.1.0/24"
		nodeNetwork             = "10.0.2.0/24"
		replicas          int32 = 1
		vpaUpdateMode           = vpaautoscalingv1.UpdateModeAuto
		controlledValues        = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		namespaceUID        = types.UID("123456")
		istioLabels         = map[string]string{"foo": "bar"}
		istioNamespace      = "istio-foo"
		istioIngressGateway = IstioIngressGateway{
			Namespace: istioNamespace,
			Labels:    istioLabels,
		}

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
      typed_config: {}
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
                    exact_match: GET
                route:
                  cluster: prometheus_stats
                  prefix_rewrite: "/stats/prometheus"
          http_filters:
          - name: envoy.filters.http.router
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
                path: /var/run/envoy.admin
admin:
  address:
    pipe:
      path: /var/run/envoy.admin`,
			},
		}

		secretDH = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-server-dh", Namespace: namespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       secretDataDH,
		}
	)

	Expect(kutil.MakeUnique(configMap)).To(Succeed())
	Expect(kutil.MakeUnique(secretDH)).To(Succeed())

	var (
		deployment = func(nodeNetwork *string) *appsv1.Deployment {
			maxSurge := intstr.FromInt(100)
			maxUnavailable := intstr.FromInt(0)
			hostPathCharDev := corev1.HostPathCharDev
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      DeploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:                      v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:                        DeploymentName,
						"networking.gardener.cloud/from-shoot-apiserver": "allowed",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             pointer.Int32(replicas),
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
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.GardenRole:                          v1beta1constants.GardenRoleControlPlane,
								v1beta1constants.LabelApp:                            DeploymentName,
								v1beta1constants.LabelNetworkPolicyToShootNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyFromPrometheus:    v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Spec: corev1.PodSpec{
							AutomountServiceAccountToken: pointer.Bool(false),
							PriorityClassName:            v1beta1constants.PriorityClassNameShootControlPlane,
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
											},
										},
									},
									Env: []corev1.EnvVar{
										{
											Name:  "SERVICE_NETWORK",
											Value: serviceNetwork,
										},
										{
											Name:  "POD_NETWORK",
											Value: podNetwork,
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
								{
									Name:            "envoy-proxy",
									Image:           envoyImage,
									ImagePullPolicy: corev1.PullIfNotPresent,
									SecurityContext: &corev1.SecurityContext{
										Capabilities: &corev1.Capabilities{
											Add: []corev1.Capability{
												"NET_BIND_SERVICE",
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
								{
									Name: "envoy-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMap.Name,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			if nodeNetwork != nil {
				deploy.Spec.Template.Spec.Containers[0].Env = append(deploy.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "NODE_NETWORK", Value: *nodeNetwork})
			}

			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}

		destinationRule = &networkingv1beta1.DestinationRule{
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
					Tls: &istionetworkingv1beta1.ClientTLSSettings{
						Mode: istionetworkingv1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}

		networkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-to-vpn-seed-server",
				Namespace: namespace,
				Annotations: map[string]string{
					"gardener.cloud/description": "Allows only Ingress/Egress between the kube-apiserver of the same control plane and the corresponding vpn-seed-server and Ingress from the istio ingress gateway to the vpn-seed-server.",
				},
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
						v1beta1constants.LabelApp:   DeploymentName,
					},
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
										v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
										v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
									},
								},
							},
						},
					},
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring,
										v1beta1constants.LabelApp:   v1beta1constants.StatefulSetNamePrometheus,
										v1beta1constants.LabelRole:  v1beta1constants.GardenRoleMonitoring,
									},
								},
							},
						},
					},
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								// we don't want to modify existing labels on the istio namespace
								NamespaceSelector: &metav1.LabelSelector{},
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: "istio-ingressgateway",
									},
								},
							},
						},
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						To: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
										v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
										v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
									},
								},
							},
						},
					},
				},
				PolicyTypes: []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ServiceName,
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.istio.io/exportTo": "*",
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
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
							ControlledValues: &controlledValues,
						},
						{
							ContainerName: "envoy-proxy",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
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

		By("expecting secrets managed outside of this package for whose secretsmanager.Get() will be called")
		c.EXPECT().Get(ctx, kutil.Key(namespace, "ca-vpn"), gomock.AssignableToTypeOf(&corev1.Secret{})).AnyTimes()

		vpnSeedServer = New(c, namespace, sm, envoyImage, vpnImage, &kubeAPIServerHost, serviceNetwork, podNetwork, nil, replicas, istioIngressGateway)
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

			It("should fail because the networkpolicy cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server"))
					}),
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
						Expect(obj.GetName()).To(HavePrefix("vpn-seed-server-tlsauth"))
					}),
					c.EXPECT().Create(ctx, configMap),
					c.EXPECT().Create(ctx, secretDH),
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).Return(fakeErr),
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy all resources (w/o node network)", func() {
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(networkPolicy))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment(nil)))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(cmp.Diff(destinationRule, obj, protocmp.Transform())).To(BeEmpty())
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				)
				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy all resources (w/ node network)", func() {
				vpnSeedServer = New(c, namespace, sm, envoyImage, vpnImage, &kubeAPIServerHost, serviceNetwork, podNetwork, &nodeNetwork, replicas, istioIngressGateway)
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
					c.EXPECT().Get(ctx, kutil.Key(namespace, "allow-to-vpn-seed-server"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(networkPolicy))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deployment(&nodeNetwork)))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName), gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&networkingv1beta1.DestinationRule{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(cmp.Diff(destinationRule, obj, protocmp.Transform())).To(BeEmpty())
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ServiceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(service))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, DeploymentName+"-vpa"), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
				)

				Expect(vpnSeedServer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		It("should fail because the networkpolicy cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the destinationRule cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the envoy filter cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the dh secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}).Return(fakeErr),
			)

			Expect(vpnSeedServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully destroy all resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "allow-to-vpn-seed-server"}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &networkingv1beta1.DestinationRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: ServiceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-vpa"}}),
				c.EXPECT().Delete(ctx, &networkingv1alpha3.EnvoyFilter{ObjectMeta: metav1.ObjectMeta{Namespace: istioNamespace, Name: namespace + "-vpn"}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: DeploymentName + "-dh"}}),
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
