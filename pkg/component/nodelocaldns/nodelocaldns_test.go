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

package nodelocaldns_test

import (
	"context"
	"strconv"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/nodelocaldns"
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
		ctx = context.TODO()

		managedResourceName = "shoot-core-node-local-dns"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

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
		upstreamDNSAddress    = "__PILLAR__UPSTREAM__SERVERS__"
		forceTcpToClusterDNS  = "force_tcp"
		forceTcpToUpstreamDNS = "force_tcp"
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:             image,
			PSPDisabled:       true,
			KubernetesVersion: semver.MustParse("1.22.1"),
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
			podSecurityPolicyYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: runtime/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: runtime/default
  creationTimestamp: null
  labels:
    app: node-local-dns
  name: gardener.kube-system.node-local-dns
spec:
  allowedCapabilities:
  - NET_ADMIN
  allowedHostPaths:
  - pathPrefix: /run/xtables.lock
  fsGroup:
    rule: RunAsAny
  hostNetwork: true
  hostPorts:
  - max: 53
    min: 53
  - max: 9253
    min: 9253
  - max: 9353
    min: 9353
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - secret
  - hostPath
  - configMap
  - projected
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: node-local-dns
  name: gardener.cloud:psp:kube-system:node-local-dns
rules:
- apiGroups:
  - policy
  - extensions
  resourceNames:
  - gardener.kube-system.node-local-dns
  resources:
  - podsecuritypolicies
  verbs:
  - use
`
			roleBindingPSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app: node-local-dns
  name: gardener.cloud:psp:node-local-dns
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:kube-system:node-local-dns
subjects:
- kind: ServiceAccount
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
        forward . ` + values.ClusterDNS + ` {
                ` + forceTcpToClusterDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        health ` + ipvsAddress + `:` + strconv.Itoa(livenessProbePort) + `
        }
    in-addr.arpa:53 {
        errors
        cache 30
        reload
        loop
        bind ` + bindIP(values) + `
        forward . ` + values.ClusterDNS + ` {
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
        forward . ` + values.ClusterDNS + ` {
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
        forward . ` + upstreamDNSAddress + ` {
                ` + forceTcpToUpstreamDNS + `
        }
        prometheus :` + strconv.Itoa(prometheusPort) + `
        }
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
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
												corev1.ResourceMemory: resource.MustParse("100Mi"),
											},
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("25m"),
												corev1.ResourceMemory: resource.MustParse("25Mi"),
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
										SecurityContext: &corev1.SecurityContext{
											Capabilities: &corev1.Capabilities{
												Add: []corev1.Capability{"NET_ADMIN"},
											},
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
													Port: intstr.FromInt(livenessProbePort),
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
												Optional: pointer.Bool(true),
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
      maxAllowed:
        cpu: 100m
        memory: 200Mi
      minAllowed:
        memory: 20Mi
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
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__node-local-dns.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__kube-dns-upstream.yaml"])).To(Equal(serviceYAML))
		})

		Context("NodeLocalDNS with ipvsEnabled not enabled", func() {
			BeforeEach(func() {
				values.ClusterDNS = "__PILLAR__CLUSTER__DNS__"
				values.DNSServer = "1.2.3.4"
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
    forward . ` + values.ClusterDNS + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + ipvsAddress + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + upstreamDNSAddress + ` {
            ` + forceTcpToUpstreamDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
					}
					configMapHash = utils.ComputeConfigMapChecksum(configMapData)[:8]
					values.ShootAnnotations = map[string]string{}
				})

				Context("ForceTcpToClusterDNS : true and ForceTcpToUpstreamDNS : true", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(false),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(false),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "force_tcp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(false),
							ForceTCPToUpstreamDNS:       pointer.Bool(false),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(true),
						}
						values.VPAEnabled = true
						upstreamDNSAddress = values.ClusterDNS
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "force_tcp"
					})

					It("should succesfully deploy all resources", func() {
						Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
						managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
						Expect(err).ToNot(HaveOccurred())
						daemonset := daemonSetYAMLFor()
						utilruntime.Must(references.InjectAnnotations(daemonset))
						Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
					})
				})

				Context("Annotation ForceTcpToClusterDNS", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(true),
						}
						values.VPAEnabled = true
						upstreamDNSAddress = values.ClusterDNS
						values.ShootAnnotations = map[string]string{v1beta1constants.AnnotationNodeLocalDNSForceTcpToClusterDns: "false"}

						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "force_tcp"
					})

					It("should succesfully deploy all resources", func() {
						Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
						managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
						Expect(err).ToNot(HaveOccurred())
						daemonset := daemonSetYAMLFor()
						utilruntime.Must(references.InjectAnnotations(daemonset))
						Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
					})
				})
				Context("Annotation ForceTcpToClusterDNS", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(true),
						}
						values.VPAEnabled = true
						upstreamDNSAddress = values.ClusterDNS
						values.ShootAnnotations = map[string]string{v1beta1constants.AnnotationNodeLocalDNSForceTcpToUpstreamDns: "false"}

						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})

					It("should succesfully deploy all resources", func() {
						Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
						managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
						Expect(err).ToNot(HaveOccurred())
						daemonset := daemonSetYAMLFor()
						utilruntime.Must(references.InjectAnnotations(daemonset))
						Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
					})
				})
				Context("Annotation ForceTcpToClusterDNS", func() {
					BeforeEach(func() {
						values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(true),
						}
						values.VPAEnabled = true
						upstreamDNSAddress = values.ClusterDNS
						values.ShootAnnotations = map[string]string{v1beta1constants.AnnotationNodeLocalDNSForceTcpToClusterDns: "false", v1beta1constants.AnnotationNodeLocalDNSForceTcpToUpstreamDns: "false"}

						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})

					It("should succesfully deploy all resources", func() {
						Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
						managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
				values.ClusterDNS = "1.2.3.4"
				values.DNSServer = ""
				upstreamDNSAddress = "__PILLAR__UPSTREAM__SERVERS__"
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
    forward . ` + values.ClusterDNS + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + ipvsAddress + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + upstreamDNSAddress + ` {
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
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(true),
							ForceTCPToUpstreamDNS:       pointer.Bool(false),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "force_tcp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(false),
							ForceTCPToUpstreamDNS:       pointer.Bool(true),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "force_tcp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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
							ForceTCPToClusterDNS:        pointer.Bool(false),
							ForceTCPToUpstreamDNS:       pointer.Bool(false),
							DisableForwardToUpstreamDNS: pointer.Bool(false),
						}
						forceTcpToClusterDNS = "prefer_udp"
						forceTcpToUpstreamDNS = "prefer_udp"
					})
					Context("w/o VPA", func() {
						BeforeEach(func() {
							values.VPAEnabled = false
						})

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
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

						It("should succesfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
							Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
							managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
							Expect(err).ToNot(HaveOccurred())
							daemonset := daemonSetYAMLFor()
							utilruntime.Must(references.InjectAnnotations(daemonset))
							Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
						})
					})
				})
			})
		})
		Context("PodSecurityPolicy", func() {
			BeforeEach(func() {
				values.ClusterDNS = "1.2.3.4"
				values.DNSServer = ""
				values.Config = &gardencorev1beta1.NodeLocalDNS{Enabled: true,
					ForceTCPToClusterDNS:        pointer.Bool(true),
					ForceTCPToUpstreamDNS:       pointer.Bool(true),
					DisableForwardToUpstreamDNS: pointer.Bool(false),
				}
				values.VPAEnabled = true
				forceTcpToClusterDNS = "force_tcp"
				forceTcpToUpstreamDNS = "force_tcp"
			})

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
    forward . ` + values.ClusterDNS + ` {
            ` + forceTcpToClusterDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    health ` + ipvsAddress + `:` + strconv.Itoa(livenessProbePort) + `
    }
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind ` + bindIP(values) + `
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + values.ClusterDNS + ` {
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
    forward . ` + upstreamDNSAddress + ` {
            ` + forceTcpToUpstreamDNS + `
    }
    prometheus :` + strconv.Itoa(prometheusPort) + `
    }
`,
				}
				configMapHash = utils.ComputeConfigMapChecksum(configMapData)[:8]

				Expect(string(managedResourceSecret.Data["configmap__kube-system__node-local-dns-"+configMapHash+".yaml"])).To(Equal(configMapYAMLFor()))
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-local-dns.yaml"])).To(Equal(vpaYAML))
				managedResourceDaemonset, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(managedResourceSecret.Data["daemonset__kube-system__node-local-dns.yaml"], nil, &appsv1.DaemonSet{})
				Expect(err).ToNot(HaveOccurred())
				daemonset := daemonSetYAMLFor()
				utilruntime.Must(references.InjectAnnotations(daemonset))
				Expect(daemonset).To(DeepEqual(managedResourceDaemonset))
			})

			Context("PSP is not disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = false
				})

				It("should succesfully deploy all resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(8))
					Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_node-local-dns.yaml"])).To(Equal(clusterRoleYAML))
					Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_node-local-dns.yaml"])).To(Equal(roleBindingPSPYAML))
					Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.node-local-dns.yaml"])).To(Equal(podSecurityPolicyYAML))

				})
			})

			Context("PSP is disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = true
				})

				It("should succesfully deploy all resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(5))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, values)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
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

				expectedChecksum := "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9"
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				}
				Expect(c.Create(ctx, secret)).To(Succeed())
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{{Name: secret.Name}},
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
						SecretsDataChecksum: &expectedChecksum,
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

func bindIP(values Values) string {
	if values.DNSServer != "" {
		return "169.254.20.10 " + values.DNSServer
	}
	return "169.254.20.10"
}

func containerArg(values Values) string {
	if values.DNSServer != "" {
		return "169.254.20.10," + values.DNSServer
	}
	return "169.254.20.10"
}
