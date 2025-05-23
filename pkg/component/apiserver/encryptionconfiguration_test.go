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
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("EncryptionConfiguration", func() {
	var (
		ctx                         = context.TODO()
		secretNameETCDEncryptionKey = "etcd-encryption-key"
		encryptionRoleLabel         = "etcd-encryption"
		namespace                   = "some-namespace"

		config ETCDEncryptionConfig

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
	)

	BeforeEach(func() {
		config = ETCDEncryptionConfig{ResourcesToEncrypt: []string{"foo"}}

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
	})

	Describe("#ReconcileSecretETCDEncryptionConfiguration", func() {
		It("should successfully deploy the ETCD encryption configuration secret resource", func() {
			etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
  - identity: {}
  resources:
  - foo
`

			By("Verify encryption config secret")
			expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace},
				Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
			}
			Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

			actualSecretETCDEncryptionConfiguration := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

			Expect(ReconcileSecretETCDEncryptionConfiguration(ctx, fakeClient, fakeSecretManager, config, actualSecretETCDEncryptionConfiguration, secretNameETCDEncryptionKey, encryptionRoleLabel)).To(Succeed())

			Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedSecretETCDEncryptionConfiguration.Name,
					Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
					Labels: map[string]string{
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"role": encryptionRoleLabel,
					},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      expectedSecretETCDEncryptionConfiguration.Data,
			}))

			By("Deploy again and ensure that labels are still present")
			actualSecretETCDEncryptionConfiguration.ResourceVersion = ""
			Expect(ReconcileSecretETCDEncryptionConfiguration(ctx, fakeClient, fakeSecretManager, config, actualSecretETCDEncryptionConfiguration, secretNameETCDEncryptionKey, encryptionRoleLabel)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
			Expect(actualSecretETCDEncryptionConfiguration.Labels).To(Equal(map[string]string{
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"role": encryptionRoleLabel,
			}))

			By("Verify encryption key secret")
			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
				"name":       secretNameETCDEncryptionKey,
				"managed-by": "secrets-manager",
			})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
			Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
		})

		It("should successfully deploy the ETCD encryption configuration secret resource with the right config when resources are removed from encryption", func() {
			config = ETCDEncryptionConfig{ResourcesToEncrypt: []string{"foo", "bin"}, EncryptedResources: []string{"bar", "bin"}}

			etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
  - identity: {}
  resources:
  - foo
  - bin
- providers:
  - identity: {}
  - aescbc:
      keys:
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
  resources:
  - bar
`

			By("Verify encryption config secret")
			expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace},
				Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
			}
			Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

			actualSecretETCDEncryptionConfiguration := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

			Expect(ReconcileSecretETCDEncryptionConfiguration(ctx, fakeClient, fakeSecretManager, config, actualSecretETCDEncryptionConfiguration, secretNameETCDEncryptionKey, encryptionRoleLabel)).To(Succeed())

			Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedSecretETCDEncryptionConfiguration.Name,
					Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
					Labels: map[string]string{
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"role": encryptionRoleLabel,
					},
					ResourceVersion: "1",
				},
				Immutable: ptr.To(true),
				Data:      expectedSecretETCDEncryptionConfiguration.Data,
			}))

			By("Deploy again and ensure that labels are still present")
			actualSecretETCDEncryptionConfiguration.ResourceVersion = ""
			Expect(ReconcileSecretETCDEncryptionConfiguration(ctx, fakeClient, fakeSecretManager, config, actualSecretETCDEncryptionConfiguration, secretNameETCDEncryptionKey, encryptionRoleLabel)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
			Expect(actualSecretETCDEncryptionConfiguration.Labels).To(Equal(map[string]string{
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"role": encryptionRoleLabel,
			}))

			By("Verify encryption key secret")
			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
				"name":       secretNameETCDEncryptionKey,
				"managed-by": "secrets-manager",
			})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
			Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
		})

		DescribeTable("successfully deploy the ETCD encryption configuration secret resource w/ old key",
			func(encryptWithCurrentKey bool) {
				config.EncryptWithCurrentKey = encryptWithCurrentKey

				oldKeyName, oldKeySecret := "key-old", "old-secret"
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNameETCDEncryptionKey + "-old",
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
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret
				} else {
					etcdEncryptionConfiguration += `
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret + `
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=`
				}

				etcdEncryptionConfiguration += `
  - identity: {}
  resources:
  - foo
`

				expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace},
					Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

				actualSecretETCDEncryptionConfiguration := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-encryption-config", Namespace: namespace}}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

				Expect(ReconcileSecretETCDEncryptionConfiguration(ctx, fakeClient, fakeSecretManager, config, actualSecretETCDEncryptionConfiguration, secretNameETCDEncryptionKey, encryptionRoleLabel)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
				Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedSecretETCDEncryptionConfiguration.Name,
						Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
						Labels: map[string]string{
							"resources.gardener.cloud/garbage-collectable-reference": "true",
							"role": encryptionRoleLabel,
						},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecretETCDEncryptionConfiguration.Data,
				}))

				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"name":       secretNameETCDEncryptionKey,
					"managed-by": "secrets-manager",
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))
				Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
			},

			Entry("encrypting with current", true),
			Entry("encrypting with old", false),
		)
	})

	Describe("#InjectEncryptionSettings", func() {
		It("should inject the correct settings", func() {
			deployment := &appsv1.Deployment{}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{})

			secretETCDEncryptionConfiguration := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "etcd-enc-config"}}

			InjectEncryptionSettings(deployment, secretETCDEncryptionConfiguration)

			Expect(deployment).To(Equal(&appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Args: []string{
									"--encryption-provider-config=/etc/kubernetes/etcd-encryption-secret/encryption-configuration.yaml",
								},
								VolumeMounts: []corev1.VolumeMount{{
									Name:      "etcd-encryption-secret",
									MountPath: "/etc/kubernetes/etcd-encryption-secret",
									ReadOnly:  true,
								}},
							}},
							Volumes: []corev1.Volume{{
								Name: "etcd-encryption-secret",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  secretETCDEncryptionConfiguration.Name,
										DefaultMode: ptr.To[int32](0640),
									},
								},
							}},
						},
					},
				},
			}))
		})
	})
})
