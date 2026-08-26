// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"

	"github.com/Masterminds/semver/v3"
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/mock"
	mockkubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsrotation "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("KubeControllerManager", func() {
	var (
		ctrl                *gomock.Controller
		botanist            *Botanist
		kubernetesClientSet *fakekubernetes.ClientSet
		fakeClient          client.Client

		ctx       = context.TODO()
		fakeErr   = errors.New("fake err")
		namespace = "foo"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		fakeClient = fakeclient.NewClientBuilder().Build()
		kubernetesClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeControllerManager", func() {
		BeforeEach(func() {
			botanist.Logger = logr.Discard()
			botanist.SeedClientSet = kubernetesClientSet
			botanist.Seed = &seedpkg.Seed{}
			botanist.Shoot = &shootpkg.Shoot{
				KubernetesVersion:        semver.MustParse("1.31.0"),
				RuntimeKubernetesVersion: semver.MustParse("1.31.0"),
				Networks:                 &shootpkg.Networks{},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a kube-controller-manager interface", func() {
			kubeControllerManager, err := botanist.DefaultKubeControllerManager()
			Expect(kubeControllerManager).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployKubeControllerManager", func() {
		var (
			kubeAPIServer         *mockkubeapiserver.MockInterface
			kubeControllerManager *mockkubecontrollermanager.MockInterface
		)

		BeforeEach(func() {
			kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
			kubeControllerManager = mockkubecontrollermanager.NewMockInterface(ctrl)

			botanist.SeedClientSet = kubernetesClientSet
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KubeAPIServer:         kubeAPIServer,
						KubeControllerManager: kubeControllerManager,
					},
				},
				ControlPlaneNamespace: namespace,
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}},
					Services: []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		Context("successfully deployment", func() {
			BeforeEach(func() {
				kubeControllerManager.EXPECT().Deploy(ctx)
				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{RuntimeConfig: map[string]bool{"foo": true}})
				kubeControllerManager.EXPECT().SetRuntimeConfig(map[string]bool{"foo": true})
				kubeControllerManager.EXPECT().SetServiceNetworks(botanist.Shoot.Networks.Services)
				kubeControllerManager.EXPECT().SetPodNetworks(botanist.Shoot.Networks.Pods)
			})

			Context("kube-apiserver is already scaled down", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(new(int32(0)))
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false) and Kube-Apiserver is already scaled down", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is nil or neither of type 'create' nor 'restore'", func() {
				BeforeEach(func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(new(int32(1))).MaxTimes(1)
					botanist.Shoot.GetInfo().Status.LastOperation = nil
				})

				It("hibernation status unequal (true/false)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status unequal (false/true)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (true/true)", func() {
					botanist.Shoot.HibernationEnabled = true
					botanist.Shoot.GetInfo().Status.IsHibernated = true

					var replicas int32 = 4
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
						Spec:       appsv1.DeploymentSpec{Replicas: new(replicas)},
					})).To(Succeed())

					kubeControllerManager.EXPECT().SetReplicaCount(replicas)
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation status equal (false/false)", func() {
					botanist.Shoot.HibernationEnabled = false
					botanist.Shoot.GetInfo().Status.IsHibernated = false

					var replicas int32 = 4
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-controller-manager",
							Namespace:   namespace,
							Annotations: map[string]string{proberapi.MeltdownProtectionActive: ""},
						},
						Spec: appsv1.DeploymentSpec{Replicas: new(replicas)},
					})).To(Succeed())

					kubeControllerManager.EXPECT().SetReplicaCount(replicas)
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is not nil and of type 'create'", func() {
				BeforeEach(func() {
					botanist.Shoot.GetInfo().Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}
				})

				It("hibernation enabled", func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
						Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
					})).To(Succeed())
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					// no deployment created → NotFound → replicas = 0
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
						Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
					})).To(Succeed())
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(new(int32(1)))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})

			Context("last operation is not nil and of type 'restore'", func() {
				BeforeEach(func() {
					botanist.Shoot.GetInfo().Status.LastOperation = &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}
				})

				It("hibernation enabled", func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
						Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
					})).To(Succeed())
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager deployment does not exist", func() {
					botanist.Shoot.HibernationEnabled = true
					kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation enabled and kube-controller-manager is already scaled up", func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(fakeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
						Spec:       appsv1.DeploymentSpec{Replicas: new(int32(1))},
					})).To(Succeed())
					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})

				It("hibernation disabled", func() {
					kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(new(int32(1)))
					botanist.Shoot.HibernationEnabled = false

					kubeControllerManager.EXPECT().SetReplicaCount(int32(1))
					Expect(botanist.DeployKubeControllerManager(ctx)).To(Succeed())
				})
			})
		})

		It("should fail when the replicas cannot be determined", func() {
			fakeClientWithErr := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fakeErr
				},
			}).Build()
			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClientWithErr).Build()
			Expect(botanist.DeployKubeControllerManager(ctx)).To(MatchError(`failed to check if deployment "foo/kube-controller-manager" is controlled by dependency-watchdog: fake err`))
		})

		It("should fail when the deploy function fails", func() {
			kubeAPIServer.EXPECT().GetAutoscalingReplicas().Return(new(int32(0)))
			kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{RuntimeConfig: map[string]bool{"foo": true}})
			kubeControllerManager.EXPECT().SetReplicaCount(int32(0))
			kubeControllerManager.EXPECT().SetRuntimeConfig(map[string]bool{"foo": true})
			kubeControllerManager.EXPECT().SetServiceNetworks(botanist.Shoot.Networks.Services)
			kubeControllerManager.EXPECT().SetPodNetworks(botanist.Shoot.Networks.Pods)
			kubeControllerManager.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployKubeControllerManager(ctx)).To(Equal(fakeErr))
		})
	})

	Describe("#ScaleKubeControllerManagerToOne", func() {
		BeforeEach(func() {
			botanist.SeedClientSet = kubernetesClientSet
			botanist.Shoot = &shootpkg.Shoot{
				ControlPlaneNamespace: namespace,
			}
		})

		It("should scale the KCM deployment", func() {
			Expect(fakeClient.Create(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: namespace},
				Spec:       appsv1.DeploymentSpec{Replicas: new(int32(0))},
			})).To(Succeed())

			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(Succeed())

			deployment := &appsv1.Deployment{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "kube-controller-manager"}, deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(new(int32(1))))
		})

		It("should fail when the scale call fails", func() {
			fakeClientWithErr := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fakeErr
				},
			}).Build()
			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClientWithErr).Build()
			Expect(botanist.ScaleKubeControllerManagerToOne(ctx)).To(MatchError(fakeErr))
		})
	})

	Describe("#WaitUntilKubeRootCAConfigMapsUpdated", func() {
		var (
			runtimeClient      client.Client
			shootClient        client.Client
			fakeSecretsManager secretsmanager.Interface

			kubeAPIServerNamespace = "shoot--foo--bar"
			bundleContent          = []byte("-----BEGIN CERTIFICATE-----\nMIIBundleCA\n-----END CERTIFICATE-----\n")
		)

		BeforeEach(func() {
			runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).WithIndex(&corev1.ConfigMap{}, "metadata.name", func(obj client.Object) []string {
				return []string{obj.GetName()}
			}).Build()
			fakeSecretsManager = fakesecretsmanager.New(runtimeClient, kubeAPIServerNamespace)

			botanist.Logger = logr.Discard()
			botanist.SecretsManager = fakeSecretsManager
			botanist.ShootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).Build()

			Expect(runtimeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.SecretNameCACluster,
					Namespace: kubeAPIServerNamespace,
				},
				Data: map[string][]byte{
					secretsutils.DataKeyCertificateBundle: bundleContent,
				},
			})).To(Succeed())
		})

		It("should succeed immediately when there are no ConfigMaps", func() {
			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(Succeed())
		})

		It("should succeed immediately when all kube-root-ca.crt ConfigMaps are already labeled", func() {
			Expect(shootClient.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: "ns1",
					Labels:    map[string]string{secretsrotation.LabelKeyCABundleName: v1beta1constants.SecretNameCACluster},
				},
				Data: map[string]string{secretsutils.DataKeyCertificateCA: string(bundleContent)},
			})).To(Succeed())

			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(Succeed())
		})

		It("should label ConfigMaps that contain the expected bundle", func() {
			cm1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: "ns1"},
				Data:       map[string]string{secretsutils.DataKeyCertificateCA: string(bundleContent)},
			}
			Expect(shootClient.Create(ctx, cm1)).To(Succeed())

			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(cm1), cm1)).To(Succeed())
			Expect(cm1.Labels).To(HaveKeyWithValue(secretsrotation.LabelKeyCABundleName, v1beta1constants.SecretNameCACluster))
		})

		It("should return an error when a ConfigMap has no ca.crt data", func() {
			Expect(shootClient.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: "ns1"},
			})).To(Succeed())

			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(MatchError(ContainSubstring("1 kube-root-ca.crt ConfigMap(s) do not yet contain the expected CA bundle")))
		})

		It("should return an error when a ConfigMap does not contain the expected bundle", func() {
			Expect(shootClient.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: "ns1"},
				Data:       map[string]string{secretsutils.DataKeyCertificateCA: "old-ca-only"},
			})).To(Succeed())

			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(MatchError(ContainSubstring("1 kube-root-ca.crt ConfigMap(s) do not yet contain the expected CA bundle")))
		})

		It("should only process unlabeled ConfigMaps and ignore non-kube-root-ca.crt ConfigMaps", func() {
			cmLabeled := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: "ns1",
					Labels:    map[string]string{secretsrotation.LabelKeyCABundleName: v1beta1constants.SecretNameCACluster},
				},
				Data: map[string]string{secretsutils.DataKeyCertificateCA: string(bundleContent)},
			}
			cmUnlabeled := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-root-ca.crt", Namespace: "ns2"},
				Data:       map[string]string{secretsutils.DataKeyCertificateCA: string(bundleContent)},
			}
			cmOtherName := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "some-other-configmap", Namespace: "ns3"},
			}
			Expect(shootClient.Create(ctx, cmLabeled)).To(Succeed())
			Expect(shootClient.Create(ctx, cmUnlabeled)).To(Succeed())
			Expect(shootClient.Create(ctx, cmOtherName)).To(Succeed())

			Expect(botanist.WaitUntilKubeRootCAConfigMapsUpdated(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(cmLabeled), cmLabeled)).To(Succeed())
			Expect(cmLabeled.Labels).To(HaveKeyWithValue(secretsrotation.LabelKeyCABundleName, v1beta1constants.SecretNameCACluster))

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(cmUnlabeled), cmUnlabeled)).To(Succeed())
			Expect(cmUnlabeled.Labels).To(HaveKeyWithValue(secretsrotation.LabelKeyCABundleName, v1beta1constants.SecretNameCACluster))

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(cmOtherName), cmOtherName)).To(Succeed())
			Expect(cmOtherName.Labels).NotTo(HaveKey(secretsrotation.LabelKeyCABundleName))
		})
	})
})
