// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/kubeproxy"
	mockkubeproxy "github.com/gardener/gardener/pkg/component/kubeproxy/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
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

		repositoryKubeProxyImage = "foo.bar.com/kube-proxy"

		poolName1                     = "pool1"
		poolName2                     = "pool2"
		poolName3                     = "pool3"
		kubernetesVersionControlPlane = "1.24.3"
		kubernetesVersionPool2        = "1.23.4"
		kubernetesVersionPool3        = kubernetesVersionControlPlane
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedKubernetesInterface = kubernetesfake.NewClientSetBuilder().WithClient(fakeSeedClient).Build()
		fakeShootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeShootKubernetesInterface = kubernetesfake.NewClientSetBuilder().WithClient(fakeShootClient).Build()
		sm = fakesecretsmanager.New(fakeSeedClient, namespace)

		By("Create secrets managed outside of this function for whose secretsmanager.Get() will be called")
		Expect(fakeSeedClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

		botanist = &Botanist{
			Operation: &operation.Operation{
				APIServerAddress: apiServerAddress,
				ImageVector: imagevector.ImageVector{
					{
						Name: "alpine",
					},
					{
						Name:          "kube-proxy",
						Repository:    repositoryKubeProxyImage,
						TargetVersion: pointer.String("1.24.x"),
					},
					{
						Name:          "kube-proxy",
						Repository:    repositoryKubeProxyImage,
						TargetVersion: pointer.String("1.23.x"),
					},
					{
						Name:          "kube-proxy",
						Repository:    repositoryKubeProxyImage,
						TargetVersion: pointer.String("1.22.x"),
					},
				},
				SeedClientSet:  fakeSeedKubernetesInterface,
				ShootClientSet: fakeShootKubernetesInterface,
				SecretsManager: sm,
				Shoot: &shootpkg.Shoot{
					InternalClusterDomain: internalClusterDomain,
					KubernetesVersion:     semver.MustParse(kubernetesVersionControlPlane),
					SeedNamespace:         namespace,
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: kubernetesVersionControlPlane,
				},
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Name: poolName1,
						},
						{
							Name: poolName2,
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: &kubernetesVersionPool2,
							},
						},
						{
							Name: poolName3,
							Kubernetes: &gardencorev1beta1.WorkerKubernetes{
								Version: &kubernetesVersionPool3,
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
		BeforeEach(func() {
			botanist.Shoot.Networks = &shootpkg.Networks{
				Pods: &net.IPNet{IP: net.ParseIP("22.23.24.25")},
			}
		})

		It("should successfully create a kube-proxy interface", func() {
			kubeProxy, err := botanist.DefaultKubeProxy()
			Expect(kubeProxy).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			kubeProxy, err := botanist.DefaultKubeProxy()
			Expect(kubeProxy).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployKubeProxy", func() {
		var (
			kubeProxy *mockkubeproxy.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			kubeProxy = mockkubeproxy.NewMockInterface(ctrl)

			botanist.Shoot.Components = &shootpkg.Components{
				SystemComponents: &shootpkg.SystemComponents{
					KubeProxy: kubeProxy,
				},
			}

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
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionControlPlane,
						},
						{
							Name:              poolName2,
							KubernetesVersion: kubernetesVersionPool2,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool2,
						},
						{
							Name:              poolName3,
							KubernetesVersion: kubernetesVersionPool3,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool3,
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
								"worker.gardener.cloud/kubernetes-version": kubernetesVersionControlPlane,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node2",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               poolName2,
								"worker.gardener.cloud/kubernetes-version": "1.24.3",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node3",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               poolName2,
								"worker.gardener.cloud/kubernetes-version": kubernetesVersionPool2,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node4",
							Labels: map[string]string{
								"worker.gardener.cloud/pool":               "pool4",
								"worker.gardener.cloud/kubernetes-version": "1.24.3",
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
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionControlPlane,
						},
						{
							Name:              poolName2,
							KubernetesVersion: kubernetesVersionPool2,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool2,
						},
						{
							Name:              poolName3,
							KubernetesVersion: kubernetesVersionPool3,
							Image:             repositoryKubeProxyImage + ":v" + kubernetesVersionPool3,
						},
						{
							Name:              poolName2,
							KubernetesVersion: "1.24.3",
							Image:             repositoryKubeProxyImage + ":v1.24.3",
						},
						{
							Name:              "pool4",
							KubernetesVersion: "1.24.3",
							Image:             repositoryKubeProxyImage + ":v1.24.3",
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
			return slice[i].KubernetesVersion < slice[j].KubernetesVersion
		}
	}

	sort.Slice(actual, getSortFn(actual))
	sort.Slice(expected, getSortFn(expected))

	Expect(actual).To(Equal(expected))
}
