// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockkubeproxy "github.com/gardener/gardener/pkg/operation/botanist/component/kubeproxy/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("KubeProxy", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist

		namespace             = "shoot--foo--bar"
		apiServerAddress      = "1.2.3.4"
		internalClusterDomain = "example.com"
		caCert                = []byte("cert")
		caSecret              = &corev1.Secret{Data: map[string][]byte{"ca.crt": caCert}}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{
			Operation: &operation.Operation{
				APIServerAddress: apiServerAddress,
				Shoot: &shootpkg.Shoot{
					InternalClusterDomain: internalClusterDomain,
					SeedNamespace:         namespace,
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		botanist.StoreSecret("ca", caSecret)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeProxy", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot.Networks = &shootpkg.Networks{
				Pods: &net.IPNet{IP: net.ParseIP("22.23.24.25")},
			}
		})

		It("should successfully create a kube-proxy interface", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: "alpine"}}

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
    certificate-authority-data: ` + utils.EncodeBase64(caCert) + `
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
			kubeProxy.EXPECT().Deploy(ctx).Return(fakeErr)

			Expect(botanist.DeployKubeProxy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy", func() {
			kubeProxy.EXPECT().Deploy(ctx)

			Expect(botanist.DeployKubeProxy(ctx)).To(Succeed())
		})
	})
})
