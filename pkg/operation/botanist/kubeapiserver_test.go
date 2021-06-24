// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		c             client.Client
		k8sSeedClient kubernetes.Interface
		botanist      *Botanist

		ctx            = context.TODO()
		shootNamespace = "shoot--foo--bar"
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		k8sSeedClient = fake.NewClientSetBuilder().WithClient(c).Build()
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sSeedClient: k8sSeedClient,
				Shoot: &shootpkg.Shoot{
					Info:          &gardencorev1beta1.Shoot{},
					SeedNamespace: shootNamespace,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{},
					},
				},
			},
		}
	})

	Describe("#DefaultKubeAPIServer", func() {
		Describe("AutoscalingConfig", func() {
			DescribeTable("should have the expected autoscaling config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, expectedConfig kubeapiserver.AutoscalingConfig) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer()
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().Autoscaling).To(Equal(expectedConfig))
				},

				Entry("default behaviour, HVPA is disabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: false,
						MinReplicas: 1,
						MaxReplicas: 4,
					},
				),
				Entry("default behaviour, HVPA is enabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: true,
						MinReplicas: 1,
						MaxReplicas: 4,
					},
				),
				Entry("shoot purpose production",
					func() {
						botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeProduction
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: false,
						MinReplicas: 2,
						MaxReplicas: 4,
					},
				),
				Entry("shoot disables scale down",
					func() {
						botanist.Shoot.Info.Annotations = map[string]string{"alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled": "true"}
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: false,
						MinReplicas: 4,
						MaxReplicas: 4,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: false,
						MinReplicas: 1,
						MaxReplicas: 4,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: true,
						MinReplicas: 1,
						MaxReplicas: 4,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: true,
						MinReplicas: 16,
						MaxReplicas: 32,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled: false,
						MinReplicas: 16,
						MaxReplicas: 32,
						Replicas:    pointer.Int32(24),
					},
				),
			)
		})
	})

	Describe("#DeployKubeAPIServer", func() {
		var (
			ctrl          *gomock.Controller
			kubeAPIServer *mockkubeapiserver.MockInterface
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
			botanist.Shoot.Components.ControlPlane.KubeAPIServer = kubeAPIServer
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				oldGetDeployKubeAPIServerFunc := GetLegacyDeployKubeAPIServerFunc
				defer func() { GetLegacyDeployKubeAPIServerFunc = oldGetDeployKubeAPIServerFunc }()
				GetLegacyDeployKubeAPIServerFunc = func(*Botanist) func(context.Context) error {
					return func(context.Context) error { return nil }
				}

				kubeAPIServer.EXPECT().GetValues().DoAndReturn(func() kubeapiserver.Values {
					return kubeapiserver.Values{Autoscaling: autoscalingConfig}
				})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			},

			Entry("no change due to already set",
				nil,
				kubeapiserver.AutoscalingConfig{Replicas: pointer.Int32(1)},
				int32(1),
			),
			Entry("use minReplicas because deployment does not exist",
				nil,
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(2),
			),
			Entry("use deployment replicas because they are greater than 0",
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(3),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(3),
			),
			Entry("use 0 because shoot is hibernated and deployment is already scaled down",
				func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(0),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(0),
			),
		)
	})
})

func featureGatePtr(f featuregate.Feature) *featuregate.Feature {
	return &f
}
