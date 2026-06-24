// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	fakeresourcemanager "github.com/gardener/gardener/pkg/component/gardener/resourcemanager/fake"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ResourceManager", func() {
	var botanist *Botanist

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	Describe("#DefaultResourceManager", func() {
		var k8sSeedClient kubernetes.Interface

		BeforeEach(func() {
			k8sSeedClient = fakekubernetes.NewClientSetBuilder().WithVersion("v1.30.1").Build()
			botanist.SeedClientSet = k8sSeedClient

			botanist.Seed = &seedpkg.Seed{}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{})
			botanist.Shoot = &shootpkg.Shoot{
				KubernetesVersion:     semver.MustParse("1.32.1"),
				ExternalClusterDomain: new("foo.local.gardener.cloud"),
				ControlPlaneNamespace: "shoot--foo--bar",
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					CredentialsBindingName: new("foo-credentials"),
				},
			})
		})

		It("should successfully create a resource-manager component", func() {
			resourceManager, err := botanist.DefaultResourceManager()
			Expect(resourceManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())

			Expect(resourceManager.GetValues().PodTopologySpreadConstraintsEnabled).To(BeFalse())
			Expect(resourceManager.GetValues().VPAInPlaceUpdatesEnabled).To(BeTrue())
			Expect(resourceManager.GetValues().MachineNamespace).To(HaveValue(Equal("shoot--foo--bar")))
		})

		It("should consider node toleration configuration", func() {
			notReadyTolerationSeconds := new(int64(60))
			unreachableTolerationSeconds := new(int64(120))

			botanist.Config = &gardenletconfigv1alpha1.GardenletConfiguration{
				NodeToleration: &gardenletconfigv1alpha1.NodeToleration{
					DefaultNotReadyTolerationSeconds:    notReadyTolerationSeconds,
					DefaultUnreachableTolerationSeconds: unreachableTolerationSeconds,
				},
			}

			resourceManager, err := botanist.DefaultResourceManager()
			Expect(resourceManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues().DefaultNotReadyToleration).To(Equal(notReadyTolerationSeconds))
			Expect(resourceManager.GetValues().DefaultUnreachableToleration).To(Equal(unreachableTolerationSeconds))
		})

		It("should successfully set PodTopologySpreadConstraintsEnabled=true if MatchLabelKeysInPodTopologySpread feature gate is disabled in the Shoot", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{"MatchLabelKeysInPodTopologySpread": false},
							},
						},
						KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{"MatchLabelKeysInPodTopologySpread": false},
							},
						},
					},
				},
			})

			resourceManager, err := botanist.DefaultResourceManager()
			Expect(resourceManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues().PodTopologySpreadConstraintsEnabled).To(BeTrue())
		})

		It("should successfully set NodeAgentAuthorizerAuthorizeWithSelectors=true if AuthorizeWithSelectors is enabled in the Shoot", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{"AuthorizeWithSelectors": true},
							},
						},
					},
				},
			})

			resourceManager, err := botanist.DefaultResourceManager()
			Expect(resourceManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceManager.GetValues().NodeAgentAuthorizerAuthorizeWithSelectors).To(PointTo(Equal(true)))
		})

		Context("self-hosted shoots", func() {
			BeforeEach(func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
					Name:         "control-plane",
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
				}}
				botanist.Shoot.SetInfo(shoot)
				botanist.Shoot.ControlPlaneNamespace = "kube-system"
			})

			Context("managed infrastructure", func() {
				It("should correctly configure the resource-manager component", func() {
					resourceManager, err := botanist.DefaultResourceManager()
					Expect(resourceManager).NotTo(BeNil())
					Expect(err).NotTo(HaveOccurred())

					Expect(resourceManager.GetValues().MachineNamespace).To(HaveValue(Equal("kube-system")))
					Expect(resourceManager.GetValues().VPAInPlaceUpdatesEnabled).To(BeFalse())
					Expect(resourceManager.GetValues().SystemComponentTolerations).To(ContainElement(HaveField("Key", "node-role.kubernetes.io/control-plane")))
				})

				When("not running inside self hosted shoot (gardenadm bootstrap)", func() {
					BeforeEach(func() {
						botanist.Shoot.ControlPlaneNamespace = "shoot--foo--bar"
					})

					It("should not have `node-role.kubernetes.io/control-plane` toleration", func() {
						resourceManager, err := botanist.DefaultResourceManager()
						Expect(resourceManager).NotTo(BeNil())
						Expect(err).NotTo(HaveOccurred())

						Expect(resourceManager.GetValues().SystemComponentTolerations).ToNot(ContainElement(HaveField("Key", "node-role.kubernetes.io/control-plane")))
					})
				})
			})

			Context("unmanaged infrastructure", func() {
				BeforeEach(func() {
					shoot := botanist.Shoot.GetInfo()
					shoot.Spec.CredentialsBindingName = nil
					botanist.Shoot.SetInfo(shoot)
				})

				It("should correctly configure the resource-manager component", func() {
					resourceManager, err := botanist.DefaultResourceManager()
					Expect(resourceManager).NotTo(BeNil())
					Expect(err).NotTo(HaveOccurred())

					Expect(resourceManager.GetValues().MachineNamespace).To(BeNil())
					Expect(resourceManager.GetValues().VPAInPlaceUpdatesEnabled).To(BeFalse())
				})
			})
		})
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			ctx                   = context.TODO()
			fakeErr               = errors.New("fake err")
			controlPlaneNamespace = "fake-seed-ns"

			fakeClient    client.Client
			fakeClock     *testclock.FakeClock
			rm            *fakeresourcemanager.ResourceManager
			kubeAPIServer *fakeKubeAPIServer

			bootstrapKubeconfigSecret *corev1.Secret
			shootAccessSecret         *corev1.Secret
			managedResource           *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
			botanist.Clock = fakeClock

			rm = &fakeresourcemanager.ResourceManager{}
			kubeAPIServer = &fakeKubeAPIServer{autoscalingReplicas: new(int32(1))}

			bootstrapKubeconfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager-bootstrap-905aeb60",
					Namespace: controlPlaneNamespace,
				},
			}
			shootAccessSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-gardener-resource-manager",
					Namespace: controlPlaneNamespace,
					Annotations: map[string]string{
						resourcesv1alpha1.ServiceAccountTokenRenewTimestamp: fakeClock.Now().Add(time.Hour).Format(time.RFC3339),
					},
				},
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-gardener-resource-manager",
					Namespace: controlPlaneNamespace,
				},
			}

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KubeAPIServer:   kubeAPIServer,
						ResourceManager: rm,
					},
				},
				ControlPlaneNamespace: controlPlaneNamespace,
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeReconcile,
					},
				},
			})
		})

		JustBeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: controlPlaneNamespace}})).To(Succeed())

			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
			botanist.SecretsManager = fakesecretsmanager.New(fakeClient, controlPlaneNamespace)
		})

		Context("w/o bootstrapping", func() {
			JustBeforeEach(func() {
				// Pre-create objects so bootstrap is not triggered
				Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
				Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			})

			Context("when GRM should not be scaled up", func() {
				It("due to shoot reconciling and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: new(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeReconcile,
							},
							IsHibernated: true,
						},
					})

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					Expect(rm.Replicas).To(PointTo(Equal(int32(0))))
					Expect(rm.DeployCalled).To(BeTrue())
				})

				It("due to shoot reconciling and not hibernated but kube-apiserver replicas are 0", func() {
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeReconcile,
							},
						},
					})
					kubeAPIServer.autoscalingReplicas = new(int32(0))

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					Expect(rm.Replicas).To(PointTo(Equal(int32(0))))
					Expect(rm.DeployCalled).To(BeTrue())
				})

				It("due to shoot creation and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: new(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeCreate,
							},
							IsHibernated: true,
						},
					})

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					Expect(rm.Replicas).To(PointTo(Equal(int32(0))))
					Expect(rm.DeployCalled).To(BeTrue())
				})

				It("due to shoot restoration and hibernated", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Hibernation: &gardencorev1beta1.Hibernation{
								Enabled: new(true),
							},
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								Type: gardencorev1beta1.LastOperationTypeRestore,
							},
							IsHibernated: true,
						},
					})

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					Expect(rm.Replicas).To(PointTo(Equal(int32(0))))
					Expect(rm.DeployCalled).To(BeTrue())
				})
			})

			Context("shoot is not hibernated", func() {
				It("should set the secrets and deploy", func() {
					kubeAPIServer.autoscalingReplicas = new(int32(2))

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					Expect(rm.Replicas).To(PointTo(Equal(int32(2))))
					Expect(rm.Secrets.BootstrapKubeconfig).To(BeNil())
					Expect(rm.DeployCalled).To(BeTrue())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), bootstrapKubeconfigSecret)).To(BeNotFoundError())
				})

				It("should fail when the deploy function fails", func() {
					rm.DeployError = fakeErr
					Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(fakeErr))
				})
			})
		})

		Context("w/ bootstrapping", func() {
			Context("with success", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithVar(&shared.WaitUntilGardenerResourceManagerBootstrapped,
						func(_ context.Context, _ client.Client, _ clock.Clock, _ string) error { return nil },
					))
				})

				AfterEach(func() {
					Expect(rm.Replicas).To(PointTo(Equal(int32(2))))
					Expect(rm.Secrets.BootstrapKubeconfig).To(BeNil())
					Expect(rm.DeployCalled).To(BeTrue())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bootstrapKubeconfigSecret), bootstrapKubeconfigSecret)).To(Succeed())
				})

				bootstrapTests := func() {
					It("bootstraps because the shoot access secret was not found", func() {
						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})

					It("bootstraps because the shoot access secret was never reconciled", func() {
						shootAccessSecret.Annotations = nil
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})

					It("bootstraps because the shoot access secret was not renewed", func() {
						shootAccessSecret.Annotations = map[string]string{
							resourcesv1alpha1.ServiceAccountTokenRenewTimestamp: fakeClock.Now().Add(-time.Hour).Format(time.RFC3339),
						}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})

					It("bootstraps because the managed resource was not found", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})

					It("bootstraps because the managed resource indicates that the shoot access token lost access", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						managedResource.Status.ObservedGeneration = managedResource.Generation
						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `forbidden: User "system:serviceaccount:kube-system:gardener-resource-manager" cannot do anything`},
							{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})

					It("bootstraps because the managed resource indicates that the shoot access token was invalidated", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						managedResource.Status.ObservedGeneration = managedResource.Generation
						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: `failed to compute all HPA target ref object keys: failed to list all HPAs: Unauthorized`},
							{Type: "ResourcesHealthy", Status: gardencorev1beta1.ConditionTrue},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
					})
				}

				Context("shoot is not hibernated", func() {
					bootstrapTests()
				})

				Context("shoot is in the process of being woken-up", func() {
					BeforeEach(func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Status: gardencorev1beta1.ShootStatus{
								IsHibernated: true,
							},
						})
					})

					bootstrapTests()
				})

				Context("shoot is hibernated but GRM should be scaled up", func() {
					BeforeEach(func() {
						botanist.Shoot.HibernationEnabled = true
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Status: gardencorev1beta1.ShootStatus{IsHibernated: true},
						})
						rm.Replicas = new(int32(2))
					})

					bootstrapTests()
				})
			})

			Context("with failure", func() {
				It("fails because the bootstrap kubeconfig secret cannot be created", func() {
					fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).
						WithObjects(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: controlPlaneNamespace}}).
						WithInterceptorFuncs(interceptor.Funcs{
							Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
								return fakeErr
							},
						}).Build()

					botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
					botanist.SecretsManager = fakesecretsmanager.New(fakeClient, controlPlaneNamespace)

					Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(fakeErr))
				})

				Context("waiting for bootstrapping process", func() {
					BeforeEach(func() {
						DeferCleanup(test.WithVars(
							&shared.TimeoutWaitForGardenerResourceManagerBootstrapping, 500*time.Millisecond,
							&shared.IntervalWaitForGardenerResourceManagerBootstrapping, 5*time.Millisecond,
						))
					})

					It("fails because the shoot access token was not generated", func() {
						shootAccessSecret.Annotations = nil
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx)).To(MatchError(ContainSubstring("token not yet generated")))
					})

					It("fails because the shoot access token renew timestamp cannot be parsed", func() {
						shootAccessSecret.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring("could not parse renew timestamp"))
					})

					It("fails because the shoot access token was not renewed", func() {
						shootAccessSecret.Annotations = map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": fakeClock.Now().Add(-time.Hour).Format(time.RFC3339)}
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring("token not yet renewed"))
					})

					It("fails because the managed resource is not getting healthy", func() {
						Expect(fakeClient.Create(ctx, shootAccessSecret)).To(Succeed())

						managedResource.Status.Conditions = []gardencorev1beta1.Condition{
							{Type: "ResourcesApplied", Status: gardencorev1beta1.ConditionFalse, Message: ": Unauthorized"},
						}
						Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

						Expect(botanist.DeployGardenerResourceManager(ctx).Error()).To(ContainSubstring("managed resource " + controlPlaneNamespace + "/" + managedResource.Name + " is not healthy"))
					})
				})
			})
		})
	})
})

type fakeKubeAPIServer struct {
	kubeapiserver.Interface

	autoscalingReplicas *int32
}

func (f *fakeKubeAPIServer) GetAutoscalingReplicas() *int32 { return f.autoscalingReplicas }
