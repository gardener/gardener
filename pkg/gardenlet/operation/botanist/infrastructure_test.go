// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Infrastructure", func() {
	var (
		ctrl           *gomock.Controller
		infrastructure *mockinfrastructure.MockInterface

		fakeClient client.Client
		sm         secretsmanager.Interface
		botanist   *Botanist

		ctx        = context.TODO()
		namespace  = "namespace"
		fakeErr    = errors.New("fake")
		shootState = &gardencorev1beta1.ShootState{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		infrastructure = mockinfrastructure.NewMockInterface(ctrl)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: namespace}})).To(Succeed())

		botanist = &Botanist{
			Operation: &operation.Operation{
				SecretsManager: sm,
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							Infrastructure: infrastructure,
						},
					},
				},
			},
		}
		botanist.Shoot.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployInfrastructure", func() {
		BeforeEach(func() {
			infrastructure.EXPECT().SetSSHPublicKey(gomock.Any())
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				infrastructure.EXPECT().Deploy(ctx)
				Expect(botanist.DeployInfrastructure(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				infrastructure.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployInfrastructure(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
				botanist.Shoot.SetInfo(shoot)
			})

			It("should restore successfully", func() {
				infrastructure.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployInfrastructure(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				infrastructure.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployInfrastructure(ctx)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#WaitForInfrastructure", func() {
		var (
			gardenFakeClient client.Client
			seedFakeClient   client.Client
			seedClientSet    *fakekubernetes.ClientSet

			shootNamespace = "namespace"
			shootName      = "name"
			nodesCIDRs     = []string{"1.2.3.4/5"}
			podsCIDRs      = []string{"2.3.4.5/6"}
			servicesCIDRs  = []string{"3.4.5.6/7"}
			egressCIDRs    = []string{"4.5.6.7/8"}
			shoot          = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						// Nodes:    ptr.To(nodesCIDRs[0]),
						Pods:     ptr.To(podsCIDRs[0]),
						Services: ptr.To(servicesCIDRs[0]),
					},
				},
			}
		)

		BeforeEach(func() {
			gardenFakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithStatusSubresource(&gardencorev1beta1.Shoot{}).
				Build()
			seedFakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedFakeClient).Build()

			botanist.GardenClient = gardenFakeClient
			botanist.SeedClientSet = seedClientSet
			botanist.Shoot.SetInfo(shoot)
			botanist.Shoot.ControlPlaneNamespace = "shoot--foo--bar"

			// Create the shoot in the garden fake client
			Expect(gardenFakeClient.Create(ctx, shoot.DeepCopy())).To(Succeed())

			// Create the Cluster resource in the seed fake client
			Expect(seedFakeClient.Create(ctx, &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: botanist.Shoot.ControlPlaneNamespace},
			})).To(Succeed())
		})

		It("should successfully wait (w/ CIDRs)", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().NodesCIDRs().Return(nodesCIDRs)
			infrastructure.EXPECT().PodsCIDRs().Return(podsCIDRs)
			infrastructure.EXPECT().ServicesCIDRs().Return(servicesCIDRs)
			infrastructure.EXPECT().EgressCIDRs().Return(egressCIDRs)

			Expect(botanist.WaitForInfrastructure(ctx)).To(Succeed())

			updatedShoot := botanist.Shoot.GetInfo()
			Expect(updatedShoot.Spec.Networking.Nodes).To(Equal(ptr.To(nodesCIDRs[0])))
			Expect(updatedShoot.Status.Networking).To(Equal(&gardencorev1beta1.NetworkingStatus{
				Nodes:       nodesCIDRs,
				Pods:        podsCIDRs,
				Services:    servicesCIDRs,
				EgressCIDRs: egressCIDRs,
			}))
		})

		It("should successfully wait (w/o CIDRs)", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().NodesCIDRs()
			infrastructure.EXPECT().PodsCIDRs()
			infrastructure.EXPECT().ServicesCIDRs()
			infrastructure.EXPECT().EgressCIDRs()

			// Get the shoot from fake client to get its current ResourceVersion
			storedShoot := &gardencorev1beta1.Shoot{}
			Expect(gardenFakeClient.Get(ctx, client.ObjectKey{Name: shootName, Namespace: shootNamespace}, storedShoot)).To(Succeed())
			storedShoot.Spec.Networking.Nodes = ptr.To(nodesCIDRs[0])
			Expect(gardenFakeClient.Update(ctx, storedShoot)).To(Succeed())
			// Also update botanist's in-memory shoot
			botanist.Shoot.SetInfo(storedShoot)

			Expect(botanist.WaitForInfrastructure(ctx)).To(Succeed())

			result := botanist.Shoot.GetInfo()
			Expect(result.Status.Networking).To(Equal(&gardencorev1beta1.NetworkingStatus{}))
		})

		It("should return the error during wait", func() {
			infrastructure.EXPECT().Wait(ctx).Return(fakeErr)

			Expect(botanist.WaitForInfrastructure(ctx)).To(MatchError(fakeErr))
			Expect(botanist.Shoot.GetInfo()).To(Equal(shoot))
		})

		It("should return the error during nodes cidr update", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().NodesCIDRs().Return(nodesCIDRs)

			// Use an interceptor to make the Patch fail
			gardenFakeClientWithErr := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithStatusSubresource(&gardencorev1beta1.Shoot{}).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return fakeErr
					},
				}).
				Build()
			botanist.GardenClient = gardenFakeClientWithErr
			Expect(gardenFakeClientWithErr.Create(ctx, shoot.DeepCopy())).To(Succeed())

			Expect(botanist.WaitForInfrastructure(ctx)).To(MatchError(fakeErr))
			Expect(botanist.Shoot.GetInfo()).To(Equal(shoot))
		})
	})
})
