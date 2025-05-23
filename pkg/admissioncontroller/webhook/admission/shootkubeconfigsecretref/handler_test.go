// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootkubeconfigsecretref_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/shootkubeconfigsecretref"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Handler", func() {
	var (
		ctx        = context.TODO()
		log        logr.Logger
		fakeClient client.Client
		warning    admission.Warnings
		err        error

		handler *Handler

		secret         *corev1.Secret
		shoot          *gardencorev1beta1.Shoot
		secretName     = "test-kubeconfig"
		shootName      = "fake-shoot-name"
		shootNamespace = "fake-cm-namespace"
	)

	BeforeEach(func() {
		log = logr.Discard()
		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Name: secretName}})
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		handler = &Handler{Logger: log, Client: fakeClient}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: shootNamespace,
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "Shoot",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{
							{
								Name: "PodNodeSelector",
							},
						},
					},
				},
			},
		}
	})

	It("should pass because no shoot references secret", func() {
		warning, err = handler.ValidateUpdate(ctx, nil, secret)
		Expect(warning).To(BeNil())
		Expect(err).NotTo(HaveOccurred())
	})

	Context("admission plugin kubeconfig secrets", func() {
		It("should fail because some shoot references secret and kubeconfig is removed from secret", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{
				Name:                 "plugin-1",
				KubeconfigSecretName: ptr.To(secret.Name),
			}}
			shoot1 := shoot.DeepCopy()
			shoot1.Name = "test-shoot"
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())

			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("Secret \"test-kubeconfig\" is forbidden: data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [fake-shoot-name, test-shoot]")))
		})

		It("should fail because some shoot references secret and kubeconfig is set to empty", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{
				Name:                 "plugin-1",
				KubeconfigSecretName: ptr.To(secret.Name),
			}}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			secret.Data = map[string][]byte{"kubeconfig": {}}
			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("Secret \"test-kubeconfig\" is forbidden: data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [fake-shoot-name]")))
		})

		It("should pass because secret contain kubeconfig and it is not empty", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{
				Name:                 "plugin-1",
				KubeconfigSecretName: ptr.To(secret.Name),
			}}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			secret.Data = map[string][]byte{"kubeconfig": []byte("secret-data")}
			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})

	Context("structured authorization kubeconfig secrets", func() {
		It("should fail because some shoot references secret and kubeconfig is removed from secret", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{
				Kubeconfigs: []gardencorev1beta1.AuthorizerKubeconfigReference{{SecretName: secret.Name}},
			}
			shoot1 := shoot.DeepCopy()
			shoot1.Name = "test-shoot"
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())

			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("Secret \"test-kubeconfig\" is forbidden: data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [fake-shoot-name, test-shoot]")))
		})

		It("should fail because some shoot references secret and kubeconfig is set to empty", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{
				Kubeconfigs: []gardencorev1beta1.AuthorizerKubeconfigReference{{SecretName: secret.Name}},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			secret.Data = map[string][]byte{"kubeconfig": {}}
			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("Secret \"test-kubeconfig\" is forbidden: data kubeconfig can't be removed from secret or set to empty because secret is in use by shoots: [fake-shoot-name]")))
		})

		It("should pass because secret contain kubeconfig and it is not empty", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{
				Kubeconfigs: []gardencorev1beta1.AuthorizerKubeconfigReference{{SecretName: secret.Name}},
			}
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

			secret.Data = map[string][]byte{"kubeconfig": []byte("secret-data")}
			warning, err = handler.ValidateUpdate(ctx, nil, secret)
			Expect(warning).To(BeNil())
			Expect(err).To(Succeed())
		})
	})

	It("should do nothing for on Create operation", func() {
		warning, err = handler.ValidateCreate(ctx, secret)
		Expect(warning).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})

	It("should do nothing for on Delete operation", func() {
		warning, err = handler.ValidateDelete(ctx, secret)
		Expect(warning).To(BeNil())
		Expect(err).ToNot(HaveOccurred())
	})
})
