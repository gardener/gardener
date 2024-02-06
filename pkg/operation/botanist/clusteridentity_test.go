// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclusteridentity "github.com/gardener/gardener/pkg/component/clusteridentity/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("ClusterIdentity", func() {
	const (
		shootName             = "shootName"
		shootNamespace        = "shootNamespace"
		shootSeedNamespace    = "shootSeedNamespace"
		shootUID              = "shootUID"
		gardenClusterIdentity = "garden-cluster-identity"
	)

	var (
		ctrl            *gomock.Controller
		clusterIdentity *mockclusteridentity.MockInterface

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake")

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		shoot *gardencorev1beta1.Shoot

		botanist *Botanist

		expectedShootClusterIdentity = fmt.Sprintf("%s-%s-%s", shootSeedNamespace, shootUID, gardenClusterIdentity)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clusterIdentity = mockclusteridentity.NewMockInterface(ctrl)

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Status: gardencorev1beta1.ShootStatus{
				UID: shootUID,
			},
		}
	})

	JustBeforeEach(func() {
		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(extensionsv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(gardencorev1beta1.AddToScheme(s)).To(Succeed())

		cluster := &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: shootSeedNamespace,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Object: shoot},
			},
		}

		gardenClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(shoot).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
		seedClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(cluster).Build()
		seedClientSet = kubernetesfake.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Shoot: &shootpkg.Shoot{
					SeedNamespace: shootSeedNamespace,
					Components: &shootpkg.Components{
						SystemComponents: &shootpkg.SystemComponents{
							ClusterIdentity: clusterIdentity,
						},
					},
				},
				GardenClusterIdentity: gardenClusterIdentity,
			},
		}
		botanist.Shoot.SetInfo(shoot)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureShootClusterIdentity", func() {
		test := func() {
			Expect(botanist.EnsureShootClusterIdentity(ctx)).NotTo(HaveOccurred())

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(shootNamespace, shootName), shoot)).To(Succeed())
			Expect(shoot.Status.ClusterIdentity).NotTo(BeNil())
			Expect(*shoot.Status.ClusterIdentity).To(Equal(expectedShootClusterIdentity))
		}

		Context("cluster identity is nil", func() {
			BeforeEach(func() {
				shoot.Status.ClusterIdentity = nil
			})
			It("should set shoot.status.clusterIdentity", test)
		})
		Context("cluster identity already exists", func() {
			BeforeEach(func() {
				shoot.Status.ClusterIdentity = ptr.To(expectedShootClusterIdentity)
			})
			It("should not touch shoot.status.clusterIdentity", test)
		})
	})

	Describe("#DeployClusterIdentity", func() {
		JustBeforeEach(func() {
			botanist.Shoot.GetInfo().Status.ClusterIdentity = &expectedShootClusterIdentity
			clusterIdentity.EXPECT().SetIdentity(expectedShootClusterIdentity)
		})

		It("should deploy successfully", func() {
			clusterIdentity.EXPECT().Deploy(ctx)
			Expect(botanist.DeployClusterIdentity(ctx)).To(Succeed())
		})

		It("should return the error during deployment", func() {
			clusterIdentity.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployClusterIdentity(ctx)).To(MatchError(fakeErr))
		})
	})
})
