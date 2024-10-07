// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"
	"sort"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	mockkubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("KubeProxy", func() {
	var (
		ctrl                         *gomock.Controller
		fakeSeedClient               client.Client
		fakeSeedKubernetesInterface  kubernetes.Interface
		fakeShootClient              client.Client
		fakeShootKubernetesInterface kubernetes.Interface
		sm                           secretsmanager.Interface
		botanist                     *Botanist

		namespace             = "shoot--foo--bar"
		apiServerAddress      = "1.2.3.4"
		internalClusterDomain = "example.com"

		repositoryKubeProxyImage = "registry.k8s.io/kube-proxy"

		poolName1                     = "pool1"
		poolName2                     = "pool2"
		poolName3                     = "pool3"
		kubernetesVersionControlPlane = semver.MustParse("1.31.1")
		kubernetesVersionPool2        = semver.MustParse("1.30.4")
		kubernetesVersionPool3        = kubernetesVersionControlPlane
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedKubernetesInterface = kubernetesfake.NewClientSetBuilder().WithClient(fakeSeedClient).Build()
		fakeShootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeShootKubernetesInterface = kubernetesfake.NewClientSetBuilder().WithClient(fakeShootClient).Build()
		sm = fakesecretsmanager.New(fakeSeedClient, namespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(fakeSeedClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

		botanist = &Botanist{
			Operation: &operation.Operation{
				APIServerAddress: apiServerAddress,
				SeedClientSet:    fakeSeedKubernetesInterface,
				ShootClientSet:   fakeShootKubernetesInterface,
				SecretsManager:   sm,
				Shoot: &shootpkg.Shoot{
					InternalClusterDomain: internalClusterDomain,
					KubernetesVersion:     kubernetesVersionControlPlane,
					ControlPlaneNamespace: namespace,
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: kubernetesVersionControlPlane.String(),
				},
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Name: poolName1,
						},
						{
							Name: poolName2,
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To(kubernetesVersionPool2.String()),
							},
						},
						{
							Name: poolName3,
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: ptr.To(kubernetesVersionPool3.String()),
							},
						},
					},
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeProxy", func() {
		It("should successfully create a kube-proxy interface", func() {
			kubeProxy, err := botanist.DefaultKubeProxy()
			Expect(kubeProxy).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeployKubeProxy", func() {
		var (
			kubeProxy *mockkubeproxy.MockInterface

			ctx     = context.TODO()
			fakeErr = errors.New("fake err")
		)

		BeforeEach(func() {
			kubeProxy = mockkubeproxy.NewMockInterface(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					KubeProxy: kubeProxy,
				},
			}
			botanist.Shoot.Networks = &shootpkg.Networks{
				Pods: []net.IPNet{{IP: net.ParseIP("22.23.24.25")}},
			}
			kubeProxy.EXPECT().SetPodNetworkCIDRs(botanist.Shoot.Networks.Pods)

			kubeProxy.EXPECT().SetKubeconfig([]byte(`apiVersion: v1
clusters:
- cluster:
    server: https://api.` + internalClusterDomain + `
  name: ` + namespace + `
contexts:
- context:
    cluster: ` + namespace + `
    user: ` + namespace + `
  name: ` + namespace + `
current-context: ` + namespace + `
kind: Config
preferences: {}
users:
- name: ` + namespace + `
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
`))
		})

		It("should fail when the deploy function fails", func() {
			kubeProxy.EXPECT().SetWorkerPools(gomock.Any())
			kubeProxy.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployKubeProxy(ctx)).To(MatchError(fakeErr))
		})

		Context("successful deployment", func() {
			It("with no still existing worker pools", func() {
				kubeProxy.EXPECT().SetWorkerPools(gomock.AssignableToTypeOf([]kubeproxy.WorkerPool{})).DoAndReturn(func(actual []kubeproxy.WorkerPool) {
					verifyWorkerPools(actual, []kubeproxy.WorkerPool{
						{
							Name:              poolName1,
							KubernetesVersion: kubernetesVersionControlPlane,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionControlPlane.String(),
						},
						{
							Name:              poolName2,
							KubernetesVersion: kubernetesVersionPool2,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool2.String(),
						},
						{
							Name:              poolName3,
							KubernetesVersion: kubernetesVersionPool3,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool3.String(),
						},
					})
				})
				kubeProxy.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeProxy(ctx)).To(Succeed())
			})

			It("with still existing worker pools", func() {
				for _, node := range []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node1",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               poolName1,
								"worker.gardener.cloud/kubernetes-version": kubernetesVersionControlPlane.String(),
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               poolName2,
								"worker.gardener.cloud/kubernetes-version": "1.27.3",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node3",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               poolName2,
								"worker.gardener.cloud/kubernetes-version": kubernetesVersionPool2.String(),
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node4",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               "pool4",
								"worker.gardener.cloud/kubernetes-version": "1.27.3",
							},
						},
					},
				} {
					Expect(fakeShootClient.Create(ctx, node)).To(Succeed())
				}

				kubeProxy.EXPECT().SetWorkerPools(gomock.AssignableToTypeOf([]kubeproxy.WorkerPool{})).DoAndReturn(func(actual []kubeproxy.WorkerPool) {
					verifyWorkerPools(actual, []kubeproxy.WorkerPool{
						{
							Name:              poolName1,
							KubernetesVersion: kubernetesVersionControlPlane,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionControlPlane.String(),
						},
						{
							Name:              poolName2,
							KubernetesVersion: kubernetesVersionPool2,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool2.String(),
						},
						{
							Name:              poolName3,
							KubernetesVersion: kubernetesVersionPool3,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool3.String(),
						},
						{
							Name:              poolName2,
							KubernetesVersion: semver.MustParse("1.27.3"),
							Image:             repositoryKubeProxyImage + ":v1.27.3",
						},
						{
							Name:              "pool4",
							KubernetesVersion: semver.MustParse("1.27.3"),
							Image:             repositoryKubeProxyImage + ":v1.27.3",
						},
					})
				})
				kubeProxy.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeProxy(ctx)).To(Succeed())
			})
		})
	})
})

func verifyWorkerPools(expected, actual []kubeproxy.WorkerPool) {
	getSortFn := func(slice []kubeproxy.WorkerPool) func(i, j int) bool {
		return func(i, j int) bool {
			if slice[i].Name < slice[j].Name {
				return true
			}
			if slice[i].Name > slice[j].Name {
				return false
			}
			return slice[i].KubernetesVersion.String() < slice[j].KubernetesVersion.String()
		}
	}

	sort.Slice(actual, getSortFn(actual))
	sort.Slice(expected, getSortFn(expected))

	Expect(actual).To(Equal(expected))
}
