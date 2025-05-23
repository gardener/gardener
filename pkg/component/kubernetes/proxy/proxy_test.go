// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package proxy_test

import (
	"context"
	"fmt"
	"net"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeProxy", func() {
	var (
		ctx = context.Background()

		namespace       = "some-namespace"
		kubeconfig      = []byte("some-kubeconfig")
		podNetworkCIDRs = []net.IPNet{{IP: net.ParseIP("4.5.6.7"), Mask: net.CIDRMask(8, 32)}, {IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(64, 128)}}
		imageAlpine     = "some-alpine:image"

		c         client.Client
		component Interface
		values    Values

		consistOf                    func(...client.Object) types.GomegaMatcher
		contain                      func(...client.Object) types.GomegaMatcher
		managedResourceCentral       *resourcesv1alpha1.ManagedResource
		managedResourceSecretCentral *corev1.Secret

		managedResourceForPool = func(pool WorkerPool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-proxy-" + pool.Name + "-v" + pool.KubernetesVersion.String(),
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": pool.KubernetesVersion.String(),
					},
				},
			}
		}
		managedResourceSecretForPool = func(pool WorkerPool) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceForPool(pool).Name,
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": pool.KubernetesVersion.String(),
					},
				},
			}
		}

		managedResourceForPoolForMajorMinorVersionOnly = func(pool WorkerPool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-proxy-" + pool.Name + "-v" + fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					},
				},
			}
		}
		managedResourceSecretForPoolForMajorMinorVersionOnly = func(pool WorkerPool) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					},
				},
			}
		}

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-kube-proxy",
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
						Replacement: ptr.To("kube-proxy"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "type",
						Replacement: ptr.To("shoot"),
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
						Action:       "keep",
						Regex:        "kube-proxy;metrics",
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
					Regex:        `^(kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_count|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_sync_proxy_rules_duration_seconds_sum)$`,
				}},
			},
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-kube-proxy",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "kube-proxy.rules",
					Rules: []monitoringv1.Rule{
						{
							Record: "kubeproxy_network_latency:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.99, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.99"},
						},
						{
							Record: "kubeproxy_network_latency:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.9"},
						},
						{
							Record: "kubeproxy_network_latency:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(kubeproxy_network_programming_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.5"},
						},
						{
							Record: "kubeproxy_sync_proxy:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.99, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.99"},
						},
						{
							Record: "kubeproxy_sync_proxy:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.9"},
						},
						{
							Record: "kubeproxy_sync_proxy:quantile",
							Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(kubeproxy_sync_proxy_rules_duration_seconds_bucket[10m])) by (le))`),
							Labels: map[string]string{"quantile": "0.5"},
						},
					},
				}},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			IPVSEnabled: true,
			FeatureGates: map[string]bool{
				"Foo": true,
				"Bar": false,
			},
			ImageAlpine: imageAlpine,
			Kubeconfig:  kubeconfig,
			VPAEnabled:  false,
			WorkerPools: []WorkerPool{
				{Name: "pool1", KubernetesVersion: semver.MustParse("1.26.4"), Image: "some-image:some-tag1"},
				{Name: "pool2", KubernetesVersion: semver.MustParse("1.29.0"), Image: "some-image:some-tag2"},
			},
		}
		component = New(c, namespace, values)

		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
		contain = NewManagedResourceContainsObjectsMatcher(c)
		managedResourceCentral = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-proxy",
				Namespace: namespace,
				Labels:    map[string]string{"component": "kube-proxy"},
			},
		}
		managedResourceSecretCentral = &corev1.Secret{}
	})

	Describe("#Deploy", func() {
		var (
			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-proxy",
					Namespace: "kube-system",
				},
				AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
			}

			clusterRoleBinding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:target:node-proxier",
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "system:node-proxier",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "kube-proxy",
						Namespace: "kube-system",
					},
				},
			}

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-proxy",
					Namespace: "kube-system",
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "proxy",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "None",
					Ports: []corev1.ServicePort{
						{
							Name:       "metrics",
							Port:       10249,
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.IntOrString{IntVal: 0},
						},
					},
					Selector: map[string]string{
						"app":  "kubernetes",
						"role": "proxy",
					},
					Type: corev1.ServiceTypeClusterIP,
				},
			}

			secretName = "kube-proxy-e3a80e6d"

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"resources.gardener.cloud/garbage-collectable-reference": "true",
					},
				},
				Data: map[string][]byte{
					"kubeconfig": kubeconfig,
				},
				Immutable: ptr.To(true),
				Type:      corev1.SecretTypeOpaque,
			}

			configMapNameFor = func(ipvsEnabled bool) string {
				if !ipvsEnabled {
					return "kube-proxy-config-c3039bb4"
				}
				return "kube-proxy-config-c09a0894"
			}

			configMapFor = func(ipvsEnabled bool) *corev1.ConfigMap {
				out := `apiVersion: kubeproxy.config.k8s.io/v1alpha1
