// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodelocaldns_test

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/nodelocaldns"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeLocalDNS", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-core-node-local-dns"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		manifests             []string
		expectedManifests     []string
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		ipvsAddress           = "169.254.20.10"
		labelKey              = "k8s-app"
		labelValue            = "node-local-dns"
		prometheusPort        = 9253
		prometheusErrorPort   = 9353
		prometheusScrape      = true
		livenessProbePort     = 8099
		configMapHash         string
		upstreamDNSAddress    = []string{"__PILLAR__UPSTREAM__SERVERS__"}
		forceTcpToClusterDNS  = "force_tcp"
		forceTcpToUpstreamDNS = "force_tcp"

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-node-local-dns",
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
					Role:       "Pod",
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
						Replacement: ptr.To("node-local-dns"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
						Action:       "keep",
						Regex:        "node-local.*",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name", "__meta_kubernetes_pod_container_port_name"},
						Action:       "keep",
						Regex:        "node-cache;metrics",
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
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Action:       "keep",
					Regex:        `^(coredns_build_info|coredns_cache_entries|coredns_cache_hits_total|coredns_cache_misses_total|coredns_dns_request_duration_seconds_count|coredns_dns_request_duration_seconds_bucket|coredns_dns_requests_total|coredns_dns_responses_total|coredns_forward_requests_total|coredns_forward_responses_total|coredns_kubernetes_dns_programming_duration_seconds_bucket|coredns_kubernetes_dns_programming_duration_seconds_count|coredns_kubernetes_dns_programming_duration_seconds_sum|process_max_fds|process_open_fds)$`,
				}},
			},
		}
		scrapeConfigErrors = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-node-local-dns-errors",
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
					Role:       "Pod",
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
						Replacement: ptr.To("node-local-dns-errors"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
						Action:       "keep",
						Regex:        "node-local.*",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name", "__meta_kubernetes_pod_container_port_name"},
						Action:       "keep",
						Regex:        "node-cache;errormetrics",
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
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Action:       "keep",
					Regex:        `^(coredns_nodecache_setup_errors_total)$`,
				}},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:             image,
			KubernetesVersion: semver.MustParse("1.31.1"),
			IPFamilies:        []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
		}

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
		var (
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: node-local-dns
  namespace: kube-system
`
			configMapYAMLFor = func() string {

				out := `apiVersion: v1
data:
  Corefile: |
    cluster.local:53 {
        errors
        cache {
                success 9984 30
                denial 9984 5
        }
        reload
        loop
        bind ` + bindIP(values) + `
        forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
                ` + forceTcpToClusterDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        health ` + healthAddress(values) + `:` + strconv.Itoa(livenessProbePort) + `
        }
    in-addr.arpa:53 {
        errors
        cache 30
        reload
        loop
        bind ` + bindIP(values) + `
        forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
                ` + forceTcpToClusterDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        }
    ip6.arpa:53 {
        errors
        cache 30
        reload
        loop
        bind ` + bindIP(values) + `
        forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
                ` + forceTcpToClusterDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        }
    .:53 {
        errors
        cache 30
        reload
        loop
        bind ` + bindIP(values) + `
        forward . ` + strings.Join(upstreamDNSAddress, " ") + ` {
                ` + forceTcpToUpstreamDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        }
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    k8s-app: node-local-dns
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: node-local-dns-` + configMapHash + `
  namespace: kube-system
`

				return out

			}
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kube-dns-upstream
  name: kube-dns-upstream
  namespace: kube-system
spec:
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 8053
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 8053
  selector:
    k8s-app: kube-dns
status:
  loadBalancer: {}
`
			maxUnavailable       = intstr.FromString("10%")
			hostPathFileOrCreate = corev1.HostPathFileOrCreate
			daemonSetYAMLFor     = func() *appsv1.DaemonSet {
				daemonset := &appsv1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "DaemonSet",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-local-dns",
						Namespace: metav1.NamespaceSystem,
						Labels: map[string]string{
							labelKey:                                    labelValue,
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
						RevisionHistoryLimit: ptr.To[int32](2),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelKey: labelValue,
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									labelKey:                                    labelValue,
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
								ServiceAccountName: "node-local-dns",
								HostNetwork:        true,
								DNSPolicy:          corev1.DNSDefault,
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
								SecurityContext: &corev1.PodSecurityContext{
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
								},
								Containers: []corev1.Container{
									{
										Name:  "node-cache",
										Image: values.Image,
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("200Mi"),
											},
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("25m"),
												corev1.ResourceMemory: resource.MustParse("25Mi"),
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
											containerArg(values),
											"-conf",
											"/etc/Corefile",
											"-upstreamsvc",
											"kube-dns-upstream",
											"-health-port",
											"8099",
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
												Name:          "metrics",
												Protocol:      corev1.ProtocolTCP,
											},
											{
												ContainerPort: int32(prometheusErrorPort),
												Name:          "errormetrics",
												Protocol:      corev1.ProtocolTCP,
											},
										},
										LivenessProbe: &corev1.Probe{
											ProbeHandler: corev1.ProbeHandler{
												HTTPGet: &corev1.HTTPGetAction{
													Host: ipvsAddress,
													Path: "/health",
													Port: intstr.FromInt32(int32(livenessProbePort)),
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
													Name: "node-local-dns-" + configMapHash,
												},
												Items: []corev1.KeyToPath{
													{
														Key:  "Corefile",
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
				return daemonset
			}
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: node-local-dns
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-local-dns
  updatePolicy:
    updateMode: Auto
status: {}
`
		)

		JustBeforeEach(func() {
			component = New(c, namespace, values)
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(component.Deploy(ctx)).To(Succeed())

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

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), actualScrapeConfig)).To(Succeed())
			Expect(actualScrapeConfig).To(DeepEqual(scrapeConfig))

			actualScrapeConfigErrors := &monitoringv1alpha1.ScrapeConfig{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfigErrors), actualScrapeConfigErrors)).To(Succeed())
			Expect(actualScrapeConfigErrors).To(DeepEqual(scrapeConfigErrors))

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			expectedManifests = append(expectedManifests, serviceAccountYAML, serviceYAML)

			DeferCleanup(func() {
				expectedManifests = nil
			})
		})

		Context("NodeLocalDNS with ipvsEnabled not enabled", func() {
			BeforeEach(func() {
				values.ClusterDNS = []string{"__PILLAR__CLUSTER__DNS__"}
				values.DNSServers = []string{"1.2.3.4", "2001:db8::1"}
			})

			Context("ConfigMap", func() {
				JustBeforeEach(func() {
					configMapData := map[string]string{
						"Corefile": `cluster.local:53 {
    errors
    cache {
            success 9984 30
            denial 9984 5
    }
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + healthAddress(values) + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
.:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(upstreamDNSAddress, " ") + ` {
            ` + forceTcpToUpstreamDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
					}
					configMapHash = utils.ComputeConfigMapChecksum(configMapData)[:8]
				})

				Context("ForceTcpToClusterDNS : true and ForceTcpToUpstreamDNS : true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(true),
							ForceTCPToUpstreamDNS:       ptr.To(true),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
					})

					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("ForceTcpToClusterDNS : true and ForceTcpToUpstreamDNS : false", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(true),
							ForceTCPToUpstreamDNS:       ptr.To(false),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})

					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})
				Context("ForceTcpToClusterDNS : false and ForceTcpToUpstreamDNS : true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(false),
							ForceTCPToUpstreamDNS:       ptr.To(true),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "force_tcp"
					})

					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("ForceTcpToClusterDNS : false and ForceTcpToUpstreamDNS : false", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(false),
							ForceTCPToUpstreamDNS:       ptr.To(false),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("DisableForwardToUpstreamDNS true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(true),
							ForceTCPToUpstreamDNS:       ptr.To(true),
							DisableForwardToUpstreamDNS: ptr.To(true),
						}
						values.VPAEnabled = true
						upstreamDNSAddress = values.ClusterDNS
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "force_tcp"
					})

					It("should successfully deploy all resources", func() {
						expectedManifests = append(expectedManifests, configMapYAMLFor())
						Expect(manifests).To(ContainElements(expectedManifests))

						managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
						Expect(err).ToNot(HaveOccurred())
						daemonset := daemonSetYAMLFor()
						utilruntime.Must(references.InjectAnnotations(daemonset))
						Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
					})
				})
			})
		})

		Context("NodeLocalDNS with ipvsEnabled enabled", func() {
			BeforeEach(func() {
				values.ClusterDNS = []string{"1.2.3.4", "2001:db8::1"}
				values.DNSServers = nil
				upstreamDNSAddress = []string{"__PILLAR__UPSTREAM__SERVERS__"}
				forceTcpToClusterDNS = "force_tcp"
				forceTcpToUpstreamDNS = "force_tcp"
			})

			Context("ConfigMap", func() {
				JustBeforeEach(func() {
					configMapData := map[string]string{
						"Corefile": `cluster.local:53 {
    errors
    cache {
            success 9984 30
            denial 9984 5
    }
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + healthAddress(values) + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(values.ClusterDNS, " ") + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
.:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + strings.Join(upstreamDNSAddress, " ") + ` {
            ` + forceTcpToUpstreamDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
					}
					configMapHash = utils.ComputeConfigMapChecksum(configMapData)[:8]
				})

				Context("ForceTcpToClusterDNS : true and ForceTcpToUpstreamDNS : true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(true),
							ForceTCPToUpstreamDNS:       ptr.To(true),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

				})

				Context("ForceTcpToClusterDNS : true and ForceTcpToUpstreamDNS : false", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(true),
							ForceTCPToUpstreamDNS:       ptr.To(false),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("ForceTcpToClusterDNS : false and ForceTcpToUpstreamDNS : true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(false),
							ForceTCPToUpstreamDNS:       ptr.To(true),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "force_tcp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("ForceTcpToClusterDNS : false and ForceTcpToUpstreamDNS : false", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(false),
							ForceTCPToUpstreamDNS:       ptr.To(false),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})

				Context("With IPv6:", func() {
					BeforeEach(func() {
						values.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        ptr.To(false),
							ForceTCPToUpstreamDNS:       ptr.To(false),
							DisableForwardToUpstreamDNS: ptr.To(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
						ipvsAddress = "fd30:1319:f1e:230b::1"
					})

					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
							values.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
						})

						It("should successfully deploy all resources", func() {
							expectedManifests = nil
							expectedManifests = append(expectedManifests, configMapYAMLFor())
							Expect(manifests).To(ContainElements(expectedManifests))
							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})

					Context("w/ VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = true
							values.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}

						})

						It("should successfully deploy all resources", func() {
							expectedManifests = append(expectedManifests, configMapYAMLFor(), vpaYAML)
							Expect(manifests).To(ContainElements(expectedManifests))

							managedResourceDaemonset, err := extractDaemonSet(manifests, kubernetes.ShootCodec.UniversalDeserializer())
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			scrapeConfig.ResourceVersion = ""
			scrapeConfigErrors.ResourceVersion = ""

			component = New(c, namespace, values)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfigErrors)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfigErrors), scrapeConfigErrors)).To(BeNotFoundError())
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
			component = New(c, namespace, values)

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

func healthAddress(values Values) string {
	if values.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv4 {
		return "169.254.20.10"
	} else {
		if len(values.DNSServers) > 0 {
			return "fd30:1319:f1e:230b::1 " + strings.Join(values.DNSServers, " ")
		}
		return "[fd30:1319:f1e:230b::1]"
	}
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

func bindIP(values Values) string {
	if values.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv4 {
		if len(values.DNSServers) > 0 {
			dnsAddress := selectIPAddress(values.DNSServers, false)
			return "169.254.20.10" + " " + dnsAddress
		}
		return "169.254.20.10"
	} else {
		if len(values.DNSServers) > 0 {
			dnsAddress := selectIPAddress(values.DNSServers, true)
			return "fd30:1319:f1e:230b::1" + " " + dnsAddress
		}
		return "fd30:1319:f1e:230b::1"
	}
}

func containerArg(values Values) string {
	if values.IPFamilies[0] == gardencorev1beta1.IPFamilyIPv4 {
		if len(values.DNSServers) > 0 {
			dnsAddress := selectIPAddress(values.DNSServers, false)
			return "169.254.20.10" + "," + dnsAddress
		} else {
			return "169.254.20.10"
		}
	} else {
		if len(values.DNSServers) > 0 {
			dnsAddress := selectIPAddress(values.DNSServers, true)
			return "fd30:1319:f1e:230b::1" + "," + dnsAddress
		} else {
			return "fd30:1319:f1e:230b::1"
		}
	}
}

func extractDaemonSet(manifests []string, decoder runtime.Decoder) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}
	for _, manifest := range manifests {
		if strings.Contains(manifest, "kind: DaemonSet") {
			_, _, err := decoder.Decode([]byte(manifest), nil, ds)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	return ds, nil
}
