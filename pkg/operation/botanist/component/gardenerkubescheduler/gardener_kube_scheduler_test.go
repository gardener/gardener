// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerkubescheduler_test

import (
	"context"
	"fmt"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("New", func() {
	const deployNS = "test-namespace"

	var (
		ctx                                              context.Context
		c                                                client.Client
		sched                                            component.DeployWaiter
		webhookClientConfig, expectedWebhookClientConfig *admissionregistrationv1.WebhookClientConfig
		expectedLabels                                   map[string]string
		codec                                            serializer.CodecFactory
		image                                            *imagevector.Image
	)

	BeforeEach(func() {
		ctx = context.TODO()
		image = &imagevector.Image{Repository: "foo"}
		webhookClientConfig = &admissionregistrationv1.WebhookClientConfig{}
		expectedWebhookClientConfig = webhookClientConfig.DeepCopy()

		expectedLabels = map[string]string{"app": "kubernetes", "role": "scheduler"}

		s := runtime.NewScheme()
		Expect(appsv1.AddToScheme(s)).To(Succeed())
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(rbacv1.AddToScheme(s)).To(Succeed())
		Expect(admissionregistrationv1.AddToScheme(s)).To(Succeed())
		Expect(resourcesv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(autoscalingv1beta2.AddToScheme(s)).To(Succeed())

		codec = serializer.NewCodecFactory(s, serializer.EnableStrict)

		c = fake.NewClientBuilder().WithScheme(s).Build()
	})

	Context("fails", func() {
		var err error

		It("when client is nil", func() {
			sched, err = New(nil, "foo", nil, nil, nil)
		})

		It("when namespace is empty", func() {
			sched, err = New(c, "", nil, nil, nil)
		})

		It("when namespace is garden", func() {
			sched, err = New(c, "garden", nil, nil, nil)
		})

		AfterEach(func() {
			Expect(err).To(HaveOccurred())
			Expect(sched).To(BeNil())
		})
	})

	Context("succeeds", func() {
		var (
			managedResourceSecret *corev1.Secret
			configMapName         string
		)

		BeforeEach(func() {
			managedResourceSecret = &corev1.Secret{}
			configMapName = "gardener-kube-scheduler-f7581b50"
		})

		JustBeforeEach(func() {
			s, err := New(c, deployNS, image, &dummyConfigurator{}, webhookClientConfig)
			Expect(err).NotTo(HaveOccurred(), "New succeeds")

			sched = s
		})

		Context("Deploy fails", func() {
			It("cannot accept nil configurator", func() {
				s, err := New(c, deployNS, image, nil, webhookClientConfig)
				Expect(err).To(Succeed(), "New succeeds")

				Expect(s.Deploy(ctx)).NotTo(Succeed(), "deploy should fail")
			})

			It("cannot accept configurator that returns error", func() {
				s, err := New(c, deployNS, image, &dummyConfigurator{err: fmt.Errorf("foo")}, webhookClientConfig)
				Expect(err).To(Succeed(), "New succeeds")

				Expect(s.Deploy(ctx)).NotTo(Succeed(), "deploy should fail")
			})

			It("cannot accept nil image", func() {
				s, err := New(c, deployNS, nil, &dummyConfigurator{}, webhookClientConfig)
				Expect(err).To(Succeed(), "New succeeds")

				Expect(s.Deploy(ctx)).NotTo(Succeed(), "deploy should fail")
			})

			It("cannot accept empty image", func() {
				s, err := New(c, deployNS, &imagevector.Image{}, &dummyConfigurator{}, webhookClientConfig)
				Expect(err).To(Succeed(), "New succeeds")

				Expect(s.Deploy(ctx)).NotTo(Succeed(), "deploy should fail")
			})

			It("cannot accept nil webhookClientConfig", func() {
				s, err := New(c, deployNS, image, &dummyConfigurator{}, nil)
				Expect(err).To(Succeed(), "New succeeds")

				Expect(s.Deploy(ctx)).NotTo(Succeed(), "deploy should fail")
			})
		})

		Context("Deploy", func() {
			JustBeforeEach(func() {
				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")

				Expect(c.Get(ctx, types.NamespacedName{
					Name:      "managedresource-gardener-kube-scheduler",
					Namespace: deployNS,
				}, managedResourceSecret)).NotTo(HaveOccurred(), "can get managed resource's secret")
			})

			It("namespace is created", func() {
				actual := &corev1.Namespace{}
				expected := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   deployNS,
						Labels: expectedLabels,
					},
				}
				Expect(c.Get(ctx, types.NamespacedName{Name: deployNS}, actual)).To(Succeed(), "can get namespace")

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("does not overwrite already existing labels on the namespace", func() {
				actual := &corev1.Namespace{}
				Expect(c.Get(ctx, types.NamespacedName{Name: deployNS}, actual)).To(Succeed(), "can get namespace")
				actual.Labels["foo"] = "bar"

				Expect(c.Update(ctx, actual)).To(Succeed(), "can update existing namespace")

				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")

				expected := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   deployNS,
						Labels: map[string]string{"app": "kubernetes", "role": "scheduler", "foo": "bar"},
					},
				}

				Expect(c.Get(ctx, types.NamespacedName{Name: deployNS}, actual)).To(Succeed(), "can get updated namespace")
				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("rolebinding is created", func() {
				const key = "rolebinding__kube-system__gardener.cloud_kube-scheduler_extension-apiserver-authentication-reader.yaml"
				actual := &rbacv1.RoleBinding{}
				expected := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener.cloud:kube-scheduler:extension-apiserver-authentication-reader",
						Namespace: "kube-system",
						Labels:    expectedLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "Role",
						Name:     "extension-apiserver-authentication-reader",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup:  corev1.SchemeGroupVersion.Group,
						Kind:      "ServiceAccount",
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("lease rolebinding is created", func() {
				const key = "rolebinding__test-namespace__gardener-kube-scheduler.yaml"
				actual := &rbacv1.RoleBinding{}
				expected := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "Role",
						Name:     "gardener-kube-scheduler",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup:  corev1.SchemeGroupVersion.Group,
						Kind:      "ServiceAccount",
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("lease role is created", func() {
				const key = "role__test-namespace__gardener-kube-scheduler.yaml"
				actual := &rbacv1.Role{}
				expected := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Rules: []rbacv1.PolicyRule{{
						Verbs:     []string{"create"},
						APIGroups: []string{"coordination.k8s.io"},
						Resources: []string{"leases"},
					}, {
						Verbs:         []string{"get", "update"},
						APIGroups:     []string{"coordination.k8s.io"},
						Resources:     []string{"leases"},
						ResourceNames: []string{"gardener-kube-scheduler"},
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("volume-scheduler clusterrolebinding is created", func() {
				const key = "clusterrolebinding____gardener.cloud_volume-scheduler.yaml"
				actual := &rbacv1.ClusterRoleBinding{}
				expected := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:volume-scheduler",
						Labels: expectedLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "system:volume-scheduler",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup:  corev1.SchemeGroupVersion.Group,
						Kind:      "ServiceAccount",
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("kube-scheduler clusterrolebinding is created", func() {
				const key = "clusterrolebinding____gardener.cloud_kube-scheduler.yaml"
				actual := &rbacv1.ClusterRoleBinding{}
				expected := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:kube-scheduler",
						Labels: expectedLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "ClusterRole",
						Name:     "system:kube-scheduler",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup:  corev1.SchemeGroupVersion.Group,
						Kind:      "ServiceAccount",
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("serviceacccount is created", func() {
				const key = "serviceaccount__test-namespace__gardener-kube-scheduler.yaml"
				actual := &corev1.ServiceAccount{}
				expected := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("mutatingwebhook is created", func() {
				const key = "mutatingwebhookconfiguration____kube-scheduler.scheduling.gardener.cloud.yaml"
				var (
					failurePolicy      = admissionregistrationv1.Ignore
					matchPolicy        = admissionregistrationv1.Exact
					actual             = &admissionregistrationv1.MutatingWebhookConfiguration{}
					reinvocationPolicy = admissionregistrationv1.NeverReinvocationPolicy
					sideEffects        = admissionregistrationv1.SideEffectClassNone
					scope              = admissionregistrationv1.NamespacedScope
					expected           = &admissionregistrationv1.MutatingWebhookConfiguration{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "kube-scheduler.scheduling.gardener.cloud",
							Labels: expectedLabels,
						},
						Webhooks: []admissionregistrationv1.MutatingWebhook{{
							Name:                    "kube-scheduler.scheduling.gardener.cloud",
							AdmissionReviewVersions: []string{admissionv1beta1.SchemeGroupVersion.Version, admissionv1.SchemeGroupVersion.Version},
							ClientConfig:            *expectedWebhookClientConfig,
							FailurePolicy:           &failurePolicy,
							MatchPolicy:             &matchPolicy,
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"gardener.cloud/role": "shoot"},
							},
							ObjectSelector:     &metav1.LabelSelector{},
							ReinvocationPolicy: &reinvocationPolicy,
							SideEffects:        &sideEffects,
							Rules: []admissionregistrationv1.RuleWithOperations{{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{corev1.SchemeGroupVersion.Group},
									APIVersions: []string{corev1.SchemeGroupVersion.Version},
									Resources:   []string{"pods"},
									Scope:       &scope,
								},
							}},
							TimeoutSeconds: pointer.Int32(2),
						}},
					}
				)

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("deployment is created", func() {
				const key = "deployment__test-namespace__gardener-kube-scheduler.yaml"
				actual := &appsv1.Deployment{}
				expected := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas:             pointer.Int32(2),
						RevisionHistoryLimit: pointer.Int32(1),
						Selector:             &metav1.LabelSelector{MatchLabels: expectedLabels},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels:      expectedLabels,
								Annotations: map[string]string{},
							},
							Spec: corev1.PodSpec{
								Affinity: &corev1.Affinity{
									PodAntiAffinity: &corev1.PodAntiAffinity{
										PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
											Weight: 100,
											PodAffinityTerm: corev1.PodAffinityTerm{
												TopologyKey:   corev1.LabelHostname,
												LabelSelector: &metav1.LabelSelector{MatchLabels: expectedLabels},
											},
										}},
									},
								},
								ServiceAccountName: "gardener-kube-scheduler",
								Containers: []corev1.Container{
									{
										Name:            "kube-scheduler",
										Image:           image.String(),
										ImagePullPolicy: corev1.PullIfNotPresent,
										Command: []string{
											"/usr/local/bin/kube-scheduler",
											"--config=/var/lib/kube-scheduler-config/config.yaml",
											"--secure-port=10259",
											"--port=0",
											"--v=2",
										},
										LivenessProbe: &corev1.Probe{
											Handler: corev1.Handler{
												HTTPGet: &corev1.HTTPGetAction{
													Path:   "/healthz",
													Scheme: corev1.URISchemeHTTPS,
													Port:   intstr.FromInt(10259),
												},
											},
											SuccessThreshold:    1,
											FailureThreshold:    2,
											InitialDelaySeconds: 15,
											PeriodSeconds:       10,
											TimeoutSeconds:      15,
										},
										Ports: []corev1.ContainerPort{
											{
												Name:          "metrics",
												ContainerPort: 10259,
												Protocol:      corev1.ProtocolTCP,
											},
										},
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("23m"),
												corev1.ResourceMemory: resource.MustParse("64Mi"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("400m"),
												corev1.ResourceMemory: resource.MustParse("512Mi"),
											},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "config",
												MountPath: "/var/lib/kube-scheduler-config",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "config",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapName,
												},
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(references.InjectAnnotations(expected)).To(Succeed())

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("configmap is created", func() {
				var key = "configmap__test-namespace__" + configMapName + ".yaml"
				actual := &corev1.ConfigMap{}
				expected := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapName,
						Namespace: deployNS,
						Labels: utils.MergeStringMaps(expectedLabels, map[string]string{
							"resources.gardener.cloud/garbage-collectable-reference": "true",
						}),
					},
					Immutable: pointer.Bool(true),
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
				Expect(actual.Data).To(HaveKey("config.yaml"))

				Expect(actual.Data["config.yaml"]).To(Equal("dummy"))
			})

			It("vpa is created", func() {
				const key = "verticalpodautoscaler__test-namespace__gardener-kube-scheduler.yaml"
				updateMode := autoscalingv1beta2.UpdateModeAuto
				actual := &autoscalingv1beta2.VerticalPodAutoscaler{}
				expected := &autoscalingv1beta2.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
							Name:       "gardener-kube-scheduler",
						},
						UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
							UpdateMode: &updateMode,
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("poddisruptionbudget is created", func() {
				const key = "poddisruptionbudget__test-namespace__gardener-kube-scheduler.yaml"
				actual := &policyv1beta1.PodDisruptionBudget{}
				expected := &policyv1beta1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gardener-kube-scheduler",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: policyv1beta1.PodDisruptionBudgetSpec{
						MinAvailable: &intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: int32(1),
						},
						MaxUnavailable: nil,
						Selector: &metav1.LabelSelector{
							MatchLabels: expectedLabels,
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})
		})

		It("scheduler config is the same", func() {
			Expect(webhookClientConfig).To(DeepDerivativeEqual(expectedWebhookClientConfig))
		})

		Context("Destroy", func() {
			JustBeforeEach(func() {
				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")
				Expect(sched.Destroy(ctx)).To(Succeed(), "destroy succeeds")
			})

			It("namespace is deleted", func() {
				actual := &corev1.Namespace{}
				Expect(c.Get(ctx, types.NamespacedName{Name: deployNS}, actual)).To(BeNotFoundError())
			})
		})
	})
})

type dummyConfigurator struct {
	err error
}

func (d *dummyConfigurator) Config() (string, error) {
	return "dummy", d.err
}
