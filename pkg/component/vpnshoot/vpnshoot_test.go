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

package vpnshoot_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/vpnshoot"
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
		ctx                 = context.TODO()
		managedResourceName = "shoot-core-vpn-shoot"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c        client.Client
		sm       secretsmanager.Interface
		vpnShoot Interface

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
			},
			PSPDisabled: true,
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
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
			networkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows the VPN to communicate with shoot components
      and makes the VPN reachable from the seed.
  creationTimestamp: null
  name: gardener.cloud--allow-vpn
  namespace: kube-system
spec:
  egress:
  - {}
  ingress:
  - {}
  podSelector:
    matchLabels:
      app: vpn-shoot
  policyTypes:
  - Egress
  - Ingress
`
			networkPolicyFromSeedYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows Ingress from the control plane to pods labeled
      with 'networking.gardener.cloud/from-seed=allowed'.
  creationTimestamp: null
  name: gardener.cloud--allow-from-seed
  namespace: kube-system
spec:
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: vpn-shoot
          gardener.cloud/role: system-component
          origin: gardener
          type: tunnel
  podSelector:
    matchLabels:
      networking.gardener.cloud/from-seed: allowed
  policyTypes:
  - Ingress
`
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app: vpn-shoot
  name: vpn-shoot
  namespace: kube-system
`
			podSecurityPolicyYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: runtime/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: runtime/default
  creationTimestamp: null
  name: gardener.kube-system.vpn-shoot
spec:
  allowedCapabilities:
  - NET_ADMIN
  allowedHostPaths:
  - pathPrefix: /dev/net/tun
  fsGroup:
    rule: RunAsAny
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - secret
  - emptyDir
  - projected
  - hostPath
`
			clusterRolePSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:psp:kube-system:vpn-shoot
rules:
- apiGroups:
  - policy
  - extensions
  resourceNames:
  - gardener.kube-system.vpn-shoot
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
  name: gardener.cloud:psp:vpn-shoot
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:kube-system:vpn-shoot
subjects:
- kind: ServiceAccount
  name: vpn-shoot
  namespace: kube-system
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: vpn-shoot
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: vpn-shoot
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: vpn-shoot
  updatePolicy:
    updateMode: Auto
