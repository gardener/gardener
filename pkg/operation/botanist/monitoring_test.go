// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockcoredns "github.com/gardener/gardener/pkg/component/coredns/mock"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/mock"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubeapiserver/mock"
	mockkubecontrollermanager "github.com/gardener/gardener/pkg/component/kubecontrollermanager/mock"
	mockkubeproxy "github.com/gardener/gardener/pkg/component/kubeproxy/mock"
	mockkubescheduler "github.com/gardener/gardener/pkg/component/kubescheduler/mock"
	mockkubestatemetrics "github.com/gardener/gardener/pkg/component/kubestatemetrics/mock"
	mockresourcemanager "github.com/gardener/gardener/pkg/component/resourcemanager/mock"
	mockvpa "github.com/gardener/gardener/pkg/component/vpa/mock"
	mockvpnseedserver "github.com/gardener/gardener/pkg/component/vpnseedserver/mock"
	mockvpnshoot "github.com/gardener/gardener/pkg/component/vpnshoot/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Monitoring", func() {
	var (
		ctrl *gomock.Controller

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		chartApplier kubernetes.ChartApplier
		sm           secretsmanager.Interface

		mockEtcdMain              *mocketcd.MockInterface
		mockEtcdEvents            *mocketcd.MockInterface
		mockKubeAPIServer         *mockkubeapiserver.MockInterface
		mockKubeScheduler         *mockkubescheduler.MockInterface
		mockKubeControllerManager *mockkubecontrollermanager.MockInterface
		mockKubeStateMetrics      *mockkubestatemetrics.MockInterface
		mockCoreDNS               *mockcoredns.MockInterface
		mockKubeProxy             *mockkubeproxy.MockInterface
		mockVPNSeedServer         *mockvpnseedserver.MockInterface
		mockVPNShoot              *mockvpnshoot.MockInterface
		mockResourceManager       *mockresourcemanager.MockInterface
		mockVPA                   *mockvpa.MockInterface

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

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, appsv1.SchemeGroupVersion})
		renderer := chartrenderer.NewWithServerVersion(&version.Info{GitVersion: "1.2.3"})
		chartApplier = kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(seedClient, mapper))

		seedClientSet = fake.NewClientSetBuilder().
			WithClient(seedClient).
			WithChartApplier(chartApplier).
			WithRESTConfig(&rest.Config{}).
			Build()
		sm = fakesecretsmanager.New(seedClient, seedNamespace)

		mockEtcdMain = mocketcd.NewMockInterface(ctrl)
		mockEtcdEvents = mocketcd.NewMockInterface(ctrl)
		mockKubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		mockKubeScheduler = mockkubescheduler.NewMockInterface(ctrl)
		mockKubeControllerManager = mockkubecontrollermanager.NewMockInterface(ctrl)
		mockKubeStateMetrics = mockkubestatemetrics.NewMockInterface(ctrl)
		mockCoreDNS = mockcoredns.NewMockInterface(ctrl)
		mockKubeProxy = mockkubeproxy.NewMockInterface(ctrl)
		mockVPNSeedServer = mockvpnseedserver.NewMockInterface(ctrl)
		mockVPNShoot = mockvpnshoot.NewMockInterface(ctrl)
		mockResourceManager = mockresourcemanager.NewMockInterface(ctrl)
		mockVPA = mockvpa.NewMockInterface(ctrl)

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
							EtcdMain:              mockEtcdMain,
							EtcdEvents:            mockEtcdEvents,
							KubeAPIServer:         mockKubeAPIServer,
							KubeScheduler:         mockKubeScheduler,
							KubeControllerManager: mockKubeControllerManager,
							KubeStateMetrics:      mockKubeStateMetrics,
							ResourceManager:       mockResourceManager,
							VerticalPodAutoscaler: mockVPA,
							VPNSeedServer:         mockVPNSeedServer,
						},
						SystemComponents: &shootpkg.SystemComponents{
							CoreDNS:   mockCoreDNS,
							KubeProxy: mockKubeProxy,
							VPNShoot:  mockVPNShoot,
						},
					},
				},
				ImageVector: imagevector.ImageVector{
					{Name: "plutono"},
					{Name: "prometheus"},
					{Name: "configmap-reloader"},
					{Name: "blackbox-exporter"},
					{Name: "kube-state-metrics"},
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

	Describe("#DeploySeedPlutono", func() {
		It("should generate two ingress secrets", func() {
			defer test.WithVar(&ChartsPath, filepath.Join("..", "..", "..", "charts"))()

			Expect(botanist.DeploySeedPlutono(ctx)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(seedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
				"name":       "observability-ingress-users",
				"managed-by": "secrets-manager",
			})).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
			Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
		})

		It("should sync the ingress credentials for the users observability to the garden project namespace", func() {
			defer test.WithVar(&ChartsPath, filepath.Join("..", "..", "..", "charts"))()

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())

			Expect(botanist.DeploySeedPlutono(ctx)).To(Succeed())

			secret := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), secret)).To(Succeed())
			Expect(secret.Annotations).To(HaveKeyWithValue("url", "https://gu-foo--bar."))
			Expect(secret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "monitoring"))
			Expect(secret.Data).To(And(HaveKey("username"), HaveKey("password"), HaveKey("auth")))
		})

		It("should cleanup the secrets when shoot purpose is changed", func() {
			defer test.WithVar(&ChartsPath, filepath.Join("..", "..", "..", "charts"))()

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())

			Expect(botanist.DeploySeedPlutono(ctx)).To(Succeed())
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(Succeed())
			Expect(*botanist.Shoot.GetInfo().Spec.Purpose == shootPurposeEvaluation).To(BeTrue())

			botanist.Shoot.Purpose = shootPurposeTesting
			Expect(botanist.DeploySeedPlutono(ctx)).To(Succeed())

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".monitoring"), &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
