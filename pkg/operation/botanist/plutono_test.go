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

package botanist_test

import (
	"context"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockplutono "github.com/gardener/gardener/pkg/component/plutono/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Plutono", func() {
	var (
		ctrl *gomock.Controller

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		sm secretsmanager.Interface

		mockPlutono *mockplutono.MockInterface

		botanist *Botanist

		ctx              = context.TODO()
		projectNamespace = "garden-foo"
		seedNamespace    = "shoot--foo--bar"
		shootName        = "bar"

		shootPurposeEvaluation = gardencorev1beta1.ShootPurposeEvaluation
		shootPurposeTesting    = gardencorev1beta1.ShootPurposeTesting
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		seedClientSet = fake.NewClientSetBuilder().
			WithClient(seedClient).
			WithRESTConfig(&rest.Config{}).
			Build()
		sm = fakesecretsmanager.New(seedClient, seedNamespace)

		By("Create secrets managed outside of this function for whose secretsmanager.Get() will be called")
		Expect(seedClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "observability-ingress-users",
				Namespace:   seedNamespace,
				Labels:      map[string]string{"gardener.cloud/role": "monitoring"},
				Annotations: map[string]string{"url": "https://gu-foo--bar.example.com"},
			},
			Data: map[string][]byte{"username": {}, "password": {}, "auth": {}},
		})).To(Succeed())

		mockPlutono = mockplutono.NewMockInterface(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:   gardenClient,
				SeedClientSet:  seedClientSet,
				SecretsManager: sm,
				Config:         &config.GardenletConfiguration{},
				Garden: &garden.Garden{
					Project: &gardencorev1beta1.Project{},
				},
				Seed:                &seedpkg.Seed{},
				SeedNamespaceObject: &corev1.Namespace{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Networks: &shootpkg.Networks{
						Pods:     &net.IPNet{},
						Services: &net.IPNet{},
					},
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							Plutono: mockPlutono,
						},
					},
				},
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: pointer.String("1.2.3"),
			},
		})

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				Purpose: &shootPurposeEvaluation,
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployPlutono", func() {
		It("should successfully deploy plutono sync the ingress credentials for the users observability to the garden project namespace", func() {
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())
			mockPlutono.EXPECT().Deploy(ctx)
			Expect(botanist.DeployPlutono(ctx)).To(Succeed())

			secret := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), secret)).To(Succeed())
			Expect(secret.Annotations).To(HaveKeyWithValue("url", "https://gu-foo--bar."))
			Expect(secret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "monitoring"))
			Expect(secret.Data).To(And(HaveKey("username"), HaveKey("password"), HaveKey("auth")))
		})

		It("should cleanup the secrets when shoot purpose is changed", func() {
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())
			mockPlutono.EXPECT().Deploy(ctx)
			Expect(botanist.DeployPlutono(ctx)).To(Succeed())
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(Succeed())
			Expect(*botanist.Shoot.GetInfo().Spec.Purpose == shootPurposeEvaluation).To(BeTrue())

			botanist.Shoot.Purpose = shootPurposeTesting
			mockPlutono.EXPECT().Destroy(ctx)
			Expect(botanist.DeployPlutono(ctx)).To(Succeed())

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