bindAddress: ""
bindAddressHardFail: false
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: /var/lib/kube-proxy-kubeconfig/kubeconfig
  qps: 0`
				if ipvsEnabled {
					out += `
clusterCIDR: ""`
				} else {
					out += `
clusterCIDR: ` + podNetworkCIDRs[0].String() + `,` + podNetworkCIDRs[1].String()
				}
				out += `
configSyncPeriod: 0s
conntrack:
  maxPerCore: 524288
  min: null
  tcpBeLiberal: false
  tcpCloseWaitTimeout: null
  tcpEstablishedTimeout: null
  udpStreamTimeout: 0s
  udpTimeout: 0s
detectLocal:
  bridgeInterface: ""
  interfaceNamePrefix: ""
detectLocalMode: ""
enableProfiling: false
featureGates:
  Bar: false
  Foo: true
healthzBindAddress: ""
hostnameOverride: ""
iptables:
  localhostNodePorts: null
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
ipvs:
  excludeCIDRs: null
  minSyncPeriod: 0s
  scheduler: ""
  strictARP: false
  syncPeriod: 0s
  tcpFinTimeout: 0s
  tcpTimeout: 0s
  udpTimeout: 0s
kind: KubeProxyConfiguration
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
    text:
      infoBufferSize: "0"
  verbosity: 0
metricsBindAddress: 0.0.0.0:10249`
				if ipvsEnabled {
					out += `
mode: ipvs`
				} else {
					out += `
mode: iptables`
				}
				out += `
nftables:
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
nodePortAddresses: null
oomScoreAdj: null
portRange: ""
showHiddenMetricsForVersion: ""
winkernel:
  enableDSR: false
  forwardHealthCheckVip: false
  networkName: ""
  rootHnsEndpointName: ""
  sourceVip: ""
`
				return &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                 "kubernetes",
							"gardener.cloud/role": "system-component",
							"resources.gardener.cloud/garbage-collectable-reference": "true",
							"role":   "proxy",
							"origin": "gardener",
						},
						Name:      configMapNameFor(ipvsEnabled),
						Namespace: "kube-system",
					},
					Immutable: ptr.To(true),
					Data: map[string]string{
						"config.yaml": out,
					},
				}
			}

			configMapConntrackFixScriptName = "kube-proxy-conntrack-fix-script-ebff3d39"

			configMapConntrackFixScript = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapConntrackFixScriptName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app":                 "kubernetes",
						"gardener.cloud/role": "system-component",
						"origin":              "gardener",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"role": "proxy",
					},
				},
				Immutable: ptr.To(true),
				Data: map[string]string{
					"conntrack_fix.sh": `#!/bin/sh -e
