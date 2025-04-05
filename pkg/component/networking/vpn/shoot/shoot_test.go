// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VPNShoot", func() {
	var (
		ctx                 = context.Background()
		managedResourceName = "shoot-core-vpn-shoot"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c        client.Client
		sm       secretsmanager.Interface
		vpnShoot component.DeployWaiter

		contain               func(...client.Object) types.GomegaMatcher
		manifests             []string
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		endPoint                        = "10.0.0.1"
		openVPNPort               int32 = 8132
		reversedVPNHeader               = "outbound|1194||vpn-seed-server.shoot--project--shoot-name.svc.cluster.local"
		reversedVPNHeaderTemplate       = "outbound|1194||vpn-seed-server-%d.shoot--project--shoot-name.svc.cluster.local"

		values = Values{
			Image: image,
			ReversedVPN: ReversedVPNValues{
				Endpoint:    endPoint,
				OpenVPNPort: openVPNPort,
				Header:      reversedVPNHeader,
				IPFamilies:  []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
			},
			SeedPodNetwork: "10.1.0.0/16",
			Network: NetworkValues{
				PodCIDRs:     []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}},
				ServiceCIDRs: []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}},
				NodeCIDRs:    []net.IPNet{{IP: net.ParseIP("10.0.2.0"), Mask: net.CIDRMask(24, 32)}},
			},
		}

		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-tunnel-probe-apiserver-proxy",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels: ptr.To(false),
				MetricsPath: ptr.To("/probe"),
				Params:      map[string][]string{"module": {"http_apiserver"}},
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       "Pod",
					APIServer:  ptr.To("https://kube-apiserver"),
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
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
						Replacement:  ptr.To("https://kube-apiserver:443/api/v1/namespaces/kube-system/pods/${1}/log?container=${2}&tailLines=1"),
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
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"__name__"},
					Action:       "keep",
					Regex:        `^(probe_http_status_code|probe_success)$`,
				}},
			},
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-tunnel-probe-apiserver-proxy",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "vpn.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "VPNShootNoPods",
							Expr:  intstr.FromString(`kube_deployment_status_replicas_available{deployment="vpn-shoot"} == 0`),
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
							Expr:  intstr.FromString(`kube_statefulset_status_replicas_ready{statefulset="vpn-shoot"} == 0`),
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
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		contain = NewManagedResourceContainsObjectsMatcher(c)
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

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-vpn", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-tlsauth", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpn-seed-server-tlsauth", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		var (
			networkPolicy = &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener.cloud--allow-vpn",
					Namespace: "kube-system",
					Annotations: map[string]string{
						"gardener.cloud/description": "Allows the VPN to communicate with shoot components and makes the VPN reachable from the seed.",
					},
				},
				Spec: networkingv1.NetworkPolicySpec{
					Ingress: []networkingv1.NetworkPolicyIngressRule{{}},
					Egress:  []networkingv1.NetworkPolicyEgressRule{{}},
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "vpn-shoot",
						},
					},
					PolicyTypes: []networkingv1.PolicyType{
						networkingv1.PolicyTypeEgress,
						networkingv1.PolicyTypeIngress,
					},
				},
			}

			networkPolicyFromSeed = &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener.cloud--allow-from-seed",
					Namespace: "kube-system",
					Annotations: map[string]string{
						"gardener.cloud/description": "Allows Ingress from the control plane to pods labeled with 'networking.gardener.cloud/from-seed=allowed'.",
					},
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"networking.gardener.cloud/from-seed": "allowed",
						},
					},
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app":                 "vpn-shoot",
											"gardener.cloud/role": "system-component",
											"origin":              "gardener",
											"type":                "tunnel",
										},
									},
								},
							},
						},
					},
					PolicyTypes: []networkingv1.PolicyType{
						networkingv1.PolicyTypeIngress,
					},
				},
			}

			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-shoot",
					Namespace: "kube-system",
					Labels: map[string]string{
						"app": "vpn-shoot",
					},
				},
				AutomountServiceAccountToken: ptr.To(false),
			}

			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-shoot",
					Namespace: "kube-system",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "vpn-shoot",
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    "vpn-shoot",
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
						},
					},
				},
			}

			vpaHA = &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-shoot",
					Namespace: "kube-system",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       "vpn-shoot",
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: func() *vpaautoscalingv1.UpdateMode {
							mode := vpaautoscalingv1.UpdateModeAuto
							return &mode
						}(),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    "vpn-shoot-s0",
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
							{
								ContainerName:    "vpn-shoot-s1",
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
							{
								ContainerName:    "vpn-shoot-s2",
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
							{
								ContainerName:    "tunnel-controller",
								ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("10Mi"),
								},
							},
						},
					},
				},
			}

			pdb = &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-shoot",
					Namespace: "kube-system",
					Labels: map[string]string{
						"app": "vpn-shoot",
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: &intstr.IntOrString{IntVal: 1},
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "vpn-shoot",
						},
					},
					UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
				},
			}

			containerFor = func(clients int, index *int, vpaEnabled, highAvailable bool) *corev1.Container {
				var (
					limits       corev1.ResourceList
					env          []corev1.EnvVar
					volumeMounts []corev1.VolumeMount
				)

				header := reversedVPNHeader
				if index != nil {
					header = fmt.Sprintf(reversedVPNHeaderTemplate, *index)
				}

				if !highAvailable {
					mountPath := "/srv/secrets/vpn-client"
					volumeMounts = []corev1.VolumeMount{
						{
							Name:      "vpn-shoot",
							MountPath: mountPath,
						},
					}
				} else {
					volumeMounts = nil
					for i := 0; i < clients; i++ {
						volumeMounts = append(volumeMounts, corev1.VolumeMount{
							Name:      fmt.Sprintf("vpn-shoot-%d", i),
							MountPath: fmt.Sprintf("/srv/secrets/vpn-client-%d", i),
						})
					}
				}
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      "vpn-shoot-tlsauth",
					MountPath: "/srv/secrets/tlsauth",
				})

				env = append(env,
					corev1.EnvVar{
						Name:  "IP_FAMILIES",
						Value: string(values.ReversedVPN.IPFamilies[0]),
					},
					corev1.EnvVar{
						Name:  "ENDPOINT",
						Value: endPoint,
					},
					corev1.EnvVar{
						Name:  "OPENVPN_PORT",
						Value: strconv.Itoa(int(openVPNPort)),
					},
					corev1.EnvVar{
						Name:  "REVERSED_VPN_HEADER",
						Value: header,
					},
					corev1.EnvVar{
						Name:  "IS_SHOOT_CLIENT",
						Value: "true",
					},
					corev1.EnvVar{
						Name:  "SEED_POD_NETWORK",
						Value: values.SeedPodNetwork,
					},
					corev1.EnvVar{
						Name:  "SHOOT_POD_NETWORKS",
						Value: values.Network.PodCIDRs[0].String(),
					},
					corev1.EnvVar{
						Name:  "SHOOT_SERVICE_NETWORKS",
						Value: values.Network.ServiceCIDRs[0].String(),
					},
					corev1.EnvVar{
						Name:  "SHOOT_NODE_NETWORKS",
						Value: values.Network.NodeCIDRs[0].String(),
					},
				)

				volumeMounts = append(volumeMounts,
					corev1.VolumeMount{
						Name:      "dev-net-tun",
						MountPath: "/dev/net/tun",
					},
				)

				if vpaEnabled {
					limits = corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					}
				} else {
					limits = corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("120Mi"),
					}
				}

				if highAvailable {
					env = append(env, []corev1.EnvVar{
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

				name := "vpn-shoot"
				if index != nil {
					name = fmt.Sprintf("vpn-shoot-s%d", *index)
				}
				container := &corev1.Container{
					Name:            name,
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env:             env,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: limits,
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged:               ptr.To(false),
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
					VolumeMounts: volumeMounts,
				}
				return container
			}

			volumesFor = func(secretNameClients []string, secretNameCA, secretNameTLSAuth string, highAvailable bool) []corev1.Volume {
				var volumes []corev1.Volume
				for i, secretName := range secretNameClients {
					name := "vpn-shoot"
					if highAvailable {
						name = fmt.Sprintf("vpn-shoot-%d", i)
					}
					volumes = append(volumes, corev1.Volume{
						Name: name,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: ptr.To[int32](0400),
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretNameCA,
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
												Name: secretName,
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
					})
				}
				volumes = append(volumes, corev1.Volume{
					Name: "vpn-shoot-tlsauth",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  secretNameTLSAuth,
							DefaultMode: ptr.To[int32](0400),
						},
					},
				})
				return volumes
			}

			templateForEx = func(servers int, secretNameClients []string, secretNameCA, secretNameTLSAuth string, vpaEnabled, highAvailable bool) *corev1.PodTemplateSpec {
				var (
					annotations = map[string]string{
						references.AnnotationKey(references.KindSecret, secretNameCA): secretNameCA,
					}

					initContainer = corev1.Container{
						Name:            "vpn-shoot-init",
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/vpn-client", "setup"},
						Env: []corev1.EnvVar{
							{
								Name:  "IP_FAMILIES",
								Value: string(values.ReversedVPN.IPFamilies[0]),
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

					volumes = volumesFor(secretNameClients, secretNameCA, secretNameTLSAuth, highAvailable)
				)

				for _, item := range secretNameClients {
					annotations[references.AnnotationKey(references.KindSecret, item)] = item
				}

				annotations[references.AnnotationKey(references.KindSecret, secretNameTLSAuth)] = secretNameTLSAuth

				hostPathCharDev := corev1.HostPathCharDev
				volumes = append(volumes,
					corev1.Volume{
						Name: "dev-net-tun",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev/net/tun",
								Type: &hostPathCharDev,
							},
						},
					},
				)

				obj := &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
						Labels: map[string]string{
							"app":                 "vpn-shoot",
							"gardener.cloud/role": "system-component",
							"origin":              "gardener",
							"type":                "tunnel",
						},
					},
					Spec: corev1.PodSpec{
						AutomountServiceAccountToken: ptr.To(false),
						ServiceAccountName:           "vpn-shoot",
						PriorityClassName:            "system-cluster-critical",
						DNSPolicy:                    corev1.DNSDefault,
						SecurityContext: &corev1.PodSecurityContext{
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Volumes: volumes,
					},
				}

				if !highAvailable {
					obj.Spec.Containers = append(obj.Spec.Containers, *containerFor(1, nil, vpaEnabled, highAvailable))
				} else {
					for i := 0; i < servers; i++ {
						obj.Spec.Containers = append(obj.Spec.Containers, *containerFor(len(secretNameClients), &i, vpaEnabled, highAvailable))
					}
					obj.Spec.Containers = append(obj.Spec.Containers, corev1.Container{
						Name:    "tunnel-controller",
						Image:   image,
						Command: []string{"/bin/tunnel-controller"},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               ptr.To(false),
							AllowPrivilegeEscalation: ptr.To(false),
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{"NET_ADMIN"},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					})
				}

				if highAvailable {
					initContainer.Env = append(initContainer.Env, []corev1.EnvVar{
						{
							Name:  "IS_HA",
							Value: "true",
						},
						{
							Name:  "HA_VPN_SERVERS",
							Value: "3",
						},
						{
							Name:  "HA_VPN_CLIENTS",
							Value: "2",
						},
					}...)
				}

				obj.Spec.InitContainers = []corev1.Container{initContainer}

				return obj
			}

			templateFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string) *corev1.PodTemplateSpec {
				return templateForEx(1, []string{secretNameClient}, secretNameCA, secretNameTLSAuth, values.VPAEnabled, false)
			}

			objectMetaForEx = func(secretNameClients []string, secretNameCA, secretNameTLSAuth string) *metav1.ObjectMeta {
				annotations := map[string]string{
					references.AnnotationKey(references.KindSecret, secretNameCA): secretNameCA,
				}
				for _, item := range secretNameClients {
					annotations[references.AnnotationKey(references.KindSecret, item)] = item
				}

				annotations[references.AnnotationKey(references.KindSecret, secretNameTLSAuth)] = secretNameTLSAuth

				return &metav1.ObjectMeta{
					Name:        "vpn-shoot",
					Namespace:   "kube-system",
					Annotations: annotations,
					Labels: map[string]string{
						"app":                 "vpn-shoot",
						"gardener.cloud/role": "system-component",
						"origin":              "gardener",
					},
				}
			}

			objectMetaFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string) *metav1.ObjectMeta {
				return objectMetaForEx([]string{secretNameClient}, secretNameCA, secretNameTLSAuth)
			}

			deploymentFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string) *appsv1.Deployment {
				var (
					intStrMax, intStrZero = intstr.FromString("100%"), intstr.FromString("0%")
				)

				return &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: *objectMetaFor(secretNameCA, secretNameClient, secretNameTLSAuth),
					Spec: appsv1.DeploymentSpec{
						RevisionHistoryLimit: ptr.To[int32](2),
						Replicas:             ptr.To[int32](1),
						Strategy: appsv1.DeploymentStrategy{
							Type: appsv1.RollingUpdateDeploymentStrategyType,
							RollingUpdate: &appsv1.RollingUpdateDeployment{
								MaxSurge:       &intStrMax,
								MaxUnavailable: &intStrZero,
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "vpn-shoot",
							},
						},
						Template: *templateFor(secretNameCA, secretNameClient, secretNameTLSAuth),
					},
				}
			}

			statefulSetFor = func(servers, replicas int, secretNameClients []string, secretNameCA, secretNameTLSAuth string) *appsv1.StatefulSet {
				return &appsv1.StatefulSet{
					ObjectMeta: *objectMetaForEx(secretNameClients, secretNameCA, secretNameTLSAuth),
					Spec: appsv1.StatefulSetSpec{
						PodManagementPolicy:  appsv1.ParallelPodManagement,
						RevisionHistoryLimit: ptr.To[int32](2),
						Replicas:             ptr.To(int32(replicas)),
						UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
							Type: appsv1.RollingUpdateStatefulSetStrategyType,
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "vpn-shoot",
							},
						},
						Template: *templateForEx(servers, secretNameClients, secretNameCA, secretNameTLSAuth, values.VPAEnabled, true),
					},
				}
			}
		)

		JustBeforeEach(func() {
			vpnShoot = New(c, namespace, sm, values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(vpnShoot.Deploy(ctx)).To(Succeed())
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
					SecretRefs:   []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
					KeepObjects:  ptr.To(false),
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

			actualPrometheusRule := &monitoringv1.PrometheusRule{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
			Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

			componenttest.PrometheusRule(actualPrometheusRule, "testdata/shoot-tunnel-probe-apiserver-proxy.prometheusrule.test.yaml")

			var err error
			manifests, err = test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			Expect(managedResource).To(contain(
				networkPolicy,
				networkPolicyFromSeed,
				serviceAccount,
			))
		})

		Context("IPv6", func() {
			BeforeEach(func() {
				values.VPAEnabled = false
				values.ReversedVPN.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
			})

			It("should successfully deploy all resources", func() {
				var (
					secretNameClient  = expectVPNShootSecret(manifests)
					secretNameCA      = expectCASecret(manifests)
					secretNameTLSAuth = expectTLSAuthSecret(manifests)
				)
				Expect(managedResource).To(contain(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth)))
			})
		})

		Context("VPNShoot with ReversedVPN enabled", func() {
			Context("w/o VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = false
				})

				It("should successfully deploy all resources", func() {
					var (
						secretNameClient  = expectVPNShootSecret(manifests)
						secretNameCA      = expectCASecret(manifests)
						secretNameTLSAuth = expectTLSAuthSecret(manifests)
					)

					Expect(managedResource).To(contain(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth)))
				})
			})

			Context("w/ VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					var (
						secretNameClient  = expectVPNShootSecret(manifests)
						secretNameCA      = expectCASecret(manifests)
						secretNameTLSAuth = expectTLSAuthSecret(manifests)
					)

					Expect(managedResource).To(contain(
						vpa,
						deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth),
					))
				})
			})

			Context("w/ VPA and high availability", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
					values.HighAvailabilityEnabled = true
					values.HighAvailabilityNumberOfSeedServers = 3
					values.HighAvailabilityNumberOfShootClients = 2
				})

				JustBeforeEach(func() {
					var (
						secretNameClient0 = expectVPNShootSecret(manifests, "-0")
						secretNameClient1 = expectVPNShootSecret(manifests, "-1")
						secretNameCA      = expectCASecret(manifests)
						secretNameTLSAuth = expectTLSAuthSecret(manifests)
					)

					vpaCopy := vpaHA.DeepCopy()
					if values.VPAUpdateDisabled {
						vpaCopy.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeOff)
					}

					Expect(managedResource).To(contain(
						vpaCopy,
						statefulSetFor(3, 2, []string{secretNameClient0, secretNameClient1}, secretNameCA, secretNameTLSAuth),
					))
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResource).To(contain(pdb))
				})

				Context("w/ VPA update mode set to off", func() {
					BeforeEach(func() {
						values.VPAUpdateDisabled = true
					})

					It("should successfully deploy all resources", func() {
						var (
							secretNameClient0 = expectVPNShootSecret(manifests, "-0")
							secretNameClient1 = expectVPNShootSecret(manifests, "-1")
							secretNameCA      = expectCASecret(manifests)
							secretNameTLSAuth = expectTLSAuthSecret(manifests)
						)

						vpaCopy := vpaHA.DeepCopy()
						vpaCopy.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeOff)

						Expect(managedResource).To(contain(
							vpaCopy,
							statefulSetFor(3, 2, []string{secretNameClient0, secretNameClient1}, secretNameCA, secretNameTLSAuth),
						))
					})
				})

				AfterEach(func() {
					values.HighAvailabilityEnabled = false
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			scrapeConfig.ResourceVersion = ""
			prometheusRule.ResourceVersion = ""

			vpnShoot = New(c, namespace, sm, Values{})
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())

			Expect(vpnShoot.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})

		It("should successfully destroy all resources", func() {
			scrapeConfig.ResourceVersion = ""
			prometheusRule.ResourceVersion = ""

			vpnShoot = New(c, namespace, sm, Values{
				HighAvailabilityEnabled:              true,
				HighAvailabilityNumberOfSeedServers:  2,
				HighAvailabilityNumberOfShootClients: 2,
			})
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, scrapeConfig)).To(Succeed())
			Expect(c.Create(ctx, prometheusRule)).To(Succeed())

			Expect(vpnShoot.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(scrapeConfig), scrapeConfig)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(prometheusRule), prometheusRule)).To(BeNotFoundError())
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
			vpnShoot = New(c, namespace, sm, Values{})

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
				Expect(vpnShoot.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
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
				Expect(vpnShoot.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
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
				Expect(vpnShoot.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(vpnShoot.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(vpnShoot.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func expectVPNShootSecret(manifests []string, haSuffix ...string) string {
	suffix := "client"

	if len(haSuffix) > 0 {
		suffix += haSuffix[0]
	}
	return expectSecret(manifests, suffix)
}

func expectCASecret(manifests []string) string {
	return expectSecret(manifests, "ca")
}

func expectTLSAuthSecret(manifests []string) string {
	return expectSecret(manifests, "tlsauth")
}

func expectSecret(manifests []string, suffix string) string {
	var secretManifest string

	for _, manifest := range manifests {
		if strings.Contains(manifest, "kind: Secret") && strings.Contains(manifest, "name: vpn-shoot-"+suffix) {
			secretManifest = manifest
			break
		}
	}

	secret := &corev1.Secret{}
	Expect(runtime.DecodeInto(newCodec(), []byte(secretManifest), secret)).To(Succeed())
	if secret.Immutable == nil {
		println("x")
	}
	Expect(secret.Immutable).To(PointTo(BeTrue()))
	Expect(secret.Data).NotTo(BeEmpty())
	Expect(secret.Labels).To(HaveKeyWithValue("resources.gardener.cloud/garbage-collectable-reference", "true"))

	return secret.Name
}

func newCodec() runtime.Codec {
	var groupVersions []schema.GroupVersion
	for k := range kubernetes.ShootScheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}
	return kubernetes.ShootCodec.CodecForVersions(kubernetes.ShootSerializer, kubernetes.ShootSerializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
}
