// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Monitoring", func() {
	var (
		fakeSeedClient client.Client
		chartApplier   kubernetes.ChartApplier
		k8sSeedClient  kubernetes.Interface
		sm             secretsmanager.Interface

		botanist *Botanist

		ctx           = context.TODO()
		seedNamespace = "shoot--foo--bar"
	)

	BeforeEach(func() {
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion})
		renderer := cr.NewWithServerVersion(&version.Info{GitVersion: "1.2.3"})
		chartApplier = kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(fakeSeedClient, mapper))

		k8sSeedClient = fake.NewClientSetBuilder().WithClient(fakeSeedClient).WithChartApplier(chartApplier).Build()
		sm = fakesecretsmanager.New(fakeSeedClient, seedNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sSeedClient:  k8sSeedClient,
				SecretsManager: sm,
				Garden:         &gardenpkg.Garden{},
				Seed:           &seedpkg.Seed{},
				Shoot:          &shootpkg.Shoot{SeedNamespace: seedNamespace},
				ImageVector:    imagevector.ImageVector{{Name: "grafana"}},
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Status: gardencorev1beta1.SeedStatus{
				KubernetesVersion: pointer.String("1.2.3"),
			},
		})

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		})
	})

	Describe("#DeploySeedGrafana", func() {
		It("should generate two ingress secrets", func() {
			defer test.WithVar(&ChartsPath, filepath.Join("..", "..", "..", "charts"))()

			Expect(botanist.DeploySeedGrafana(ctx)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeSeedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
				"name":       "observability-ingress",
				"managed-by": "secrets-manager",
			})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
			Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))

			Expect(fakeSeedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
				"name":       "observability-ingress-users",
				"managed-by": "secrets-manager",
			})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
			Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
		})

		It("should delete the legacy ingress secrets", func() {
			defer test.WithVar(&ChartsPath, filepath.Join("..", "..", "..", "charts"))()

			legacySecret1 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace, Name: "monitoring-ingress-credentials"}}
			Expect(fakeSeedClient.Create(ctx, legacySecret1)).To(Succeed())

			legacySecret2 := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace, Name: "monitoring-ingress-credentials-users"}}
			Expect(fakeSeedClient.Create(ctx, legacySecret2)).To(Succeed())

			Expect(botanist.DeploySeedGrafana(ctx)).To(Succeed())

			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(legacySecret1), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(legacySecret2), &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
