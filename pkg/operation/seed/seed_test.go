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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/pkg/operation/seed"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("seed", func() {
	var (
		ctrl           *gomock.Controller
		restMockClient *mock.MockInterface
		runtimeClient  *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		restMockClient = mock.NewMockInterface(ctrl)
		runtimeClient = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetWildcardCertificate", func() {
		It("should return no wildcard certificate secret", func() {
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(BeNil())
		})

		It("should return a wildcard certificate secret", func() {
			secretList := &corev1.SecretList{
				Items: []corev1.Secret{
					{},
				},
			}
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ client.ListOption, _ client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

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
			runtimeClient.EXPECT().List(context.TODO(), gomock.AssignableToTypeOf(&corev1.SecretList{}), client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert}).DoAndReturn(
				func(_ context.Context, secrets *corev1.SecretList, _ client.ListOption, _ client.ListOption) error {
					*secrets = *secretList
					return nil
				})

			secret, err := GetWildcardCertificate(context.TODO(), runtimeClient)

			Expect(err).To(HaveOccurred())
			Expect(secret).To(BeNil())
		})
	})

	Describe("#GetFluentdReplicaCount", func() {
		It("should return single replica when stateful set does not exist", func() {
			restMockClient.EXPECT().Client().Return(runtimeClient)
			runtimeClient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key client.ObjectKey, _ *appsv1.StatefulSet) error {
				return errors.NewNotFound(appsv1.Resource("StatefulSet"), key.Name)
			})

			replicas, err := GetFluentdReplicaCount(restMockClient)

			Expect(err).NotTo(HaveOccurred())
			var expectedReplicas int32 = 1
			Expect(replicas).To(Equal(expectedReplicas))
		})

		It("should get stateful set replicas", func() {
			var expectedReplicas int32 = 3
			restMockClient.EXPECT().Client().Return(runtimeClient)
			runtimeClient.EXPECT().Get(context.TODO(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ client.ObjectKey, statefulSet *appsv1.StatefulSet) error {
				statefulSet.Spec.Replicas = &expectedReplicas
				return nil
			})

			replicas, err := GetFluentdReplicaCount(restMockClient)

			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(expectedReplicas))
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
