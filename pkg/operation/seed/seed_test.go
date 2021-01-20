// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"
	"errors"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/operation/seed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("seed", func() {
	var (
		ctx              context.Context
		ctrl             *gomock.Controller
		runtimeClient    *mockclient.MockClient
		kubernetesClient *mockkubernetes.MockInterface
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		runtimeClient = mockclient.NewMockClient(ctrl)
		kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetWildcardCertificate", func() {
		It("should return no wildcard certificate secret", func() {
			runtimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert})

			secret, err := GetWildcardCertificate(ctx, runtimeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(BeNil())
		})

		It("should return a wildcard certificate secret", func() {
			secretList := &corev1.SecretList{
				Items: []corev1.Secret{
					{},
				},
			}
			runtimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ client.ListOption, _ client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(ctx, runtimeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(*secret).To(Equal(secretList.Items[0]))
		})

		It("should return an error because more than one wildcard secrets is found", func() {
			secretList := &corev1.SecretList{
				Items: []corev1.Secret{
					{},
					{},
				},
			}
			runtimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ client.ListOption, _ client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(ctx, runtimeClient)

			Expect(err).To(HaveOccurred())
			Expect(secret).To(BeNil())
		})
	})

	Describe("#DeleteDNSProvider", func() {
		var (
			dnsProvider    *dnsv1alpha1.DNSProvider
			providerSecret *corev1.Secret
			fakeErr        error
		)
		BeforeEach(func() {
			dnsProvider = &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "seed",
				},
			}
			providerSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "dnsprovider-seed",
				},
			}
			kubernetesClient.EXPECT().Client().Return(runtimeClient)
			fakeErr = errors.New("fake")
		})

		It("should delete DSNProvider and Secret", func() {
			runtimeClient.EXPECT().Delete(ctx, dnsProvider).Return(nil)
			runtimeClient.EXPECT().Delete(ctx, providerSecret).Return(nil)
			err := DeleteDNSProvider(ctx, kubernetesClient.Client())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should propagate the error if deletion fails on dnsProvider", func() {
			runtimeClient.EXPECT().Delete(ctx, dnsProvider).Return(fakeErr)
			err := DeleteDNSProvider(ctx, kubernetesClient.Client())
			Expect(err).To(Equal(fakeErr))
		})
		It("should propagate the error if deletion fails on providerSecret", func() {
			runtimeClient.EXPECT().Delete(ctx, dnsProvider).Return(nil)
			runtimeClient.EXPECT().Delete(ctx, providerSecret).Return(fakeErr)
			err := DeleteDNSProvider(ctx, kubernetesClient.Client())
			Expect(err).To(Equal(fakeErr))
		})
	})

	Describe("#DeleteIngressDNSEntry", func() {
		var (
			chartApplier *mockkubernetes.MockChartApplier
			seed         *Seed
			entry        *dnsv1alpha1.DNSEntry
		)
		BeforeEach(func() {
			seed = &Seed{Info: &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "fakeSeed"}}}
			entry = &dnsv1alpha1.DNSEntry{ObjectMeta: metav1.ObjectMeta{Name: "ingress", Namespace: v1beta1constants.GardenNamespace}}
			chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
			kubernetesClient.EXPECT().ChartApplier().Return(chartApplier)
			kubernetesClient.EXPECT().Client().Return(runtimeClient)
			logger.Logger = logger.NewNopLogger()
		})

		It("should delete the DNS Entry", func() {
			runtimeClient.EXPECT().Delete(ctx, entry).Return(nil)
			err := DeleteIngressDNSEntry(ctx, kubernetesClient, seed)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should propagate the error if deletion fails", func() {
			fakeErr := errors.New("fake")
			runtimeClient.EXPECT().Delete(ctx, entry).Return(fakeErr)
			err := DeleteIngressDNSEntry(ctx, kubernetesClient, seed)
			Expect(err).To(Equal(fakeErr))
		})
	})

	Describe("#CopyDNSProviderSecretToSeed", func() {
		var (
			gardenRuntimeClient    *mockclient.MockClient
			cloudProviderSecretKey = kutil.Key("cloudprovider-secret-ns", "cloudprovider-secret-name")
			dnsProviderSecretKey   = kutil.Key(v1beta1constants.GardenNamespace, "dnsprovider-seed")
		)
		BeforeEach(func() {
			gardenRuntimeClient = mockclient.NewMockClient(ctrl)
		})

		It("should copy the provided secret", func() {
			gardenRuntimeClient.EXPECT().Get(ctx, cloudProviderSecretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
				obj.Type = "someType"
				obj.Data = map[string][]byte{"somedata": []byte{}}
				return nil
			})
			runtimeClient.EXPECT().Get(ctx, dnsProviderSecretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil)
			runtimeClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
				secret, ok := obj.(*corev1.Secret)
				Expect(ok).To(BeTrue())
				Expect(secret.Name).To(Equal(dnsProviderSecretKey.Name))
				Expect(secret.Namespace).To(Equal(dnsProviderSecretKey.Namespace))
				Expect(secret.Data).To(Equal(map[string][]byte{"somedata": []byte{}}))
				Expect(secret.Type).To(Equal(corev1.SecretType("someType")))
				return nil
			})

			err := CopyDNSProviderSecretToSeed(ctx, gardenRuntimeClient, runtimeClient, cloudProviderSecretKey)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail if the provider secret is not found", func() {
			gardenRuntimeClient.EXPECT().Get(ctx, cloudProviderSecretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "baz", Resource: "bar"}, "foo"))
			err := CopyDNSProviderSecretToSeed(ctx, gardenRuntimeClient, runtimeClient, cloudProviderSecretKey)
			Expect(err).To(BeNotFoundError())
		})
	})

	Describe("#GetValidVolumeSize", func() {
		It("should return the size because no minimum size was set", func() {
			var (
				size = "20Gi"
				seed = &Seed{
					Info: &gardencorev1beta1.Seed{
						Spec: gardencorev1beta1.SeedSpec{
							Volume: nil,
						},
					},
				}
			)

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})

		It("should return the minimum size because the given value is smaller", func() {
			var (
				size                = "20Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{
					Info: &gardencorev1beta1.Seed{
						Spec: gardencorev1beta1.SeedSpec{
							Volume: &gardencorev1beta1.SeedVolume{
								MinimumSize: &minimumSizeQuantity,
							},
						},
					},
				}
			)

			Expect(seed.GetValidVolumeSize(size)).To(Equal(minimumSize))
		})

		It("should return the given value size because the minimum size is smaller", func() {
			var (
				size                = "30Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{
					Info: &gardencorev1beta1.Seed{
						Spec: gardencorev1beta1.SeedSpec{
							Volume: &gardencorev1beta1.SeedVolume{
								MinimumSize: &minimumSizeQuantity,
							},
						},
					},
				}
			)

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})
	})
})
