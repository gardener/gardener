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

package admissionpluginsecret_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/admissionpluginsecret"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Handler", func() {
	var (
		ctx        = context.TODO()
		log        logr.Logger
		fakeClient client.Client

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
		Expect(handler.ValidateUpdate(ctx, nil, secret)).To(BeNil())
	})

	It("should fail because some shoot references secret and kubeconfig is removed from secret", func() {
		shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
			{
				Name:                 "plugin-1",
				KubeconfigSecretName: pointer.String(secret.Name),
			},
		}
		shoot1 := shoot.DeepCopy()
		shoot1.Name = "test-shoot"
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())

		Expect(handler.ValidateUpdate(ctx, nil, secret)).To(MatchError(ContainSubstring("Secret \"test-kubeconfig\" is forbidden: data kubeconfig can't be removed from secret because secret is in use by shoots: [fake-shoot-name test-shoot]")))
	})

	It("should pass because secret contains data kubeconfig", func() {
		shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
			{
				Name:                 "plugin-1",
				KubeconfigSecretName: pointer.String(secret.Name),
			},
		}
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		secret.Data = map[string][]byte{"kubeconfig": {}}
		Expect(handler.ValidateUpdate(ctx, nil, secret)).To(BeNil())
	})
})
