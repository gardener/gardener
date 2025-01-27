// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	mocknetwork "github.com/gardener/gardener/pkg/component/extensions/network/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("DualStackMigration", func() {
	var (
		ctrl             *gomock.Controller
		botanist         *Botanist
		mockClock        *testing.FakeClock
		infrastructure   *mockinfrastructure.MockInterface
		network          *mocknetwork.MockInterface
		shootInterface   *kubernetesmock.MockInterface
		shootClient      *mockclient.MockClient
		gardenClient     *mockclient.MockClient
		mockStatusWriter *mockclient.MockStatusWriter
		ctx              = context.TODO()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		// Use a mock clock with a fixed time
		mockClock = testing.NewFakeClock(time.Date(2025, 3, 30, 3, 33, 33, 33, time.UTC))

		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{},
				},
			},
		}}
		shootInterface = kubernetesmock.NewMockInterface(ctrl)
		shootClient = mockclient.NewMockClient(ctrl)
		botanist.ShootClientSet = shootInterface
		gardenClient = mockclient.NewMockClient(ctrl)
		botanist.GardenClient = gardenClient
		mockStatusWriter = mockclient.NewMockStatusWriter(ctrl)

		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		infrastructure.EXPECT().Get(gomock.Any()).Return(&extensionsv1alpha1.Infrastructure{
			Status: extensionsv1alpha1.InfrastructureStatus{
				Networking: &extensionsv1alpha1.InfrastructureStatusNetworking{
					Nodes: []string{"0.0.0.0", "2001:db8::1"},
				},
			},
		}, nil).AnyTimes()

		network = mocknetwork.NewMockInterface(ctrl)

		botanist.Shoot = &shootpkg.Shoot{
			Components: &shootpkg.Components{
				Extensions: &shootpkg.Extensions{
					Infrastructure: infrastructure,
					Network:        network,
				},
			},
		}
		botanist.Clock = mockClock
	})

	Describe("#CheckDualStackMigration", func() {
		It("Nodes are migrated to dual-stack networking", func() {
			condition := v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			condition = v1beta1helper.UpdatedConditionWithClock(mockClock, condition, gardencorev1beta1.ConditionFalse, "NodesNotMigrated", "Nodes are not migrated to dual-stack networking.")

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{condition},
				},
			}
			botanist.Shoot.SetInfo(shoot)

			shootInterface.EXPECT().Client().Return(shootClient).AnyTimes()

			shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
				*list = corev1.NodeList{Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				}}
				return nil
			}).AnyTimes()

			network.EXPECT().Get(gomock.Any()).Return(&extensionsv1alpha1.Network{
				Status: extensionsv1alpha1.NetworkStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{Raw: []byte(`{"ipFamilies": ["IPv4"]}`)},
					},
				},
			}, nil).AnyTimes()

			updatedShoot := shoot.DeepCopy()
			updatedCondition := v1beta1helper.UpdatedConditionWithClock(mockClock, condition, gardencorev1beta1.ConditionTrue, "NodesMigrated", "Nodes are migrated to dual-stack networking.")
			updatedShoot.Status.Constraints = v1beta1helper.MergeConditions(updatedShoot.Status.Constraints, updatedCondition)

			gardenClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()
			test.EXPECTStatusPatch(ctx, mockStatusWriter, updatedShoot, shoot, types.StrategicMergePatchType).AnyTimes()

			err := botanist.CheckPodCIDRsInNodes(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Nodes are not migrated to dual-stack networking", func() {
			condition := v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			condition = v1beta1helper.UpdatedConditionWithClock(mockClock, condition, gardencorev1beta1.ConditionFalse, "NodesNotMigrated", "Nodes are not migrated to dual-stack networking.")

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{condition},
				},
			}
			botanist.Shoot.SetInfo(shoot)

			shootInterface.EXPECT().Client().Return(shootClient).AnyTimes()

			shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
				*list = corev1.NodeList{Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24"}}},
				}}
				return nil
			}).AnyTimes()

			network.EXPECT().Get(gomock.Any()).Return(&extensionsv1alpha1.Network{
				Status: extensionsv1alpha1.NetworkStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{Raw: []byte(`{"ipFamilies": ["IPv4"]}`)},
					},
				},
			}, nil).AnyTimes()

			updatedShoot := shoot.DeepCopy()
			updatedCondition := v1beta1helper.UpdatedConditionWithClock(mockClock, condition, gardencorev1beta1.ConditionFalse, "NodesNotMigrated", "Nodes are not migrated to dual-stack networking.")
			updatedShoot.Status.Constraints = v1beta1helper.MergeConditions(updatedShoot.Status.Constraints, updatedCondition)

			gardenClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()
			test.EXPECTStatusPatch(ctx, mockStatusWriter, updatedShoot, shoot, types.StrategicMergePatchType).AnyTimes()

			err := botanist.CheckPodCIDRsInNodes(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Nodes and network config are migrated to dual-stack networking", func() {
			condition := v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			condition = v1beta1helper.UpdatedConditionWithClock(mockClock, condition, gardencorev1beta1.ConditionFalse, "NodesMigrated", "Nodes are migrated to dual-stack networking.")

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{condition},
				},
			}
			botanist.Shoot.SetInfo(shoot)

			shootInterface.EXPECT().Client().Return(shootClient).AnyTimes()

			shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
				*list = corev1.NodeList{Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				}}
				return nil
			}).AnyTimes()

			network.EXPECT().Get(gomock.Any()).Return(&extensionsv1alpha1.Network{
				Status: extensionsv1alpha1.NetworkStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{Raw: []byte(`{"ipFamilies": ["IPv4", "IPv6"]}`)},
					},
				},
			}, nil).AnyTimes()

			updatedShoot := shoot.DeepCopy()
			updatedShoot.Status.Constraints = v1beta1helper.RemoveConditions(updatedShoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)

			gardenClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()
			test.EXPECTStatusPatch(ctx, mockStatusWriter, updatedShoot, shoot, types.StrategicMergePatchType).AnyTimes()

			err := botanist.CheckPodCIDRsInNodes(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Doesn't add the constraint to a migrated shoot.", func() {

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{},
			}
			botanist.Shoot.SetInfo(shoot)

			shootInterface.EXPECT().Client().Return(shootClient).AnyTimes()

			shootClient.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
				*list = corev1.NodeList{Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				}}
				return nil
			}).AnyTimes()

			network.EXPECT().Get(gomock.Any()).Return(&extensionsv1alpha1.Network{
				Status: extensionsv1alpha1.NetworkStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{Raw: []byte(`{"ipFamilies": ["IPv4", "IPv6"]}`)},
					},
				},
			}, nil).AnyTimes()

			updatedShoot := shoot.DeepCopy()

			gardenClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()
			test.EXPECTStatusPatch(ctx, mockStatusWriter, updatedShoot, shoot, types.StrategicMergePatchType).AnyTimes()

			err := botanist.CheckPodCIDRsInNodes(context.TODO())
			Expect(err).NotTo(HaveOccurred())
		})

	})
})
