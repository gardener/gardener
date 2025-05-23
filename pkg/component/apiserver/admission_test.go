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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/apiserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Admission", func() {
	var (
		ctx       = context.TODO()
		namespace = "some-namespace"

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#ReconcileSecretAdmissionKubeconfigs", func() {
		It("should successfully deploy the secret resource w/o admission plugin kubeconfigs", func() {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-kubeconfigs", Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())

			Expect(ReconcileSecretAdmissionKubeconfigs(ctx, fakeClient, secret, Values{})).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: secret.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())
			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secret.Name,
					Namespace:       secret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      map[string][]byte{},
			}))
		})

		It("should successfully deploy the configmap resource w/ admission plugins", func() {
			admissionPlugins := []AdmissionPluginConfig{
				{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
				{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz"}, Kubeconfig: []byte("foo")},
			}

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-kubeconfigs", Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())

			Expect(ReconcileSecretAdmissionKubeconfigs(ctx, fakeClient, secret, Values{EnabledAdmissionPlugins: admissionPlugins})).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: secret.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())
			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secret.Name,
					Namespace:       secret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data: map[string][]byte{
					"baz-kubeconfig.yaml": []byte("foo"),
				},
			}))
		})
	})

	Describe("#ReconcileConfigMapAdmission", func() {
		It("should successfully deploy the configmap resource w/o admission plugins", func() {
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAdmission(ctx, fakeClient, configMap, Values{})).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			Expect(configMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data: map[string]string{"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: null
`},
			}))
		})

		It("should successfully deploy the configmap resource w/ admission plugins", func() {
			admissionPlugins := []AdmissionPluginConfig{
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
				{
					AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
						Name: "ImagePolicyWebhook",
						Config: &runtime.RawExtension{Raw: []byte(`imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
					},
					Kubeconfig: []byte("foo"),
				},
			}

			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAdmission(ctx, fakeClient, configMap, Values{EnabledAdmissionPlugins: admissionPlugins})).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			Expect(configMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
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
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
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
					"imagepolicywebhook.yaml": `imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/imagepolicywebhook-kubeconfig.yaml
`,
				},
			}))
		})

		It("should successfully deploy the configmap resource w/ admission plugins w/ config but w/o kubeconfigs", func() {
			admissionPlugins := []AdmissionPluginConfig{
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
				{
					AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
						Name: "ImagePolicyWebhook",
						Config: &runtime.RawExtension{Raw: []byte(`imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
					},
				},
			}

			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAdmission(ctx, fakeClient, configMap, Values{EnabledAdmissionPlugins: admissionPlugins})).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			Expect(configMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
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
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
`,
					"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: ""
`,
					"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: ""
`,
					"imagepolicywebhook.yaml": `imagePolicy:
  foo: bar
  kubeConfigFile: ""
`,
				},
			}))
		})

		It("should successfully deploy the configmap resource w/ admission plugins w/o configs but w/ kubeconfig", func() {
			admissionPlugins := []AdmissionPluginConfig{
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
				{
					AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
						Name: "ImagePolicyWebhook",
					},
					Kubeconfig: []byte("foo"),
				},
			}

			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(BeNotFoundError())

			Expect(ReconcileConfigMapAdmission(ctx, fakeClient, configMap, Values{EnabledAdmissionPlugins: admissionPlugins})).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
			Expect(configMap).To(DeepEqual(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            configMap.Name,
					Namespace:       configMap.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
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
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
`,
					"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
					"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
					"imagepolicywebhook.yaml": `imagePolicy:
  kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/imagepolicywebhook-kubeconfig.yaml
`,
				},
			}))
		})
	})

	Describe("#InjectAdmissionSettings", func() {
		It("should inject the correct settings", func() {
			deployment := &appsv1.Deployment{}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{})

			configMapAdmissionConfigs := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "admission-configs"}}
			secretAdmissionKubeconfigs := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "admission-kubeconfigs"}}

			InjectAdmissionSettings(deployment, configMapAdmissionConfigs, secretAdmissionKubeconfigs, Values{
				EnabledAdmissionPlugins: []AdmissionPluginConfig{
					{
						AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"},
						Kubeconfig:      []byte("foo"),
					},
					{
						AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"},
					},
				},
				DisabledAdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{
					{Name: "Baz"},
				},
			})

			Expect(deployment).To(Equal(&appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: []string{
									"--enable-admission-plugins=Foo,Bar",
									"--disable-admission-plugins=Baz",
									"--admission-control-config-file=/etc/kubernetes/admission/admission-configuration.yaml",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "admission-config",
										MountPath: "/etc/kubernetes/admission",
									},
									{
										Name:      "admission-kubeconfigs",
										MountPath: "/etc/kubernetes/admission-kubeconfigs",
									},
								},
							}},
							Volumes: []corev1.Volume{
								{
									Name: "admission-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapAdmissionConfigs.Name,
											},
										},
									},
								},
								{
									Name: "admission-kubeconfigs",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretAdmissionKubeconfigs.Name,
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
