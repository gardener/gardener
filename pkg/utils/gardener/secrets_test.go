// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Secrets", func() {
	Describe("#FetchKubeconfigFromSecret", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			secret *corev1.Secret
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: "test-namespace",
				},
			}
		})

		Describe("#FetchKubeconfigFromSecret", func() {
			It("should return an error because the secret does not exist", func() {
				_, err := FetchKubeconfigFromSecret(ctx, fakeClient, client.ObjectKeyFromObject(secret))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("secrets \"secret-1\" not found")))
			})

			It("should return an error because the secret does not contain kubeconfig", func() {
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())
				_, err := FetchKubeconfigFromSecret(ctx, fakeClient, client.ObjectKeyFromObject(secret))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(("the secret's field 'kubeconfig' is either not present or empty")))
			})

			It("should return an error because the kubeconfig data is empty", func() {
				secret.Data = map[string][]byte{kubernetes.KubeConfig: {}}
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())
				_, err := FetchKubeconfigFromSecret(ctx, fakeClient, client.ObjectKeyFromObject(secret))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(("the secret's field 'kubeconfig' is either not present or empty")))
			})

			It("should return kubeconfig data if secret is prensent and contains valid kubeconfig", func() {
				secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte("secret-data")}
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())
				kubeConfig, err := FetchKubeconfigFromSecret(ctx, fakeClient, client.ObjectKeyFromObject(secret))
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeConfig).To(Equal([]byte("secret-data")))
			})
		})
	})

	Describe("#ReplicateGlobalMonitoringSecret", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			prefix                 = "prefix"
			namespace              = "namespace"
			globalMonitoringSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "global-monitoring-secret",
					Namespace:   "foo",
					Labels:      map[string]string{"bar": "baz"},
					Annotations: map[string]string{"baz": "foo"},
				},
				Type:      corev1.SecretTypeOpaque,
				Immutable: ptr.To(false),
				Data:      map[string][]byte{"username": []byte("bar"), "password": []byte("baz")},
			}
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		It("should replicate the secret", func() {
			assertions := func(secret *corev1.Secret) {
				Expect(secret.Labels).To(HaveKeyWithValue("gardener.cloud/purpose", "global-monitoring-secret-replica"))
				Expect(secret.Type).To(Equal(globalMonitoringSecret.Type))
				Expect(secret.Immutable).To(Equal(globalMonitoringSecret.Immutable))
				for k, v := range globalMonitoringSecret.Data {
					Expect(secret.Data).To(HaveKeyWithValue(k, v), "have key "+k+" with value "+string(v))
				}
				hashedPassword := strings.TrimPrefix(string(secret.Data["auth"]), string(secret.Data["username"])+":")
				Expect(bcrypt.CompareHashAndPassword([]byte(hashedPassword), secret.Data["password"])).To(Succeed())
			}

			secret, err := ReplicateGlobalMonitoringSecret(ctx, fakeClient, prefix, namespace, globalMonitoringSecret)
			Expect(err).NotTo(HaveOccurred())
			assertions(secret)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			assertions(secret)
		})
	})

	Describe("#MutateObjectsInSecretData", func() {
		var (
			secretData map[string][]byte

			configMap  *corev1.ConfigMap
			secret     *corev1.Secret
			deployment *appsv1.Deployment
		)

		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "res1",
					Namespace: "garden",
				},
				Data: map[string]string{
					"key1": "key2",
				},
			}

			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "res2",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"key": []byte("secret"),
				},
			}

			deployment = &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "res3",
					Namespace: "garden",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "container1", Image: "image1"},
								{Name: "container2", Image: "image2"},
							},
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			secretData = map[string][]byte{}

			var names []string

			for _, obj := range []client.Object{configMap, secret, deployment} {
				name := obj.GetName()

				Expect(names).NotTo(ContainElement(name), "object name is already in use: "+name)
				names = append(names, name)

				data, err := yaml.Marshal(obj)
				Expect(err).ToNot(HaveOccurred(), "error marshalling "+obj.GetObjectKind().GroupVersionKind().String())
				secretData[name] = data
			}
		})

		It("should add an annotation value to all objects in the garden namespace", func() {
			Expect(MutateObjectsInSecretData(secretData, "garden", []string{"", "apps"}, func(obj runtime.Object) error {
				obj.(client.Object).SetAnnotations(map[string]string{"foo": "bar"})
				return nil
			})).To(Succeed())

			for _, obj := range []client.Object{configMap, deployment} {
				obj.SetAnnotations(map[string]string{"foo": "bar"})
			}

			actualConfigMap := &corev1.ConfigMap{}
			Expect(yaml.Unmarshal(secretData["res1"], &actualConfigMap)).To(Succeed())
			Expect(actualConfigMap).To(Equal(configMap))

			actualSecret := &corev1.Secret{}
			Expect(yaml.Unmarshal(secretData["res2"], &actualSecret)).To(Succeed())
			Expect(actualSecret).To(Equal(secret))

			actualDeployment := &appsv1.Deployment{}
			Expect(yaml.Unmarshal(secretData["res3"], &actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(Equal(deployment))
		})

		It("should add an annotation to objects of a specific group only", func() {
			Expect(MutateObjectsInSecretData(secretData, "garden", []string{"apps"}, func(obj runtime.Object) error {
				obj.(client.Object).SetAnnotations(map[string]string{"foo": "bar"})
				return nil
			})).To(Succeed())

			actualConfigMap := &corev1.ConfigMap{}
			Expect(yaml.Unmarshal(secretData["res1"], &actualConfigMap)).To(Succeed())
			Expect(actualConfigMap).To(Equal(configMap))

			actualSecret := &corev1.Secret{}
			Expect(yaml.Unmarshal(secretData["res2"], &actualSecret)).To(Succeed())
			Expect(actualSecret).To(Equal(secret))

			deployment.SetAnnotations(map[string]string{"foo": "bar"})
			actualDeployment := &appsv1.Deployment{}
			Expect(yaml.Unmarshal(secretData["res3"], &actualDeployment)).To(Succeed())
			Expect(actualDeployment).To(Equal(deployment))
		})

		It("should fail because mutation errored", func() {
			Expect(MutateObjectsInSecretData(secretData, "garden", []string{"apps"}, func(_ runtime.Object) error {
				return fmt.Errorf("some error")
			})).To(MatchError("some error"))
		})

		It("should have a stable output", func() {
			secretResource := `
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    reference.resources.gardener.cloud/configmap-32c4dfab: oidc-apps-controller-imagevector-overwrite
    reference.resources.gardener.cloud/secret-8d3ae69b: oidc-apps-controller
    reference.resources.gardener.cloud/secret-795f7ca6: garden-access-extension
    reference.resources.gardener.cloud/secret-83438e60: generic-garden-kubeconfig-a1b02908
  creationTimestamp: null
  name: foo
  namespace: bar

---
`

			secretData := map[string][]byte{"res1": []byte(secretResource)}

			Expect(MutateObjectsInSecretData(secretData, deployment.Namespace, []string{"apps"}, func(_ runtime.Object) error {
				return nil
			})).To(Succeed())

			Expect(string(secretData["res1"])).To(Equal(secretResource))
		})
	})

	Describe("#ObjectsInSecretData", func() {
		var (
			secretData map[string][]byte

			configMap *corev1.ConfigMap
			secret    *corev1.Secret
		)

		BeforeEach(func() {
			configMap = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap",
					Namespace: "garden",
				},
				Data: map[string]string{
					"key1": "key2",
				},
			}

			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					"key": []byte("secret"),
				},
			}
		})

		JustBeforeEach(func() {
			secretData = make(map[string][]byte)

			var names []string

			for _, obj := range []client.Object{configMap, secret} {
				name := obj.GetName()

				Expect(names).NotTo(ContainElement(name), "object name is already in use: "+name)
				names = append(names, name)

				data, err := yaml.Marshal(obj)
				Expect(err).ToNot(HaveOccurred(), "error marshalling "+obj.GetObjectKind().GroupVersionKind().String())
				secretData[name] = data
			}
		})

		It("should return all objects in the secret data", func() {
			objects, err := ObjectsInSecretData(secretData)
			Expect(err).NotTo(HaveOccurred())
			Expect(objects).To(HaveLen(2))

			for _, object := range objects {
				if object.(client.Object).GetName() == "configmap" {
					returnedConfigMap := &corev1.ConfigMap{}
					Expect(kubernetesscheme.Scheme.Convert(object, returnedConfigMap, nil)).To(Succeed())
					returnedConfigMap.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
					Expect(returnedConfigMap).To(Equal(configMap))
				}

				if object.(client.Object).GetName() == "secret" {
					returnedSecret := &corev1.Secret{}
					Expect(kubernetesscheme.Scheme.Convert(object, returnedSecret, nil)).To(Succeed())
					returnedSecret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
					Expect(returnedSecret).To(Equal(secret))
				}
			}
		})
	})
})
