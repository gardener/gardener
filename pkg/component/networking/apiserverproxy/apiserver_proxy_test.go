// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverproxy_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/networking/apiserverproxy"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("APIServerProxy", func() {
	var (
		ctx = context.Background()

		c  client.Client
		sm secretsmanager.Interface

		consistOf             func(...client.Object) types.GomegaMatcher
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		values                 Values
		component              Interface
		advertiseIPAddress     string
		reversedVPNHeaderValue string

		managedResourceName = "shoot-core-apiserver-proxy"
		namespace           = "shoot--internal--internal"
		image               = "some-image:some-tag"
		sidecarImage        = "sidecar-image:some-tag"
		proxySeedServerHost = "api.internal.local."

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-proxy",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":                 "kubernetes",
					"gardener.cloud/role": "system-component",
					"origin":              "gardener",
					"role":                "apiserver-proxy",
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{
						Name:       "metrics",
						Port:       16910,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt32(16910),
					},
				},
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver-proxy",
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		}

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-apiserver-proxy",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels: ptr.To(false),
				Scheme:      ptr.To("HTTPS"),
				TLSConfig:   &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
					Key:                  "token",
				}},
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					APIServer:  ptr.To("https://kube-apiserver"),
					Role:       "Endpoints",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
					TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: ptr.To("apiserver-proxy"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
						Action:       "keep",
						Regex:        "apiserver-proxy;metrics",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
						TargetLabel:  "pod",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
						TargetLabel:  "node",
					},
					{
						TargetLabel: "__address__",
						Replacement: ptr.To("kube-apiserver:443"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_port_number"},
						Regex:        `(.+);(.+)`,
						TargetLabel:  "__metrics_path__",
						Replacement:  ptr.To("/api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics"),
					},
				},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(envoy_cluster_bind_errors|envoy_cluster_lb_healthy_panic|envoy_cluster_update_attempt|envoy_cluster_update_failure|envoy_cluster_upstream_cx_connect_ms_bucket|envoy_cluster_upstream_cx_length_ms_bucket|envoy_cluster_upstream_cx_none_healthy|envoy_cluster_upstream_cx_rx_bytes_total|envoy_cluster_upstream_cx_tx_bytes_total|envoy_listener_downstream_cx_destroy|envoy_listener_downstream_cx_length_ms_bucket|envoy_listener_downstream_cx_overflow|envoy_listener_downstream_cx_total|envoy_tcp_downstream_cx_no_route|envoy_tcp_downstream_cx_rx_bytes_total|envoy_tcp_downstream_cx_total|envoy_tcp_downstream_cx_tx_bytes_total)$`,
					},
					{
						SourceLabels: []monitoringv1.LabelName{"envoy_cluster_name"},
						Regex:        `^uds_admin$`,
						Action:       "drop",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"envoy_listener_address"},
						Regex:        `^^0.0.0.0_16910$`,
						Action:       "drop",
					},
				},
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apiserver-proxy",
				Namespace: "kube-system",
				Labels: map[string]string{
					"app":                 "kubernetes",
					"gardener.cloud/role": "system-component",
					"origin":              "gardener",
					"role":                "apiserver-proxy",
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
	)

	BeforeEach(func() {
		advertiseIPAddress = "10.2.170.21"
		reversedVPNHeaderValue = "outbound|443||kube-apiserver.shoot--internal--internal.svc.cluster.local"
		values = Values{
			Image:               image,
			SidecarImage:        sidecarImage,
			ProxySeedServerHost: proxySeedServerHost,
			DNSLookupFamily:     "V4_ONLY",
			IstioTLSTermination: false,
		}
	})

	JustBeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}, Data: map[string][]byte{"bundle.crt": []byte("FOOBAR")}})).To(Succeed())

		component = New(c, namespace, sm, values)
		component.SetAdvertiseIPAddress(advertiseIPAddress)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		testFunc := func(hash string) {
			By("Verify that managed resource does not exist yet")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			By("Deploy the managed resource successfully")
			component = New(c, namespace, sm, values)
			component.SetAdvertiseIPAddress(advertiseIPAddress)
			Expect(component.Deploy(ctx)).To(Succeed())

			By("Verify that managed resource is consistent")
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))
			Expect(managedResource).To(consistOf(
				getConfigYAML(hash, values.DNSLookupFamily, advertiseIPAddress, reversedVPNHeaderValue),
				getDaemonSet(hash, advertiseIPAddress),
				service,
				serviceAccount,
			))

			By("Verify that referenced secret is consistent")
			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), actualScrapeConfig)).To(Succeed())
			Expect(actualScrapeConfig).To(DeepEqual(scrapeConfig))
		}

		Context("IPv4", func() {
			It("should deploy the managed resource successfully", func() {
				testFunc("6049033b")
			})
		})

		Context("IPv6", func() {
			BeforeEach(func() {
				values.DNSLookupFamily = "V6_ONLY"
				advertiseIPAddress = "2001:db8::1"
			})

			It("should deploy the managed resource successfully", func() {
				testFunc("5460b295")
			})
		})

		Context("IstioTLSTermination", func() {
			BeforeEach(func() {
				values.IstioTLSTermination = true
				reversedVPNHeaderValue = fmt.Sprintf("%s--kube-apiserver-socket", namespace)
			})

			It("should deploy the managed resource successfully", func() {
				testFunc("7b4e78d0")
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			scrapeConfig.ResourceVersion = ""

			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func getConfigYAML(hash, dnsLookUpFamily, advertiseIPAddress, xGardenerDestination string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "apiserver-proxy-config-" + hash,
			Namespace: "kube-system",
			Labels: map[string]string{
				"app":                 "kubernetes",
				"gardener.cloud/role": "system-component",
				"origin":              "gardener",
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"role": "apiserver-proxy",
			},
		},
		Immutable: ptr.To(true),
		Data: map[string]string{
			"envoy.yaml": `layered_runtime:
  layers:
    - name: static_layer_0
      static_layer:
        envoy:
          resource_limits:
            listener:
              kube_apiserver:
                connection_limit: 10000
        overload:
          global_downstream_max_connections: 10000