trap "kill -s INT 1" TERM
sleep 120 & wait
date
# conntrack example:
# tcp      6 113 SYN_SENT src=21.73.193.93 dst=21.71.0.65 sport=1413 dport=443 \
#   [UNREPLIED] src=21.71.0.65 dst=21.73.193.93 sport=443 dport=1413 mark=0 use=1
eval "$(
  conntrack -L -p tcp --state SYN_SENT \
  | sed 's/=/ /g'                      \
  | awk '$6 !~ /^10\./ &&
         $8 !~ /^10\./ &&
         $6  == $17    &&
         $8  == $15    &&
         $10 == $21    &&
         $12 == $19 {
           printf "conntrack -D -p tcp -s %s --sport %s -d %s --dport %s;\n",
                                          $6,        $10,  $8,        $12}'
)"
while true; do
  date
  sleep 3600 & wait
done
`,
				},
			}

			configMapCleanupScriptName = "kube-proxy-cleanup-script-a4263ada"
			configMapCleanupScript     = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapCleanupScriptName,
					Namespace: "kube-system",
					Labels: map[string]string{
						"app":                 "kubernetes",
						"gardener.cloud/role": "system-component",
						"origin":              "gardener",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"role": "proxy",
					},
				},
				Immutable: ptr.To(true),
				Data: map[string]string{
					"cleanup.sh": `#!/bin/sh -e
OLD_KUBE_PROXY_MODE="$(cat "$1")"
if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
  echo "${KUBE_PROXY_MODE}" >"$1"
  echo "Nothing to cleanup - the mode didn't change."
  exit 0
fi

/usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
echo "${KUBE_PROXY_MODE}" >"$1"
`,
				},
			}

			daemonSetNameFor = func(pool WorkerPool) string {
				return "kube-proxy-" + pool.Name + "-v" + pool.KubernetesVersion.String()
			}
			daemonSetFor = func(pool WorkerPool, ipvsEnabled, vpaEnabled, k8sGreaterEqual129, k8sGreaterEqual128 bool) *appsv1.DaemonSet {
				referenceAnnotations := func() map[string]string {
					if ipvsEnabled {
						return map[string]string{
							references.AnnotationKey(references.KindConfigMap, configMapCleanupScriptName):      configMapCleanupScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapConntrackFixScriptName): configMapConntrackFixScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapNameFor(ipvsEnabled)):   configMapNameFor(ipvsEnabled),
							references.AnnotationKey(references.KindSecret, secretName):                         secretName,
						}
					}
					return map[string]string{
						references.AnnotationKey(references.KindConfigMap, configMapCleanupScriptName):      configMapCleanupScriptName,
						references.AnnotationKey(references.KindConfigMap, configMapNameFor(ipvsEnabled)):   configMapNameFor(ipvsEnabled),
						references.AnnotationKey(references.KindConfigMap, configMapConntrackFixScriptName): configMapConntrackFixScriptName,
						references.AnnotationKey(references.KindSecret, secretName):                         secretName,
					}
				}

				kubeProxyMode := func() string {
					if ipvsEnabled {
						return "ipvs"
					}
					return "iptables"
				}

				ds := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:        daemonSetNameFor(pool),
						Namespace:   "kube-system",
						Annotations: referenceAnnotations(),
						Labels: map[string]string{
							"gardener.cloud/role":                    "system-component",
							"node.gardener.cloud/critical-component": "true",
							"origin":                                 "gardener",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						RevisionHistoryLimit: ptr.To[int32](2),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":     "kubernetes",
								"pool":    pool.Name,
								"role":    "proxy",
								"version": pool.KubernetesVersion.String(),
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: referenceAnnotations(),
								Labels: map[string]string{
									"app":                                    "kubernetes",
									"gardener.cloud/role":                    "system-component",
									"node.gardener.cloud/critical-component": "true",
									"origin":                                 "gardener",
									"pool":                                   pool.Name,
									"role":                                   "proxy",
									"version":                                pool.KubernetesVersion.String(),
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Command:         []string{"/usr/local/bin/kube-proxy", "--config=/var/lib/kube-proxy-config/config.yaml", "--v=2"},
										Image:           pool.Image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Name:            "kube-proxy",
										Ports: []corev1.ContainerPort{
											{
												ContainerPort: 10249,
												HostPort:      10249,
												Name:          "metrics",
												Protocol:      corev1.ProtocolTCP,
											},
										},
										ReadinessProbe: &corev1.Probe{
											ProbeHandler: corev1.ProbeHandler{
												HTTPGet: &corev1.HTTPGetAction{
													Path:   "/healthz",
													Port:   intstr.FromInt32(10256),
													Scheme: corev1.URISchemeHTTP,
												},
											},
											InitialDelaySeconds: 15,
											TimeoutSeconds:      15,
											SuccessThreshold:    1,
											FailureThreshold:    2,
										},
										Resources: corev1.ResourceRequirements{
											Requests: map[corev1.ResourceName]resource.Quantity{
												corev1.ResourceCPU:    resource.MustParse("20m"),
												corev1.ResourceMemory: resource.MustParse("64Mi"),
											},
										},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: ptr.To(false),
										},
										VolumeMounts: []corev1.VolumeMount{
											{MountPath: "/var/lib/kube-proxy-kubeconfig", Name: "kubeconfig"},
											{MountPath: "/var/lib/kube-proxy-config", Name: "kube-proxy-config"},
											{MountPath: "/etc/ssl/certs", Name: "ssl-certs-hosts", ReadOnly: true},
											{MountPath: "/lib/modules", Name: "kernel-modules"},
											{MountPath: "/run/xtables.lock", Name: "xtables-lock"},
										},
									},
									{
										Command:         []string{"/bin/sh", "/script/conntrack_fix.sh"},
										Image:           imageAlpine,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Name:            "conntrack-fix",
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: ptr.To(false),
											Capabilities: &corev1.Capabilities{
												Add: []corev1.Capability{"NET_ADMIN"},
											},
										},
										VolumeMounts: []corev1.VolumeMount{
											{MountPath: "/script", Name: "conntrack-fix-script"},
										},
									},
								},
								HostNetwork: true,
								InitContainers: []corev1.Container{
									{
										Command: []string{"sh", "-c", "/script/cleanup.sh /var/lib/kube-proxy/mode"},
										Env: []corev1.EnvVar{
											{Name: "KUBE_PROXY_MODE", Value: kubeProxyMode()},
										},
										Image:           pool.Image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Name:            "cleanup",
										SecurityContext: &corev1.SecurityContext{
											Privileged: ptr.To(true),
										},
										VolumeMounts: []corev1.VolumeMount{
											{MountPath: "/script", Name: "kube-proxy-cleanup-script"},
											{MountPath: "/lib/modules", Name: "kernel-modules"},
											{MountPath: "/var/lib/kube-proxy", Name: "kube-proxy-dir"},
											{MountPath: "/var/lib/kube-proxy/mode", Name: "kube-proxy-mode"},
											{MountPath: "/var/lib/kube-proxy-kubeconfig", Name: "kubeconfig"},
											{MountPath: "/var/lib/kube-proxy-config", Name: "kube-proxy-config"},
										},
									},
								},
								NodeSelector: map[string]string{
									"worker.gardener.cloud/kubernetes-version": pool.KubernetesVersion.String(),
									"worker.gardener.cloud/pool":               pool.Name,
								},
								PriorityClassName: "system-node-critical",
								SecurityContext: &corev1.PodSecurityContext{
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
								},
								ServiceAccountName: "kube-proxy",
								Tolerations: []corev1.Toleration{
									{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
									{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
								},
								Volumes: []corev1.Volume{
									{
										Name: "kubeconfig",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: secretName,
											},
										},
									},
									{
										Name: "kube-proxy-config",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapNameFor(ipvsEnabled),
												},
											},
										},
									},
									{
										Name: "ssl-certs-hosts",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: "/usr/share/ca-certificates",
											},
										},
									},
									{
										Name: "kernel-modules",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: "/lib/modules",
											},
										},
									},
									{
										Name: "kube-proxy-cleanup-script",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapCleanupScriptName,
												},
												DefaultMode: ptr.To[int32](511),
											},
										},
									},
									{
										Name: "kube-proxy-dir",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: "/var/lib/kube-proxy",
												Type: ptr.To(corev1.HostPathDirectoryOrCreate),
											},
										},
									},
									{
										Name: "kube-proxy-mode",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: "/var/lib/kube-proxy/mode",
												Type: ptr.To(corev1.HostPathFileOrCreate),
											},
										},
									},
									{
										Name: "conntrack-fix-script",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapConntrackFixScriptName,
												},
											},
										},
									},
									{
										Name: "xtables-lock",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: "/run/xtables.lock",
												Type: ptr.To(corev1.HostPathFileOrCreate),
											},
										},
									},
								},
							},
						},
						UpdateStrategy: appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType},
					},
				}

				if k8sGreaterEqual128 {
					ds.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path = "/livez"
				}

				if k8sGreaterEqual129 {
					ds.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "SYS_RESOURCE"},
						},
					}

					ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, corev1.Container{
						Command:         []string{"/usr/local/bin/kube-proxy", "--config=/var/lib/kube-proxy-config/config.yaml", "--v=2", "--init-only"},
						Image:           pool.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Name:            "kube-proxy-init",
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: ptr.To(true),
						},
						VolumeMounts: []corev1.VolumeMount{
							{MountPath: "/var/lib/kube-proxy-kubeconfig", Name: "kubeconfig"},
							{MountPath: "/var/lib/kube-proxy-config", Name: "kube-proxy-config"},
							{MountPath: "/etc/ssl/certs", Name: "ssl-certs-hosts", ReadOnly: true},
							{MountPath: "/lib/modules", Name: "kernel-modules"},
							{MountPath: "/run/xtables.lock", Name: "xtables-lock"},
						},
					})

					if vpaEnabled {
						ds.Spec.Template.Spec.InitContainers[1].Resources.Limits = map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						}
					}
				} else {
					ds.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
						Privileged: ptr.To(true),
					}
				}

				if vpaEnabled {
					ds.Spec.Template.Spec.Containers[0].Resources.Limits = map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					}
				}

				return ds
			}

			vpaNameFor = func(pool WorkerPool) string {
				return fmt.Sprintf("kube-proxy-%s-v%d.%d", pool.Name, pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor())
			}
			vpaFor = func(pool WorkerPool) *vpaautoscalingv1.VerticalPodAutoscaler {
				return &vpaautoscalingv1.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      vpaNameFor(pool),
						Namespace: "kube-system",
					},
					Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
								{
									ContainerName:    "kube-proxy",
									ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("10G"),
									},
								},
								{
									ContainerName: "conntrack-fix",
									Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
								},
							},
						},
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "DaemonSet",
							Name:       daemonSetNameFor(pool),
						},
						UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
							UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
						},
					},
				}
			}
		)

		It("should successfully deploy all resources when IPVS is enabled", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(BeNotFoundError())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			}

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceCentral.Name,
					Namespace:       managedResourceCentral.Namespace,
					ResourceVersion: "1",
					Labels: map[string]string{
						"origin":    "gardener",
						"component": "kube-proxy",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(false),
				},
			}

			Expect(managedResourceCentral).To(consistOf(
				serviceAccount,
				clusterRoleBinding,
				service,
				secret,
				configMapFor(values.IPVSEnabled),
				configMapConntrackFixScript,
				configMapCleanupScript,
			))

			managedResourceSecretCentral = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceCentral.Spec.SecretRefs[0].Name, Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(Succeed())
			Expect(managedResourceSecretCentral.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecretCentral.Labels).To(Equal(map[string]string{
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"component": "kube-proxy",
				"origin":    "gardener",
			}))
			Expect(managedResourceSecretCentral.Immutable).To(Equal(ptr.To(true)))
			expectedMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecretCentral.Name}}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResourceCentral).To(DeepEqual(expectedMr))

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedPoolMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": pool.KubernetesVersion.String(),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  ptr.To(false),
					},
				}

				Expect(managedResource).To(consistOf(daemonSetFor(pool, values.IPVSEnabled, values.VPAEnabled,
					versionutils.ConstraintK8sGreaterEqual129.Check(pool.KubernetesVersion), versionutils.ConstraintK8sGreaterEqual128.Check(pool.KubernetesVersion))))
				managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResource.Spec.SecretRefs[0].Name, Namespace: namespace}}
				expectedPoolMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}}
				utilruntime.Must(references.InjectAnnotations(expectedPoolMr))
				Expect(managedResource).To(DeepEqual(expectedPoolMr))

				actualScrapeConfig := &monitoringv1alpha1.ScrapeConfig{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), actualScrapeConfig)).To(Succeed())
				Expect(actualScrapeConfig).To(DeepEqual(scrapeConfig))

				actualPrometheusRule := &monitoringv1.PrometheusRule{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
				Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			}
		})

		It("should successfully deploy the expected config when IPVS is disabled", func() {
			values.IPVSEnabled = false
			values.PodNetworkCIDRs = podNetworkCIDRs
			component = New(c, namespace, values)

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
			expectedMR := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceCentral.Name,
					Namespace:       managedResourceCentral.Namespace,
					ResourceVersion: "1",
					Labels: map[string]string{
						"origin":    "gardener",
						"component": "kube-proxy",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(false),
				},
			}
			Expect(managedResourceCentral).To(contain(configMapFor(values.IPVSEnabled)))

			managedResourceSecretCentral = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceCentral.Spec.SecretRefs[0].Name,
				Namespace: namespace,
			}}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(Succeed())
			Expect(managedResourceSecretCentral.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecretCentral.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecretCentral.Labels).To(Equal(map[string]string{
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"component": "kube-proxy",
				"origin":    "gardener",
			}))
			expectedMR.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecretCentral.Name}}
			utilruntime.Must(references.InjectAnnotations(expectedMR))
			Expect(managedResourceCentral).To(DeepEqual(expectedMR))

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				poolExpectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": pool.KubernetesVersion.String(),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  ptr.To(false),
					},
				}

				managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: namespace,
				}}
				Expect(managedResource).To(consistOf(daemonSetFor(pool, values.IPVSEnabled, values.VPAEnabled,
					versionutils.ConstraintK8sGreaterEqual129.Check(pool.KubernetesVersion), versionutils.ConstraintK8sGreaterEqual128.Check(pool.KubernetesVersion))))

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Labels).To(Equal(map[string]string{
					"component":          "kube-proxy",
					"kubernetes-version": pool.KubernetesVersion.String(),
					"origin":             "gardener",
					"pool-name":          pool.Name,
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"role": "pool",
				}))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				poolExpectedMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}}
				utilruntime.Must(references.InjectAnnotations(poolExpectedMr))
				Expect(managedResource).To(DeepEqual(poolExpectedMr))
			}
		})

		It("should successfully deploy the expected resources when VPA is enabled", func() {
			values.VPAEnabled = true
			component = New(c, namespace, values)

			Expect(component.Deploy(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				// assertions for resources specific to the full Kubernetes version
				managedResource := managedResourceForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: namespace,
				}}
				Expect(managedResource).To(consistOf(daemonSetFor(pool, values.IPVSEnabled, values.VPAEnabled,
					versionutils.ConstraintK8sGreaterEqual129.Check(pool.KubernetesVersion), versionutils.ConstraintK8sGreaterEqual128.Check(pool.KubernetesVersion))))

				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": pool.KubernetesVersion.String(),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
						KeepObjects:  ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))

				Expect(managedResource).To(consistOf(daemonSetFor(pool, values.IPVSEnabled, values.VPAEnabled,
					versionutils.ConstraintK8sGreaterEqual129.Check(pool.KubernetesVersion), versionutils.ConstraintK8sGreaterEqual128.Check(pool.KubernetesVersion))))

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				// assertions for resources specific to the major/minor parts only of the Kubernetes version
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceForMajorMinorVersionOnly), managedResourceForMajorMinorVersionOnly)).To(Succeed())
				expectedMrForMajorMinorVersionOnly := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceForMajorMinorVersionOnly.Name,
						Namespace:       managedResourceForMajorMinorVersionOnly.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResourceForMajorMinorVersionOnly.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMrForMajorMinorVersionOnly))
				Expect(managedResourceForMajorMinorVersionOnly).To(Equal(expectedMrForMajorMinorVersionOnly))
				managedResourceSecretForMajorMinorVersionOnly.Name = managedResourceForMajorMinorVersionOnly.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretForMajorMinorVersionOnly), managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
				Expect(managedResourceSecretForMajorMinorVersionOnly.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretForMajorMinorVersionOnly.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretForMajorMinorVersionOnly.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResourceForMajorMinorVersionOnly).To(consistOf(vpaFor(pool)))
				Expect(managedResource).To(DeepEqual(expectedMr))
			}
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources despite undesired managed resources", func() {
			Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

			undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
			undesiredManagedResource := managedResourceForPool(undesiredPool)
			undesiredManagedResourceSecret := managedResourceSecretForPool(undesiredPool)

			Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecret)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			}

			scrapeConfig.ResourceVersion = ""
			prometheusRule.ResourceVersion = ""
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResource), undesiredManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecret), undesiredManagedResourceSecret)).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should successfully delete all stale resources", func() {
			Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

			undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
			undesiredManagedResource := managedResourceForPool(undesiredPool)
			undesiredManagedResourceSecret := managedResourceSecretForPool(undesiredPool)
			undesiredManagedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(undesiredPool)
			undesiredManagedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(undesiredPool)

			Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecretForMajorMinorVersionOnly)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
				Expect(c.Create(ctx, managedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
			}

			Expect(component.DeleteStaleResources(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceForMajorMinorVersionOnly), managedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretForMajorMinorVersionOnly), managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
			}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResource), undesiredManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecret), undesiredManagedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceForMajorMinorVersionOnly), undesiredManagedResourceForMajorMinorVersionOnly)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecretForMajorMinorVersionOnly), undesiredManagedResourceSecretForMajorMinorVersionOnly)).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 2}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the central ManagedResource doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because a pool-specific ManagedResource doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: unhealthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because a pool-specific ManagedResource for major/minor version only doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: unhealthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})

			It("should successfully wait for the managed resource to become healthy despite undesired managed resource unhealthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPool(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})

			It("should successfully wait for the managed resource to become healthy despite undesired managed resource for major/minor version only unhealthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPool(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPoolForMajorMinorVersionOnly(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out because of central resource", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
					Expect(c.Delete(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the managed resource deletion times out because of pool-specific resource", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
				Expect(c.Delete(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for the deletion", func() {
				for _, pool := range values.WorkerPools {
					managedResource := managedResourceForPool(pool)
					Expect(c.Create(ctx, managedResource)).To(Succeed())
					Expect(c.Delete(ctx, managedResource)).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite undesired still existing managed resources", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
				Expect(c.Delete(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)
				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())

				for _, pool := range values.WorkerPools {
					managedResource := managedResourceForPool(pool)

					Expect(c.Create(ctx, managedResource)).To(Succeed())
					Expect(c.Delete(ctx, managedResource)).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanupStaleResources", func() {
			It("should succeed when there is nothing to wait for", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should fail when the wait for the managed resource deletion times out", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, managedResourceForPool(undesiredPool))).To(Succeed())

				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the managed resource for major/minor version only deletion times out", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, managedResourceForPoolForMajorMinorVersionOnly(undesiredPool))).To(Succeed())

				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for the deletion", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)
				undesiredManagedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(c.Create(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite desired existing managed resources", func() {
				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite desired existing managed resources for major/minor version only", func() {
				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPoolForMajorMinorVersionOnly(pool))).To(Succeed())
				}

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})
		})
	})
})

var (
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
)
