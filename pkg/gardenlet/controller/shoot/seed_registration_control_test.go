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

package shoot

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Seed registration control", func() {
	Describe("deployGardenlet", func() {
		var (
			ctx context.Context

			gardenClient, seedClient, shootClient kubernetes.Interface

			ctrl             *gomock.Controller
			mockGardenClient *mockclient.MockClient
			mockShootClient  *mockclient.MockClient

			gardenletConfig   *config.GardenletConfiguration
			shootedSeedConfig *gardencorev1beta1helper.ShootedSeed
			shoot             *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			ctx = context.TODO()

			ctrl = gomock.NewController(GinkgoT())

			mockGardenClient = mockclient.NewMockClient(ctrl)
			gardenClient = fake.NewClientSetBuilder().WithClient(mockGardenClient).Build()

			seedClient = fake.NewClientSetBuilder().Build()

			mockShootClient = mockclient.NewMockClient(ctrl)
			shootClient = fake.NewClientSetBuilder().WithClient(mockShootClient).Build()

			gardenletConfig = &config.GardenletConfiguration{
				SeedConfig: &config.SeedConfig{SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "test-seed"},
				}},
			}
			shootedSeedConfig = &gardencorev1beta1helper.ShootedSeed{}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "test-shoot"},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: "test-secretbinding",
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should successfully convert config", func() {
			mockShootClient.EXPECT().Get(gomock.Any(), kutil.Key("garden", "gardenlet-kubeconfig"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil)
			mockGardenClient.EXPECT().Get(gomock.Any(), kutil.Key("garden", shoot.Spec.SecretBindingName), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBinding{})).
				Return(apierrors.NewNotFound(schema.GroupResource{Group: "core.gardener.cloud", Resource: "secretbindings"}, shoot.Spec.SecretBindingName))

			err := deployGardenlet(ctx, gardenClient, seedClient, shootClient, shoot, shootedSeedConfig, nil, gardenletConfig)
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError(ContainSubstring("unknown conversion")))
			Expect(err).To(MatchError(ContainSubstring("\"test-secretbinding\" not found")))
		})
	})
})