admin:
  access_log:
  - name: envoy.access_loggers.stdout
    # Remove spammy readiness/liveness probes and metrics requests from access log
    filter:
      and_filter:
        filters:
        - header_filter:
            header:
              name: :Path
              string_match:
                exact: /ready
              invert_match: true
        - header_filter:
            header:
              name: :Path
              string_match:
                exact: /stats/prometheus
              invert_match: true
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
  address:
    pipe:
      # The admin interface should not be exposed as a TCP address.
      # It's only used and exposed via the metrics lister that
      # exposes only /stats/prometheus path for metrics scrape.
      path: /etc/admin-uds/admin.socket
static_resources:
  listeners:
  - name: kube_apiserver
    address:
      socket_address:
        address: ` + advertiseIPAddress + `
        port_value: 443
    per_connection_buffer_limit_bytes: 32768 # 32 KiB
    filter_chains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          stat_prefix: kube_apiserver
          cluster: kube_apiserver
          tunneling_config:
            # hostname is irrelevant as it will be dropped by envoy, we still need it for the configuration though
            hostname: "api.internal.local.:443"
            headers_to_add:
            - header:
                key: Reversed-VPN
                value: "` + xGardenerDestination + `"
          access_log:
          - name: envoy.access_loggers.stdout
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
              log_format:
                text_format_source:
                  inline_string: "[%START_TIME%] %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%UPSTREAM_HOST%\"\n"
  - name: metrics
    address:
      socket_address:
        address: "0.0.0.0"
        port_value: 16910
    additional_addresses:
    - address:
        socket_address:
          address: "::"
          port_value: 16910
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          use_remote_address: true
          common_http_protocol_options:
            idle_timeout: 8s
            max_connection_duration: 10s
            max_headers_count: 20
            max_stream_duration: 8s
            headers_with_underscores_action: REJECT_REQUEST
          http2_protocol_options:
            max_concurrent_streams: 5
            initial_stream_window_size: 65536
            initial_connection_window_size: 1048576
          stream_idle_timeout: 8s
          request_timeout: 9s
          codec_type: AUTO
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match:
                  path: /metrics
                route:
                  cluster: uds_admin
                  prefix_rewrite: /stats/prometheus
              - match:
                  path: /ready
                route:
                  cluster: uds_admin
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

  clusters:
  - name: kube_apiserver
    connect_timeout: 5s
    per_connection_buffer_limit_bytes: 32768 # 32 KiB
    type: LOGICAL_DNS
    dns_lookup_family: ` + dnsLookUpFamily + `
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: kube_apiserver
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: api.internal.local.
                port_value: 8132
    upstream_connection_options:
      tcp_keepalive:
        keepalive_time: 7200
        keepalive_interval: 55
  - name: uds_admin
    connect_timeout: 0.25s
    type: STATIC
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: uds_admin
      endpoints:
      - lb_endpoints:
          - endpoint:
              address:
                pipe:
                  path: /etc/admin-uds/admin.socket
    transport_socket:
      name: envoy.transport_sockets.raw_buffer
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer
`},
	}
}

func getDaemonSet(hash string, advertiseIPAddress string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				references.AnnotationKey(references.KindConfigMap, "apiserver-proxy-config-"+hash): "apiserver-proxy-config-" + hash,
			},
			Name:      "apiserver-proxy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"app":                                    "kubernetes",
				"gardener.cloud/role":                    "system-component",
				"node.gardener.cloud/critical-component": "true",
				"origin":                                 "gardener",
				"role":                                   "apiserver-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver-proxy",
				},
			},
			RevisionHistoryLimit: ptr.To[int32](2),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						references.AnnotationKey(references.KindConfigMap, "apiserver-proxy-config-"+hash): "apiserver-proxy-config-" + hash,
					},
					Labels: map[string]string{
						"app":                                    "kubernetes",
						"gardener.cloud/role":                    "system-component",
						"networking.gardener.cloud/from-seed":    "allowed",
						"networking.gardener.cloud/to-apiserver": "allowed",
						"networking.gardener.cloud/to-dns":       "allowed",
						"node.gardener.cloud/critical-component": "true",
						"origin":                                 "gardener",
						"role":                                   "apiserver-proxy",
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
					Containers: []corev1.Container{
						{
							Args:            []string{"--ip-address=" + advertiseIPAddress, "--interface=lo"},
							Image:           "sidecar-image:some-tag",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "sidecar",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("90Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("15Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_ADMIN"},
								},
							},
						},
						{
							Command:         []string{"envoy", "--concurrency", "2", "--use-dynamic-base-id", "-c", "/etc/apiserver-proxy/envoy.yaml"},
							Image:           "some-image:some-tag",
							ImagePullPolicy: corev1.PullIfNotPresent,
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(16910),
									},
								},
								InitialDelaySeconds: 1,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							Name: "proxy",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 16910,
									HostPort:      16910,
									Name:          "metrics",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(16910),
									},
								},
								InitialDelaySeconds: 1,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								TimeoutSeconds:      1,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("30Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_BIND_SERVICE"},
								},
								RunAsUser: func(i int64) *int64 { return &i }(0),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/etc/apiserver-proxy",
									Name:      "proxy-config",
								},
								{
									MountPath: "/etc/admin-uds",
									Name:      "admin-uds",
								},
							},
						},
					},
					HostNetwork: true,
					InitContainers: []corev1.Container{
						{
							Args:            []string{"--ip-address=" + advertiseIPAddress, "--daemon=false", "--interface=lo"},
							Image:           "sidecar-image:some-tag",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "setup",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"NET_ADMIN"},
								},
							},
						},
					},
					PriorityClassName: "system-node-critical",
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					ServiceAccountName: "apiserver-proxy",
					Tolerations: []corev1.Toleration{
						{
							Effect:   corev1.TaintEffectNoSchedule,
							Operator: corev1.TolerationOpExists,
						},
						{
							Effect:   corev1.TaintEffectNoExecute,
							Operator: corev1.TolerationOpExists,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "proxy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "apiserver-proxy-config-" + hash,
									},
								},
							},
						},
						{
							Name: "admin-uds",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
		},
	}
}
