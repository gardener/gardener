// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
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
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockclusteridentity "github.com/gardener/gardener/pkg/component/clusteridentity/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ClusterIdentity", func() {
	const (
		shootName                  = "shootName"
		shootNamespace             = "shootNamespace"
		shootControlPlaneNamespace = "shootControlPlaneNamespace"
		shootUID                   = "shootUID"
		gardenClusterIdentity      = "garden-cluster-identity"
	)

	var (
		ctrl            *gomock.Controller
		clusterIdentity *mockclusteridentity.MockInterface

		ctx     = context.TODO()
		fakeErr = errors.New("fake")

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		shoot *gardencorev1beta1.Shoot

		botanist *Botanist

		expectedShootClusterIdentity = fmt.Sprintf("%s-%s-%s", shootControlPlaneNamespace, shootUID, gardenClusterIdentity)
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
				Name: shootControlPlaneNamespace,
			},
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Object: shoot},
			},
		}

		gardenClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(shoot).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()
		seedClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(cluster).Build()
		seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: shootControlPlaneNamespace,
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

			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: shootNamespace, Name: shootName}, shoot)).To(Succeed())
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
