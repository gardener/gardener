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

package vpnshoot_test

import (
	"context"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnshoot"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

		serviceNetwork = "10.0.0.0/24"
		podNetwork     = "192.168.0.0/16"
		nodeNetwork    = "172.16.0.0/20"

		endPoint                = "10.0.0.1"
		openVPNPort       int32 = 8132
		reversedVPNHeader       = "outbound|1194||vpn-seed-server.shoot--project--shoot-name.svc.cluster.local"

		secretNameDH     = "vpn-shoot-dh"
		secretChecksumDH = "5678"
		secretDataDH     = map[string][]byte{"foo": []byte("dash")}
		secretNameDHTest = "vpn-shoot-dh-" + utils.ComputeSecretChecksum(secretDataDH)[:8]

		secretNameTLSAuthLegacyVPN = "vpn-shoot-tlsauth-03c727cf"

		secrets = Secrets{}

		values = Values{
			Image: image,
			Network: NetworkValues{
				ServiceCIDR: serviceNetwork,
				PodCIDR:     podNetwork,
				NodeCIDR:    nodeNetwork,
			},
			ReversedVPN: ReversedVPNValues{
				Endpoint:    endPoint,
				OpenVPNPort: openVPNPort,
				Header:      reversedVPNHeader,
			},
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
	})

	Describe("#Deploy", func() {
		var (
			secretDHYAML = `apiVersion: v1
data:
  foo: ZGFzaA==
immutable: true
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + secretNameDHTest + `
  namespace: kube-system
type: Opaque
`
			secretTLSAuthYAML = `apiVersion: v1
data:
  data-for: dnBuLXNlZWQtdGxzYXV0aA==
immutable: true
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + secretNameTLSAuthLegacyVPN + `
  namespace: kube-system
type: Opaque
`
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
  creationTimestamp: null
  name: gardener.kube-system.vpn-shoot
spec:
  allowedCapabilities:
  - NET_ADMIN
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
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        cpu: 100m
        memory: 10Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: vpn-shoot
  updatePolicy:
    updateMode: Auto
status: {}
`
			deploymentFor = func(secretNameCA, secretNameClient, secretNameTLSAuth string, reversedVPNEnabled, vpaEnabled bool) *appsv1.Deployment {
				var (
					intStrMax, intStrZero = intstr.FromString("100%"), intstr.FromString("0%")

					annotations = map[string]string{
						references.AnnotationKey(references.KindSecret, secretNameCA):     secretNameCA,
						references.AnnotationKey(references.KindSecret, secretNameClient): secretNameClient,
					}

					env = []corev1.EnvVar{
						{
							Name:  "SERVICE_NETWORK",
							Value: serviceNetwork,
						},
						{
							Name:  "POD_NETWORK",
							Value: podNetwork,
						},
						{
							Name:  "NODE_NETWORK",
							Value: nodeNetwork,
						},
					}

					limits = corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("120Mi"),
					}

					volumeMounts = []corev1.VolumeMount{
						{
							Name:      "vpn-shoot",
							MountPath: "/srv/secrets/vpn-shoot",
						},
						{
							Name:      "vpn-shoot-tlsauth",
							MountPath: "/srv/secrets/tlsauth",
						},
					}

					volumes = []corev1.Volume{
						{
							Name: "vpn-shoot",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: pointer.Int32(420),
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
													Name: secretNameClient,
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
							Name: "vpn-shoot-tlsauth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameTLSAuth,
									DefaultMode: pointer.Int32(0400),
								},
							},
						},
					}
				)

				if reversedVPNEnabled {
					annotations[references.AnnotationKey(references.KindSecret, secretNameTLSAuth)] = secretNameTLSAuth

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
							Value: reversedVPNHeader,
						},
					)
				} else {
					annotations[references.AnnotationKey(references.KindSecret, secretNameTLSAuthLegacyVPN)] = secretNameTLSAuthLegacyVPN
					annotations[references.AnnotationKey(references.KindSecret, secretNameDHTest)] = secretNameDHTest

					volumeMounts = append(volumeMounts, corev1.VolumeMount{
						Name:      "vpn-shoot-dh",
						MountPath: "/srv/secrets/dh",
					})

					volumes = append(volumes, corev1.Volume{
						Name: "vpn-shoot-dh",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  secretNameDHTest,
								DefaultMode: pointer.Int32(0400),
							},
						},
					})
				}

				if vpaEnabled {
					limits = corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					}
				}

				obj := &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        "vpn-shoot",
						Namespace:   "kube-system",
						Annotations: annotations,
						Labels: map[string]string{
							"app":                 "vpn-shoot",
							"gardener.cloud/role": "system-component",
							"origin":              "gardener",
						},
					},
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
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: utils.MergeStringMaps(annotations, map[string]string{
									"security.gardener.cloud/trigger": "rollout",
								}),
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
								NodeSelector:                 map[string]string{"worker.gardener.cloud/system-components": "true"},
								Tolerations: []corev1.Toleration{{
									Key:      "CriticalAddonsOnly",
									Operator: corev1.TolerationOpExists,
								}},
								Containers: []corev1.Container{
									{
										Name:            "vpn-shoot",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Env:             env,
										SecurityContext: &corev1.SecurityContext{
											Privileged: pointer.Bool(true),
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
									},
								},
								Volumes: volumes,
							},
						},
					},
				}

				return obj
			}

			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:gardener.cloud:vpn-seed
rules:
- apiGroups:
  - ""
  resourceNames:
  - vpn-shoot
  resources:
  - services
  verbs:
  - get
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: system:gardener.cloud:vpn-seed
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:gardener.cloud:vpn-seed
subjects:
- kind: User
  name: vpn-seed
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: vpn-shoot
  name: vpn-shoot
  namespace: kube-system
spec:
  ports:
  - name: openvpn
    port: 4314
    protocol: TCP
    targetPort: 1194
  selector:
    app: vpn-shoot
  type: LoadBalancer
status:
  loadBalancer: {}
`
		)

		JustBeforeEach(func() {
			vpnShoot = New(c, namespace, sm, values)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

			vpnShoot.SetSecrets(secrets)
			Expect(vpnShoot.Deploy(ctx)).To(Succeed())

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
					SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
					KeepObjects:  pointer.BoolPtr(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))

			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-vpn.yaml"])).To(Equal(networkPolicyYAML))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__vpn-shoot.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.vpn-shoot.yaml"])).To(Equal(podSecurityPolicyYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_vpn-shoot.yaml"])).To(Equal(clusterRolePSPYAML))
			Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_vpn-shoot.yaml"])).To(Equal(roleBindingPSPYAML))

			if !values.ReversedVPN.Enabled {
				Expect(string(managedResourceSecret.Data["secret__kube-system__"+secretNameTLSAuthLegacyVPN+".yaml"])).To(Equal(secretTLSAuthYAML))
			}
		})

		Context("VPNShoot with ReversedVPN not enabled", func() {
			BeforeEach(func() {
				values.ReversedVPN.Enabled = false
				secrets.DH = &component.Secret{Name: secretNameDH, Checksum: secretChecksumDH, Data: secretDataDH}
			})

			JustBeforeEach(func() {
				Expect(string(managedResourceSecret.Data["clusterrole____system_gardener.cloud_vpn-seed.yaml"])).To(Equal(clusterRoleYAML))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____system_gardener.cloud_vpn-seed.yaml"])).To(Equal(clusterRoleBindingYAML))
				Expect(string(managedResourceSecret.Data["service__kube-system__vpn-shoot.yaml"])).To(Equal(serviceYAML))
				Expect(string(managedResourceSecret.Data["secret__kube-system__"+secretNameDHTest+".yaml"])).To(Equal(secretDHYAML))
			})

			Context("w/o VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = false
				})

				It("should successfully deploy all resources", func() {
					secretNameClient := expectVPNShootSecret(managedResourceSecret.Data, values.ReversedVPN.Enabled)
					secretNameCA := expectCASecret(managedResourceSecret.Data)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(Equal(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuthLegacyVPN, values.ReversedVPN.Enabled, values.VPAEnabled)))
				})
			})

			Context("w/ VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__vpn-shoot.yaml"])).To(Equal(vpaYAML))

					secretNameClient := expectVPNShootSecret(managedResourceSecret.Data, values.ReversedVPN.Enabled)
					secretNameCA := expectCASecret(managedResourceSecret.Data)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(Equal(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuthLegacyVPN, values.ReversedVPN.Enabled, values.VPAEnabled)))
				})
			})
		})

		Context("VPNShoot with ReversedVPN enabled", func() {
			BeforeEach(func() {
				values.ReversedVPN.Enabled = true
			})

			Context("w/o VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = false
				})

				It("should successfully deploy all resources", func() {
					var (
						secretNameClient  = expectVPNShootSecret(managedResourceSecret.Data, values.ReversedVPN.Enabled)
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(Equal(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth, values.ReversedVPN.Enabled, values.VPAEnabled)))
				})
			})

			Context("w/ VPA", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__vpn-shoot.yaml"])).To(Equal(vpaYAML))

					var (
						secretNameClient  = expectVPNShootSecret(managedResourceSecret.Data, values.ReversedVPN.Enabled)
						secretNameCA      = expectCASecret(managedResourceSecret.Data)
						secretNameTLSAuth = expectTLSAuthSecret(managedResourceSecret.Data)
					)

					deployment := &appsv1.Deployment{}
					Expect(runtime.DecodeInto(newCodec(), managedResourceSecret.Data["deployment__kube-system__vpn-shoot.yaml"], deployment)).To(Succeed())
					Expect(deployment).To(Equal(deploymentFor(secretNameCA, secretNameClient, secretNameTLSAuth, values.ReversedVPN.Enabled, values.VPAEnabled)))
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
				}))
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
				}))
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

func expectVPNShootSecret(data map[string][]byte, reversedVPNEnabled bool) string {
	suffix := "client"
	if !reversedVPNEnabled {
		suffix = "server"
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
