// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference_test

import (
	"context"
	"errors"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/reference"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Shoot References", func() {
	const (
		shootNamespace = "shoot--foo--bar"
		shootName      = "bar"
	)

	var (
		ctx            context.Context
		namespacedName types.NamespacedName
		reconciler     reconcile.Reconciler
		shoot          gardencorev1beta1.Shoot
		gardenClient   *mockclient.MockClient
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl := gomock.NewController(GinkgoT())
		gardenClient = mockclient.NewMockClient(ctrl)
		namespacedName = types.NamespacedName{
			Namespace: shootNamespace,
			Name:      shootName,
		}

		shoot = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
		}
	})

	JustBeforeEach(func() {
		reconciler = &Reconciler{Client: gardenClient}
	})

	Context("Common controller tests", func() {
		It("should do nothing because shoot in request cannot be found", func() {
			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(apierrors.NewNotFound(gardencorev1beta1.SchemeGroupVersion.WithResource("shoots").GroupResource(), namespacedName.Name))

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(emptyResult()))
		})

		It("should error because shoot in request cannot be requested", func() {
			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(errors.New("foo"))

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(emptyResult()))
		})
	})

	Context("DNS secret reference test", func() {
		var (
			secrets []corev1.Secret
		)

		BeforeEach(func() {
			secrets = []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-3",
					Namespace: shootNamespace},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = []corev1.ConfigMap{}
					return nil
				})
		})

		It("should not add finalizers because shoot does not define a DNS section", func() {
			shoot.Spec.DNS = nil

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should not add finalizers because shoot does not refer to any secret", func() {
			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.String("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.String("managed-dns")},
				},
			}

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should add finalizer to shoot and secrets", func() {
			secretName := secrets[0].Name
			secretName2 := secrets[1].Name
			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.String("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.String("managed-dns"), SecretName: pointer.String(secretName)},
					{Type: pointer.String("managed-dns2"), SecretName: pointer.String(secretName2)},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			var (
				m              sync.Mutex
				updatedSecrets []*corev1.Secret
			)
			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(secrets[0].Namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[0]
					return nil
				})
			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(secrets[1].Namespace, secretName2), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[1]
					return nil
				})
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(ConsistOf(Equal(v1beta1constants.ReferenceProtectionFinalizerName)))
			Expect(updatedSecrets).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(v1beta1constants.ReferenceProtectionFinalizerName),
						"Name":       Equal(secretName),
					}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(v1beta1constants.ReferenceProtectionFinalizerName),
						"Name":       Equal(secretName2),
					}),
				})),
			))
		})

		It("should remove finalizer from shoot and secret because shoot is in deletion", func() {
			secretName := secrets[0].Name
			secrets[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.String("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.String("managed-dns"), SecretName: pointer.String(secretName)},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			var updatedSecret *corev1.Secret
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					updatedSecret = secret
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})

		It("should remove finalizer only from shoot because secret is still referenced by another shoot", func() {
			secretName := secrets[0].Name
			secrets[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			dnsProvider := gardencorev1beta1.DNSProvider{Type: pointer.String("managed-dns"), SecretName: pointer.String(secretName)}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain:    pointer.String("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{dnsProvider},
			}

			shoot2 := gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar2",
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{dnsProvider},
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot, shoot2)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedShoot.Name).To(Equal(shoot.Name))
		})

		It("should remove finalizer from secret because it is not referenced any more", func() {
			secretName := secrets[1].Name
			secrets[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}
			secrets[1].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Domain: pointer.String("shoot.example.com"),
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.String("managed-dns"), SecretName: pointer.String(secrets[1].Name)},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[1]
					return nil
				})

			var updatedSecret *corev1.Secret
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					updatedSecret = secret
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})
	})

	Context("Audit policy ConfigMap reference test", func() {
		var configMaps []corev1.ConfigMap

		BeforeEach(func() {
			configMaps = []corev1.ConfigMap{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-1",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-2",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-3",
					Namespace: shootNamespace},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{}
					return nil
				})
		})

		It("should not add finalizers because shoot does not define an audit config section", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = nil

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should add finalizer to shoot and configmap", func() {
			configMapName := configMaps[1].Name
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(configMaps[1].Namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = configMaps[1]
					return nil
				})

			var updatedConfigMap *corev1.ConfigMap
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, configMap *corev1.ConfigMap, _ client.Patch, _ ...client.PatchOption) error {
					updatedConfigMap = configMap
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(ConsistOf(Equal(v1beta1constants.ReferenceProtectionFinalizerName)))
			Expect(updatedConfigMap).To(PointTo(
				MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(v1beta1constants.ReferenceProtectionFinalizerName),
						"Name":       Equal(configMapName),
					}),
				})),
			)
		})

		It("should remove finalizer from shoot and configmap because shoot is in deletion", func() {
			configMapName := configMaps[0].Name
			configMaps[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			var updatedConfigMap *corev1.ConfigMap
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, cm *corev1.ConfigMap, _ client.Patch, _ ...client.PatchOption) error {
					updatedConfigMap = cm
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.ObjectMeta.Name).To(Equal(configMaps[0].Name))
		})

		It("should remove finalizer only from shoot because configmap is still referenced by another shoot", func() {
			configMapName := configMaps[0].Name
			configMaps[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			apiServerConfig := &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			shoot.Spec.Kubernetes.KubeAPIServer = apiServerConfig

			shoot2 := gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar2",
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: apiServerConfig,
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot, shoot2)
					return nil
				}).Times(2)

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedShoot.Name).To(Equal(shoot.Name))
		})

		It("should remove finalizer from configmap because it is not referenced any more", func() {
			configMapName := configMaps[1].Name
			configMaps[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}
			configMaps[1].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				AuditConfig: &gardencorev1beta1.AuditConfig{
					AuditPolicy: &gardencorev1beta1.AuditPolicy{
						ConfigMapRef: &corev1.ObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				}).Times(2)

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = append(list.Items, configMaps...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, cm *corev1.ConfigMap, _ ...client.GetOption) error {
					*cm = configMaps[1]
					return nil
				})

			var updatedConfigMap *corev1.ConfigMap
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, configMap *corev1.ConfigMap, _ client.Patch, _ ...client.PatchOption) error {
					updatedConfigMap = configMap
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedConfigMap.Finalizers).To(BeEmpty())
			Expect(updatedConfigMap.ObjectMeta.Name).To(Equal(configMaps[0].Name))
		})
	})

	Context("Shoot resources reference test", func() {
		var (
			secrets []corev1.Secret
		)

		BeforeEach(func() {
			secrets = []corev1.Secret{
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: shootNamespace},
				},
				{ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: shootNamespace},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.ConfigMapList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					list.Items = []corev1.ConfigMap{}
					return nil
				})
		})

		It("should not add finalizers because shoot does not refer to any secret in resources", func() {
			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(shoot.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should add finalizer to secrets referneced in resources", func() {
			secretName := secrets[0].Name
			secretName2 := secrets[1].Name
			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "resource-1",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secrets[0].Name,
					},
				},
				{
					Name: "resource-2",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secrets[1].Name,
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			var (
				m              sync.Mutex
				updatedSecrets []*corev1.Secret
			)
			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(secrets[0].Namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[0]
					return nil
				})
			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(secrets[1].Namespace, secretName2), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[1]
					return nil
				})
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					defer m.Unlock()
					m.Lock()
					updatedSecrets = append(updatedSecrets, secret)
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(ConsistOf(Equal(v1beta1constants.ReferenceProtectionFinalizerName)))
			Expect(updatedSecrets).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(v1beta1constants.ReferenceProtectionFinalizerName),
						"Name":       Equal(secretName),
					}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{
						"Finalizers": ConsistOf(v1beta1constants.ReferenceProtectionFinalizerName),
						"Name":       Equal(secretName2),
					}),
				})),
			))
		})

		It("should remove finalizer from secret because shoot is in deletion", func() {
			secretName := secrets[0].Name
			secrets[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			now := metav1.Now()
			shoot.ObjectMeta.DeletionTimestamp = &now
			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "resource-1",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName,
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			var updatedSecret *corev1.Secret
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					updatedSecret = secret
					return nil
				})

			var updatedShoot *gardencorev1beta1.Shoot
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, shoot *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					updatedShoot = shoot
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedShoot.ObjectMeta.Finalizers).To(BeEmpty())
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})

		It("should remove finalizer from secret because it is not referenced any more", func() {
			secretName := secrets[1].Name
			secrets[0].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}
			secrets[1].Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Finalizers = []string{v1beta1constants.ReferenceProtectionFinalizerName}

			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{
				{
					Name: "resource-1",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName,
					},
				},
			}

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), client.InNamespace(shootNamespace)).DoAndReturn(
				func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					list.Items = append(list.Items, shoot)
					return nil
				})

			gardenClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(shootNamespace), UserManagedSelector).DoAndReturn(
				func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = append(list.Items, secrets...)
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), namespacedName, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*s = shoot
					return nil
				})

			gardenClient.EXPECT().Get(gomock.Any(), kubernetesutils.Key(shootNamespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
					*s = secrets[1]
					return nil
				})

			var updatedSecret *corev1.Secret
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					updatedSecret = secret
					return nil
				})

			request := reconcile.Request{NamespacedName: namespacedName}
			result, err := reconciler.Reconcile(ctx, request)

			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(emptyResult()))
			Expect(updatedSecret.Finalizers).To(BeEmpty())
			Expect(updatedSecret.ObjectMeta.Name).To(Equal(secrets[0].Name))
		})
	})
})

func emptyResult() reconcile.Result {
	return reconcile.Result{}
}
