// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("StaticPods", func() {
	var (
		ctrl                  *gomock.Controller
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		seedClient            client.Client
		botanist              *Botanist

		ctx = context.Background()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)

		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		botanist = &Botanist{
			Operation: &operation.Operation{
				SeedClientSet: fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build(),
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							OperatingSystemConfig: operatingSystemConfig,
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#UpdateNodeAgentSecretNameLabelsOnNodes", func() {
		const (
			pool1 = "pool1"
			pool2 = "pool2"
		)

		var (
			oscSecretName1 = "cloud-config-pool1-abc123"
			oscSecretName2 = "cloud-config-pool2-def456"

			oscMap = map[string]*operatingsystemconfig.OperatingSystemConfigs{
				pool1: {Original: operatingsystemconfig.Data{GardenerNodeAgentSecretName: oscSecretName1}},
				pool2: {Original: operatingsystemconfig.Data{GardenerNodeAgentSecretName: oscSecretName2}},
			}
		)

		It("should do nothing when all labels are already up-to-date", func() {
			node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool1,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: oscSecretName1,
				},
			}}
			node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool2,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: oscSecretName2,
				},
			}}
			node3 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node3",
			}}
			Expect(seedClient.Create(ctx, node1)).To(Succeed())
			Expect(seedClient.Create(ctx, node2)).To(Succeed())
			Expect(seedClient.Create(ctx, node3)).To(Succeed())

			operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(oscMap)

			Expect(botanist.UpdateNodeAgentSecretNameLabelsOnNodes(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node1), node1)).To(Succeed())
			Expect(node1.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName1))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node2), node2)).To(Succeed())
			Expect(node2.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName2))
		})

		It("should update the label on nodes where it is stale", func() {
			staleSecretName := "cloud-config-pool1-old"
			node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool1,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: staleSecretName,
				},
			}}
			node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool2,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: oscSecretName2,
				},
			}}
			Expect(seedClient.Create(ctx, node1)).To(Succeed())
			Expect(seedClient.Create(ctx, node2)).To(Succeed())

			operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(oscMap)

			Expect(botanist.UpdateNodeAgentSecretNameLabelsOnNodes(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node1), node1)).To(Succeed())
			Expect(node1.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName1))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node2), node2)).To(Succeed())
			Expect(node2.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName2))
		})

		It("should update labels on all stale nodes in parallel", func() {
			staleSecretName1 := "cloud-config-pool1-old"
			staleSecretName2 := "cloud-config-pool2-old"
			node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool1,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: staleSecretName1,
				},
			}}
			node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
				Labels: map[string]string{
					v1beta1constants.LabelWorkerPool:                            pool2,
					v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: staleSecretName2,
				},
			}}
			Expect(seedClient.Create(ctx, node1)).To(Succeed())
			Expect(seedClient.Create(ctx, node2)).To(Succeed())

			operatingSystemConfig.EXPECT().WorkerPoolNameToOperatingSystemConfigsMap().Return(oscMap)

			Expect(botanist.UpdateNodeAgentSecretNameLabelsOnNodes(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node1), node1)).To(Succeed())
			Expect(node1.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName1))

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(node2), node2)).To(Succeed())
			Expect(node2.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]).To(Equal(oscSecretName2))
		})
	})

	Describe("#useShootAccessTokensForSelfHostedShootControlPlane", func() {
		var controlPlaneNamespace = "kube-system"

		selfHostedShoot := func(gardenerName string) *gardencorev1beta1.Shoot {
			return &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
						}},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Gardener: gardencorev1beta1.Gardener{Name: gardenerName},
				},
			}
		}

		BeforeEach(func() {
			botanist.Shoot.ControlPlaneNamespace = controlPlaneNamespace
		})

		It("should return false for a non-self-hosted shoot", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when the shoot status indicates gardenadm", func() {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenadm"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager deployment does not exist", func() {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return false when gardener-resource-manager runs with host network (bootstrap phase)", func() {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: controlPlaneNamespace},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: true}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeFalse())
		})

		It("should return true when gardener-resource-manager runs in the pod network", func() {
			botanist.Shoot.SetInfo(selfHostedShoot("gardenlet"))
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager", Namespace: controlPlaneNamespace},
				Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: false}}},
			}
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())

			result, err := UseShootAccessTokensForSelfHostedShootControlPlane(botanist, ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})
})
