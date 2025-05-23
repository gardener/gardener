// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/apiserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("AuditWebhook", func() {
	var (
		ctx        = context.TODO()
		namespace  = "some-namespace"
		kubeconfig = []byte("some-kubeconfig")

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#ReconcileSecretAuditWebhookKubeconfig", func() {
		It("should do nothing because config is nil", func() {
			Expect(ReconcileSecretAuditWebhookKubeconfig(ctx, fakeClient, nil, nil)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})

		It("should do nothing because webhook config is nil", func() {
			Expect(ReconcileSecretAuditWebhookKubeconfig(ctx, fakeClient, nil, &AuditConfig{})).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})

		It("should do nothing because webhook kubeconfig is nil", func() {
			Expect(ReconcileSecretAuditWebhookKubeconfig(ctx, fakeClient, nil, &AuditConfig{Webhook: &AuditWebhook{}})).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})

		It("should successfully deploy the audit webhook kubeconfig secret resource", func() {
			expectedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "apiserver-audit-webhook-kubeconfig", Namespace: namespace},
				Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
			}
			Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-audit-webhook-kubeconfig", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

			Expect(ReconcileSecretAuditWebhookKubeconfig(ctx, fakeClient, actualSecret, &AuditConfig{Webhook: &AuditWebhook{Kubeconfig: kubeconfig}})).To(Succeed())

			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            expectedSecret.Name,
					Namespace:       expectedSecret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      expectedSecret.Data,
			}))
		})
	})

	Describe("#ReconcileSecretWebhookKubeconfig", func() {
		It("should successfully deploy the kubeconfig secret and make it unique", func() {
			expectedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "apiserver-kubeconfig", Namespace: namespace},
				Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
			}
			Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-kubeconfig", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

			Expect(ReconcileSecretAuditWebhookKubeconfig(ctx, fakeClient, actualSecret, &AuditConfig{Webhook: &AuditWebhook{Kubeconfig: kubeconfig}})).To(Succeed())

			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            expectedSecret.Name,
					Namespace:       expectedSecret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      expectedSecret.Data,
			}))
		})
	})

	Describe("#ReconcileConfigMapAuditPolicy", func() {
		It("should successfully deploy the configmap resource w/ default policy", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
				Data: map[string]string{"audit-policy.yaml": `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`},
			}
			Expect(kubernetesutils.MakeUnique(configMap)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAuditPolicy(ctx, fakeClient, configMap, nil)).To(Succeed())

			actualConfigMap := &corev1.ConfigMap{ObjectMeta: configMap.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualConfigMap), actualConfigMap)).To(Succeed())
			Expect(actualConfigMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      configMap.Data,
			}))
		})

		It("should successfully deploy the configmap resource w/o default policy", func() {
			policy := "some-audit-policy"

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
				Data:       map[string]string{"audit-policy.yaml": policy},
			}
			Expect(kubernetesutils.MakeUnique(configMap)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAuditPolicy(ctx, fakeClient, configMap, &AuditConfig{Policy: &policy})).To(Succeed())

			actualConfigMap := &corev1.ConfigMap{ObjectMeta: configMap.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualConfigMap), actualConfigMap)).To(Succeed())
			Expect(actualConfigMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      configMap.Data,
			}))
		})
	})

	Describe("#InjectAuditSettings", func() {
		It("should inject the correct settings w/o webhook", func() {
			deployment := &appsv1.Deployment{}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{})

			configMapAuditPolicy := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "audit-policy"}}

			InjectAuditSettings(deployment, configMapAuditPolicy, nil, nil)

			Expect(deployment).To(Equal(&appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: []string{
									"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
								},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      "audit-policy-config",
									MountPath: "/etc/kubernetes/audit",
								}},
							}},
							Volumes: []corev1.Volume{{
								Name: "audit-policy-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapAuditPolicy.Name,
										},
									},
								},
							}},
						},
					},
				},
			}))
		})

		It("should inject the correct settings w/ webhook", func() {
			deployment := &appsv1.Deployment{}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{})

			configMapAuditPolicy := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "audit-policy"}}
			secretWebhookKubeconfig := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "audit-webhook"}}

			InjectAuditSettings(deployment, configMapAuditPolicy, secretWebhookKubeconfig, &AuditConfig{Webhook: &AuditWebhook{
				Kubeconfig:   []byte("foo"),
				BatchMaxSize: ptr.To[int32](2),
				Version:      ptr.To("bar"),
			}})

			Expect(deployment).To(Equal(&appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: []string{
									"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
									"--audit-webhook-config-file=/etc/kubernetes/webhook/audit/kubeconfig.yaml",
									"--audit-webhook-batch-max-size=2",
									"--audit-webhook-version=bar",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "audit-policy-config",
										MountPath: "/etc/kubernetes/audit",
									},
									{
										Name:      "audit-webhook-kubeconfig",
										MountPath: "/etc/kubernetes/webhook/audit",
										ReadOnly:  true,
									},
								},
							}},
							Volumes: []corev1.Volume{
								{
									Name: "audit-policy-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapAuditPolicy.Name,
											},
										},
									},
								},
								{
									Name: "audit-webhook-kubeconfig",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretWebhookKubeconfig.Name,
										},
									},
								},
							},
						},
					},
				},
			}))
		})
	})
})