status: {}
`
			vpaHAYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: vpn-shoot
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: vpn-shoot-s0
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
    - containerName: vpn-shoot-s1
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
    - containerName: vpn-shoot-s2
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
  targetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: vpn-shoot
  updatePolicy:
    updateMode: Auto
status: {}
`
			containerFor = func(clients int, index *int, vpaEnabled, highAvailable bool) *corev1.Container {
				var (
					limits = corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("120Mi"),
					}

					env []corev1.EnvVar

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
						Name:  "DO_NOT_CONFIGURE_KERNEL_SETTINGS",
						Value: "true",
					},
					corev1.EnvVar{
						Name:  "IS_SHOOT_CLIENT",
						Value: "true",
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
				}

				if highAvailable {
					env = append(env, []corev1.EnvVar{
						{
							Name:  "VPN_SERVER_INDEX",
							Value: fmt.Sprintf("%d", *index),
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
				return &corev1.Container{
					Name:            name,
					Image:           image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env:             env,
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.Bool(false),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: limits,
					},
					VolumeMounts: volumeMounts,
				}
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
								DefaultMode: pointer.Int32(0400),
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
							DefaultMode: pointer.Int32(0400),
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

					reversedVPNInitContainers = []corev1.Container{
						{
							Name:            "vpn-shoot-init",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
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
								{
									Name:  "EXIT_AFTER_CONFIGURING_KERNEL_SETTINGS",
									Value: "true",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Bool(true),
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
						AutomountServiceAccountToken: pointer.Bool(false),
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
				}

				if highAvailable {
					reversedVPNInitContainers[0].Env = append(reversedVPNInitContainers[0].Env, []corev1.EnvVar{
						{
							Name:  "CONFIGURE_BONDING",
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
				obj.Spec.InitContainers = reversedVPNInitContainers

				return obj
			}

			templateFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string, vpaEnabled bool) *corev1.PodTemplateSpec {
				return templateForEx(1, []string{secretNameClient}, secretNameCA, secretNameTLSAuth, vpaEnabled, false)
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

			deploymentFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string, vpaEnabled bool) *appsv1.Deployment {
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
						RevisionHistoryLimit: pointer.Int32(2),
						Replicas:             pointer.Int32(1),
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
						Template: *templateFor(secretNameCA, secretNameClient, secretNameTLSAuth, vpaEnabled),
					},
				}
			}

			statefulSetFor = func(servers, replicas int, secretNameClients []string, secretNameCA, secretNameTLSAuth string, vpaEnabled bool) *appsv1.StatefulSet {
				return &appsv1.StatefulSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
					},
					ObjectMeta: *objectMetaForEx(secretNameClients, secretNameCA, secretNameTLSAuth),
					Spec: appsv1.StatefulSetSpec{
						PodManagementPolicy:  appsv1.ParallelPodManagement,
						RevisionHistoryLimit: pointer.Int32(2),
						Replicas:             pointer.Int32(int32(replicas)),
						UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
							Type: appsv1.RollingUpdateStatefulSetStrategyType,
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "vpn-shoot",
							},
						},
						Template: *templateForEx(servers, secretNameClients, secretNameCA, secretNameTLSAuth, vpaEnabled, true),
					},
				}
			}
		)

		JustBeforeEach(func() {
			vpnShoot = New(c, namespace, sm, values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(vpnShoot.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

			expectedMr := &resourcesv1alpha1.ManagedResource{
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
					SecretRefs:   []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
					KeepObjects:  pointer.Bool(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-vpn.yaml"])).To(Equal(networkPolicyYAML))
			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-from-seed.yaml"])).To(Equal(networkPolicyFromSeedYAML))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__vpn-shoot.yaml"])).To(Equal(serviceAccountYAML))
		})

		Context("VPNShoot with ReversedVPN enabled", func() {

			Context("w/o VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = false
				})

				It("should successfully deploy all resources", func() {
					var (
						secretNameClient  = expectVPNShootSecret(managedResourceSecret.Data)
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(DeepEqual(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth, values.VPAEnabled)))
				})
			})

			Context("w/ VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__vpn-shoot.yaml"])).To(Equal(vpaYAML))

					var (
						secretNameClient  = expectVPNShootSecret(managedResourceSecret.Data)
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(DeepEqual(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth, values.VPAEnabled)))
				})
			})

			Context("w/ VPA and high availability", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
					values.HighAvailabilityEnabled = true
					values.HighAvailabilityNumberOfSeedServers = 3
					values.HighAvailabilityNumberOfShootClients = 2
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__vpn-shoot.yaml"])).To(Equal(vpaHAYAML))

					var (
						secretNameClient0 = expectVPNShootSecret(managedResourceSecret.Data, "-0")
						secretNameClient1 = expectVPNShootSecret(managedResourceSecret.Data, "-1")
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					statefulSet := &appsv1.StatefulSet{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["statefulset__kube-system__vpn-shoot.yaml"], statefulSet)).To(Succeed())
					expected := statefulSetFor(3, 2, []string{secretNameClient0, secretNameClient1}, secretNameCA, secretNameTLSAuth, values.VPAEnabled)
					Expect(statefulSet).To(DeepEqual(expected))
				})

				AfterEach(func() {
					values.HighAvailabilityEnabled = false
				})
			})
		})

		Context("PodSecurityPolicy", func() {
			BeforeEach(func() {
				values.VPAEnabled = true
			})

			Context("PSP is not disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = false
				})
				JustBeforeEach(func() {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__vpn-shoot.yaml"])).To(Equal(vpaYAML))

					var (
						secretNameClient  = expectVPNShootSecret(managedResourceSecret.Data)
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(DeepEqual(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth, values.VPAEnabled)))
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(11))
					Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.vpn-shoot.yaml"])).To(Equal(podSecurityPolicyYAML))
					Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_vpn-shoot.yaml"])).To(Equal(clusterRolePSPYAML))
					Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_vpn-shoot.yaml"])).To(Equal(roleBindingPSPYAML))
				})
			})

			Context("PSP is disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = true
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResourceSecret.Data).To(HaveLen(8))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			vpnShoot = New(c, namespace, sm, Values{})
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(vpnShoot.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
		})

		It("should successfully destroy all resources", func() {
			vpnShoot = New(c, namespace, sm, Values{
				HighAvailabilityEnabled:              true,
				HighAvailabilityNumberOfSeedServers:  2,
				HighAvailabilityNumberOfShootClients: 2,
			})
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(vpnShoot.Destroy(ctx)).To(Succeed())

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

func expectVPNShootSecret(data map[string][]byte, haSuffix ...string) string {
	suffix := "client"

	if len(haSuffix) > 0 {
		suffix += haSuffix[0]
	}
	return expectSecret(data, suffix)
}

func expectCASecret(data map[string][]byte) string {
	return expectSecret(data, "ca")
}

func expectTLSAuthSecret(data map[string][]byte) string {
	return expectSecret(data, "tlsauth")
}

func expectSecret(data map[string][]byte, suffix string) string {
	var secretName string

	for key := range data {
		if strings.HasPrefix(key, "secret__kube-system__vpn-shoot-"+suffix) {
			secretName = strings.TrimSuffix(strings.TrimPrefix(key, "secret__kube-system__"), ".yaml")
			break
		}
	}

	secret := &corev1.Secret{}
	Expect(runtime.DecodeInto(newCodec(), data["secret__kube-system__"+secretName+".yaml"], secret)).To(Succeed())
	if secret.Immutable == nil {
		println("x")
	}
	Expect(secret.Immutable).To(PointTo(BeTrue()))
	Expect(secret.Data).NotTo(BeEmpty())
	Expect(secret.Labels).To(HaveKeyWithValue("resources.gardener.cloud/garbage-collectable-reference", "true"))

	return secretName
}

func newCodec() runtime.Codec {
	var groupVersions []schema.GroupVersion
	for k := range kubernetes.ShootScheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}
	return kubernetes.ShootCodec.CodecForVersions(kubernetes.ShootSerializer, kubernetes.ShootSerializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
}
