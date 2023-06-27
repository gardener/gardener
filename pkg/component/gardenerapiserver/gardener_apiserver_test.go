// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerapiserver_test

import (
	"context"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/apiserver"
	. "github.com/gardener/gardener/pkg/component/gardenerapiserver"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerAPIServer", func() {
	var (
		ctx = context.TODO()

		managedResourceNameRuntime = "gardener-apiserver-runtime"
		managedResourceNameVirtual = "gardener-apiserver-virtual"
		namespace                  = "some-namespace"

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		values            Values
		deployer          Interface

		fakeOps *retryfake.Ops

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		podDisruptionBudget *policyv1.PodDisruptionBudget
		serviceRuntime      *corev1.Service
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
		hvpa                *hvpav1alpha1.Hvpa
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			Values: apiserver.Values{
				ETCDEncryption: apiserver.ETCDEncryptionConfig{
					Resources: []string{"shootstates.core.gardener.cloud"},
				},
				RuntimeVersion: semver.MustParse("1.27.1"),
			},
			TopologyAwareRoutingEnabled: true,
		}
		deployer = New(fakeClient, namespace, fakeSecretManager, values)

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: gardenerutils.IntStrPtrFromInt(1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				}},
			},
		}
		serviceRuntime = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver",
				Namespace: namespace,
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-webhook-targets-allowed-ports": `[{"protocol":"TCP","port":8443}]`,
					"service.kubernetes.io/topology-mode":                                        "auto",
				},
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
					"endpoint-slice-hints.resources.gardener.cloud/consider": "true",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
				Ports: []corev1.ServicePort{{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8443),
				}},
			},
		}
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver-vpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-apiserver",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
		}
		hvpa = &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-apiserver-hvpa",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "apiserver",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
			},
			Spec: hvpav1alpha1.HvpaSpec{
				Replicas: pointer.Int32(1),
				Hpa: hvpav1alpha1.HpaSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "gardener-apiserver-hpa"}},
					Deploy:   true,
					ScaleUp: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
					},
					ScaleDown: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
					},
					Template: hvpav1alpha1.HpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"role": "gardener-apiserver-hpa"},
						},
						Spec: hvpav1alpha1.HpaTemplateSpec{
							MinReplicas: pointer.Int32(1),
							MaxReplicas: 4,
							Metrics: []autoscalingv2beta1.MetricSpec{
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceCPU,
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     corev1.ResourceMemory,
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
							},
						},
					},
				},
				Vpa: hvpav1alpha1.VpaSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "gardener-apiserver-vpa"}},
					Deploy:   true,
					ScaleUp: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
						StabilizationDuration: pointer.String("3m"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("300m"),
								Percentage: pointer.Int32(80),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("200M"),
								Percentage: pointer.Int32(80),
							},
						},
					},
					ScaleDown: hvpav1alpha1.ScaleType{
						UpdatePolicy: hvpav1alpha1.UpdatePolicy{
							UpdateMode: pointer.String("Auto"),
						},
						StabilizationDuration: pointer.String("15m"),
						MinChange: hvpav1alpha1.ScaleParams{
							CPU: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("600m"),
								Percentage: pointer.Int32(80),
							},
							Memory: hvpav1alpha1.ChangeParams{
								Value:      pointer.String("600M"),
								Percentage: pointer.Int32(80),
							},
						},
					},
					LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1"),
							Percentage: pointer.Int32(70),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1G"),
							Percentage: pointer.Int32(70),
						},
					},
					Template: hvpav1alpha1.VpaTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"role": "gardener-apiserver-vpa"},
						},
						Spec: hvpav1alpha1.VpaTemplateSpec{
							ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
								ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
									ContainerName: "gardener-apiserver",
									MinAllowed: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("400M"),
									},
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("25G"),
									},
								}},
							},
						},
					},
				},
				WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{{
					VpaWeight:         0,
					StartReplicaCount: 1,
					LastReplicaCount:  3,
				}},
				TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "gardener-apiserver",
				},
			},
		}
	})

	Describe("#Deploy", func() {
		Context("deployment", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			Context("secrets", func() {
				Context("etcd encryption config secrets", func() {
					It("should successfully deploy the ETCD encryption configuration secret resource", func() {
						etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key-62135596800
        secret: ________________________________
  - identity: {}
  resources:
  - shootstates.core.gardener.cloud
`

						By("Verify encryption config secret")
						expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-etcd-encryption-configuration", Namespace: namespace},
							Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
						}
						Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

						actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

						Expect(deployer.Deploy(ctx)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
						Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      expectedSecretETCDEncryptionConfiguration.Name,
								Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
								Labels: map[string]string{
									"resources.gardener.cloud/garbage-collectable-reference": "true",
									"role": "gardener-apiserver-etcd-encryption-configuration",
								},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      expectedSecretETCDEncryptionConfiguration.Data,
						}))

						By("Deploy again and ensure that labels are still present")
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
						Expect(actualSecretETCDEncryptionConfiguration.Labels).To(Equal(map[string]string{
							"resources.gardener.cloud/garbage-collectable-reference": "true",
							"role": "gardener-apiserver-etcd-encryption-configuration",
						}))

						By("Verify encryption key secret")
						secretList := &corev1.SecretList{}
						Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
							"name":       "gardener-apiserver-etcd-encryption-key",
							"managed-by": "secrets-manager",
						})).To(Succeed())
						Expect(secretList.Items).To(HaveLen(1))
						Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
					})

					DescribeTable("successfully deploy the ETCD encryption configuration secret resource w/ old key",
						func(encryptWithCurrentKey bool) {
							deployer = New(fakeClient, namespace, fakeSecretManager, Values{
								Values: apiserver.Values{
									ETCDEncryption: apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: encryptWithCurrentKey, Resources: []string{"shootstates.core.gardener.cloud"}},
								},
							})

							oldKeyName, oldKeySecret := "key-old", "old-secret"
							Expect(fakeClient.Create(ctx, &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "gardener-apiserver-etcd-encryption-key-old",
									Namespace: namespace,
								},
								Data: map[string][]byte{
									"key":    []byte(oldKeyName),
									"secret": []byte(oldKeySecret),
								},
							})).To(Succeed())

							etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:`

							if encryptWithCurrentKey {
								etcdEncryptionConfiguration += `
      - name: key-62135596800
        secret: ________________________________
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret
							} else {
								etcdEncryptionConfiguration += `
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret + `
      - name: key-62135596800
        secret: ________________________________`
							}

							etcdEncryptionConfiguration += `
  - identity: {}
  resources:
  - shootstates.core.gardener.cloud
`

							expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-etcd-encryption-configuration", Namespace: namespace},
								Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
							}
							Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

							actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
							Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

							Expect(deployer.Deploy(ctx)).To(Succeed())

							Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
							Expect(actualSecretETCDEncryptionConfiguration).To(DeepEqual(&corev1.Secret{
								TypeMeta: metav1.TypeMeta{
									APIVersion: corev1.SchemeGroupVersion.String(),
									Kind:       "Secret",
								},
								ObjectMeta: metav1.ObjectMeta{
									Name:      expectedSecretETCDEncryptionConfiguration.Name,
									Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
									Labels: map[string]string{
										"resources.gardener.cloud/garbage-collectable-reference": "true",
										"role": "gardener-apiserver-etcd-encryption-configuration",
									},
									ResourceVersion: "1",
								},
								Immutable: pointer.Bool(true),
								Data:      expectedSecretETCDEncryptionConfiguration.Data,
							}))

							secretList := &corev1.SecretList{}
							Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
								"name":       "gardener-apiserver-etcd-encryption-key",
								"managed-by": "secrets-manager",
							})).To(Succeed())
							Expect(secretList.Items).To(HaveLen(1))
							Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
						},

						Entry("encrypting with current", true),
						Entry("encrypting with old", false),
					)
				})

				It("should successfully deploy the access secret for the virtual garden", func() {
					accessSecret := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "shoot-access-gardener-apiserver",
							Namespace: namespace,
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
							Annotations: map[string]string{
								"serviceaccount.resources.gardener.cloud/name":      "gardener-apiserver",
								"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
							},
						},
						Type: corev1.SecretTypeOpaque,
					}

					Expect(deployer.Deploy(ctx)).To(Succeed())

					actualShootAccessSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), actualShootAccessSecret)).To(Succeed())
					accessSecret.ResourceVersion = "1"
					Expect(actualShootAccessSecret).To(Equal(accessSecret))
				})

				It("should successfully deploy the audit webhook kubeconfig secret resource", func() {
					var (
						kubeconfig  = []byte("some-kubeconfig")
						auditConfig = &apiserver.AuditConfig{Webhook: &apiserver.AuditWebhook{Kubeconfig: kubeconfig}}
					)

					deployer = New(fakeClient, namespace, fakeSecretManager, Values{
						Values: apiserver.Values{
							Audit: auditConfig,
						},
					})

					expectedSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-webhook-kubeconfig", Namespace: namespace},
						Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
					}
					Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

					actualSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
					Expect(actualSecret).To(DeepEqual(&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            expectedSecret.Name,
							Namespace:       expectedSecret.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: pointer.Bool(true),
						Data:      expectedSecret.Data,
					}))
				})

				Context("admission kubeconfigs", func() {
					It("should successfully deploy the secret resource w/o admission plugin kubeconfigs", func() {
						secretAdmissionKubeconfigs := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-kubeconfigs", Namespace: namespace},
							Data:       map[string][]byte{},
						}
						Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
						Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      secretAdmissionKubeconfigs.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz"}, Kubeconfig: []byte("foo")},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						secretAdmissionKubeconfigs := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-kubeconfigs", Namespace: namespace},
							Data: map[string][]byte{
								"baz-kubeconfig.yaml": []byte("foo"),
							},
						}
						Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
						Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "Secret",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            secretAdmissionKubeconfigs.Name,
								Namespace:       secretAdmissionKubeconfigs.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      secretAdmissionKubeconfigs.Data,
						}))
					})
				})
			})

			Context("configmaps", func() {
				Context("audit", func() {
					It("should successfully deploy the configmap resource w/ default policy", func() {
						configMapAuditPolicy := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-policy-config", Namespace: namespace},
							Data: map[string]string{"audit-policy.yaml": `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`},
						}
						Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/o default policy", func() {
						var (
							policy      = "some-audit-policy"
							auditConfig = &apiserver.AuditConfig{Policy: &policy}
						)

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								Audit: auditConfig,
							},
						})

						configMapAuditPolicy := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-audit-policy-config", Namespace: namespace},
							Data:       map[string]string{"audit-policy.yaml": policy},
						}
						Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})
				})

				Context("admission", func() {
					It("should successfully deploy the configmap resource w/o admission plugins", func() {
						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: null
`},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
							{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("some-config-for-baz")}}},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
								Kubeconfig: []byte("foo"),
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
								Kubeconfig: []byte("foo"),
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: Baz
  path: /etc/kubernetes/admission/baz.yaml
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"baz.yaml": "some-config-for-baz",
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins w/ config but w/o kubeconfigs", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
								},
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: ""
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: ""
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins w/o configs but w/ kubeconfig", func() {
						admissionPlugins := []apiserver.AdmissionPluginConfig{
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "MutatingAdmissionWebhook",
								},
								Kubeconfig: []byte("foo"),
							},
							{
								AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
									Name: "ValidatingAdmissionWebhook",
								},
								Kubeconfig: []byte("foo"),
							},
						}

						deployer = New(fakeClient, namespace, fakeSecretManager, Values{
							Values: apiserver.Values{
								EnabledAdmissionPlugins: admissionPlugins,
							},
						})

						configMapAdmission := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
`,
								"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
								"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							},
						}
						Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
						Expect(deployer.Deploy(ctx)).To(Succeed())
						Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})
				})
			})

			Context("resources generation", func() {
				JustBeforeEach(func() {
					Expect(deployer.Deploy(ctx)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
					Expect(managedResourceRuntime).To(Equal(&resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceRuntime.Name,
							Namespace:       managedResourceRuntime.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							Class:       pointer.String("seed"),
							SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceSecretRuntime.Name}},
							KeepObjects: pointer.Bool(false),
						},
						Status: healthyManagedResourceStatus,
					}))

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
					Expect(managedResourceVirtual).To(Equal(&resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceVirtual.Name,
							Namespace:       managedResourceVirtual.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels:          map[string]string{"origin": "gardener"},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecretVirtual.Name}},
							KeepObjects:  pointer.Bool(false),
						},
						Status: healthyManagedResourceStatus,
					}))

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())
				})

				Context("when HVPA is disabled", func() {
					It("should successfully deploy all resources when HVPA is disabled", func() {
						Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
						Expect(managedResourceSecretRuntime.Data).To(HaveLen(3))
						Expect(string(managedResourceSecretRuntime.Data["poddisruptionbudget__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(podDisruptionBudget)))
						Expect(string(managedResourceSecretRuntime.Data["service__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(serviceRuntime)))
						Expect(string(managedResourceSecretRuntime.Data["verticalpodautoscaler__some-namespace__gardener-apiserver-vpa.yaml"])).To(Equal(componenttest.Serialize(vpa)))

						Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
						Expect(managedResourceSecretVirtual.Data).To(HaveLen(0))
					})
				})

				Context("when HVPA is enabled", func() {
					BeforeEach(func() {
						values.Values.Autoscaling.HVPAEnabled = true
						deployer = New(fakeClient, namespace, fakeSecretManager, values)
					})

					It("should successfully deploy all resources when HVPA is enabled", func() {
						Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
						Expect(managedResourceSecretRuntime.Data).To(HaveLen(3))
						Expect(string(managedResourceSecretRuntime.Data["poddisruptionbudget__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(podDisruptionBudget)))
						Expect(string(managedResourceSecretRuntime.Data["service__some-namespace__gardener-apiserver.yaml"])).To(Equal(componenttest.Serialize(serviceRuntime)))
						Expect(string(managedResourceSecretRuntime.Data["hvpa__some-namespace__gardener-apiserver-hvpa.yaml"])).To(Equal(componenttest.Serialize(hvpa)))

						Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
						Expect(managedResourceSecretVirtual.Data).To(HaveLen(0))
					})
				})
			})
		})

		Context("waiting logic", func() {
			It("should fail because the runtime ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Deploy(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the virtual ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Deploy(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is unhealthy")))
			})

			It("should fail because the runtime ManagedResource is still progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("still progressing")))
			})

			It("should fail because the virtual ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the both ManagedResource are healthy and progressed", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
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
)
