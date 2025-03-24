// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	"github.com/Masterminds/semver/v3"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("MachineControllerManager", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake err")

		ctrl               *gomock.Controller
		kubernetesClient   *kubernetesmock.MockInterface
		fakeClient         client.Client
		fakeSecretsManager secretsmanager.Interface

		shoot      *gardencorev1beta1.Shoot
		deployment *appsv1.Deployment

		botanist  *Botanist
		namespace = "shoot--foo--bar"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretsManager = fakesecretsmanager.New(fakeClient, namespace)

		shoot = &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"}}}
		deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: namespace}}

		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.SeedClientSet = kubernetesClient
		botanist.SecretsManager = fakeSecretsManager
		botanist.Seed = &seedpkg.Seed{KubernetesVersion: semver.MustParse("1.31.0")}
		botanist.Shoot = &shootpkg.Shoot{ControlPlaneNamespace: namespace}
		botanist.Shoot.SetInfo(shoot)

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("#DeployMachineControllerManager", func() {
		BeforeEach(func() {
			kubernetesClient.EXPECT().Version()
			kubernetesClient.EXPECT().Client().Return(fakeClient)

			botanist.SeedNamespaceObject = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("5678"),
				},
			}

			machineControllerManager, err := botanist.DefaultMachineControllerManager()
			Expect(machineControllerManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			botanist.Shoot.Components = &shootpkg.Components{ControlPlane: &shootpkg.ControlPlane{MachineControllerManager: machineControllerManager}}

			By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
		})

		DescribeTable("it should successfully create a machine-controller-manager interface",
			func(expectedReplicas int, prepTest func()) {
				kubernetesClient.EXPECT().Client().Return(fakeClient)

				if prepTest != nil {
					prepTest()
				}

				Expect(botanist.DeployMachineControllerManager(ctx)).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				Expect(deployment.Spec.Replicas).To(PointTo(Equal(int32(expectedReplicas))))
			},

			Entry("with default replicas", 1, nil),
			Entry("when shoot shall be deleted", 1, func() {
				shoot.DeletionTimestamp = &metav1.Time{}
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when machine deployments with positive replica count exist", 1, func() {
				machineDeployment := &machinev1alpha1.MachineDeployment{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace},
					Spec:       machinev1alpha1.MachineDeploymentSpec{Replicas: 5},
				}
				Expect(fakeClient.Create(ctx, machineDeployment)).To(Succeed())
			}),
			Entry("when shoot is fully hibernated", 0, func() {
				botanist.Shoot.HibernationEnabled = true
				shoot.Status.IsHibernated = true
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot shall be hibernated but last operation is nil", 0, func() {
				botanist.Shoot.HibernationEnabled = true
				shoot.Status.LastOperation = nil
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot shall be hibernated but last operation is create", 0, func() {
				botanist.Shoot.HibernationEnabled = true
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot shall be hibernated but last operation is not create", 1, func() {
				botanist.Shoot.HibernationEnabled = true
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile}
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot shall be hibernated but process is not finished yet", 1, func() {
				botanist.Shoot.HibernationEnabled = true
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile}
				shoot.Status.IsHibernated = false
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot shall be woken up but process is not finished yet", 1, func() {
				botanist.Shoot.HibernationEnabled = false
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile}
				shoot.Status.IsHibernated = true
				botanist.Shoot.SetInfo(shoot)
			}),
			Entry("when shoot is in restore phase", 0, func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				botanist.Shoot.SetInfo(shoot)
			}),
		)
	})

	Describe("#ScaleMachineControllerManagerToZero", func() {
		var (
			mockClient            *mockclient.MockClient
			mockSubresourceClient *mockclient.MockSubResourceClient
			patch                 = client.RawPatch(types.MergePatchType, []byte(`{"spec":{"replicas":0}}`))
		)

		BeforeEach(func() {
			mockClient = mockclient.NewMockClient(ctrl)
			mockSubresourceClient = mockclient.NewMockSubResourceClient(ctrl)

			kubernetesClient.EXPECT().Client().Return(mockClient)
			mockClient.EXPECT().SubResource("scale").Return(mockSubresourceClient)

			botanist.SeedClientSet = kubernetesClient
		})

		It("should scale the CA deployment", func() {
			mockSubresourceClient.EXPECT().Patch(ctx, deployment, patch)
			Expect(botanist.ScaleMachineControllerManagerToZero(ctx)).To(Succeed())
		})

		It("should fail when the scale call fails", func() {
			mockSubresourceClient.EXPECT().Patch(ctx, deployment, patch).Return(fakeErr)
			Expect(botanist.ScaleMachineControllerManagerToZero(ctx)).To(MatchError(fakeErr))
		})
	})
})
