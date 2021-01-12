// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mockkonnectivity "github.com/gardener/gardener/pkg/mock/gardener/operation/botanist/controlplane/konnectivity"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"
	"github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KonnectivityServer", func() {
	var (
		botanist     *Botanist
		c            client.Client
		ctrl         *gomock.Controller
		ctx          context.Context
		scheme       *runtime.Scheme
		serverSecret konnectivity.ServerSecrets
		mockClient   *mockkubernetes.MockInterface
	)

	const (
		kubeconfigSecretName = "konnectivity-server-kubeconfig"
		kubeconfigChecksum   = "123"
		serverSecretName     = "konnectivity-server"
		serverChecksum       = "456"
		clientCASecretName   = "konnectivity-server-ca"
		clientCAChecksum     = "789"
	)

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.CheckSums = map[string]string{
			kubeconfigSecretName: kubeconfigChecksum,
			serverSecretName:     serverChecksum,
			clientCASecretName:   clientCAChecksum,
		}

		serverSecret = konnectivity.ServerSecrets{
			Kubeconfig: component.Secret{Name: kubeconfigSecretName, Checksum: kubeconfigChecksum},
			ClientCA:   component.Secret{Name: clientCASecretName, Checksum: clientCAChecksum},
			Server:     component.Secret{Name: serverSecretName, Checksum: serverChecksum},
		}

		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(resourcesv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		mockClient = mockkubernetes.NewMockInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKonnectivityServer", func() {
		BeforeEach(func() {
			botanist.K8sSeedClient = mockClient
			botanist.Shoot = &shootpkg.Shoot{
				SeedNamespace: "bar",
			}
			botanist.ImageVector = imagevector.ImageVector{{
				Name:       "konnectivity-server",
				Repository: "foo.bar",
			}}
		})

		Context("when konnectivity is disabled", func() {
			BeforeEach(func() {
				botanist.Shoot.KonnectivityTunnelEnabled = false
				c = fake.NewFakeClientWithScheme(
					scheme,
					&resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "konnectivity-server",
							Namespace: "bar",
						},
					},
				)

				mockClient.EXPECT().Client().Return(c)
			})

			It("should call Destroy when calling Deploy", func() {
				konnectivityServer, err := botanist.DefaultKonnectivityServer()
				Expect(err).NotTo(HaveOccurred())
				Expect(konnectivityServer).NotTo(BeNil())

				Expect(c.Get(
					ctx,
					types.NamespacedName{Name: "konnectivity-server", Namespace: "bar"},
					&resourcesv1alpha1.ManagedResource{},
				)).ToNot(HaveOccurred())

				konnectivityServer.SetSecrets(serverSecret)
				Expect(konnectivityServer.Deploy(ctx)).To(Succeed())

				Expect(c.Get(
					ctx,
					types.NamespacedName{Name: "konnectivity-server", Namespace: "bar"},
					&resourcesv1alpha1.ManagedResource{},
				)).To(BeNotFoundError())
			})
		})

		Context("when konnectivty is enabled", func() {
			BeforeEach(func() {
				gardenletfeatures.RegisterFeatureGates()

				botanist.Shoot.KonnectivityTunnelEnabled = true
				botanist.Shoot.Info = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{Domain: pointer.StringPtr("foo")},
					},
				}
				botanist.Shoot.DisableDNS = false
				botanist.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
				botanist.Shoot.ExternalClusterDomain = pointer.StringPtr("foo.bar")
				botanist.Shoot.InternalClusterDomain = "baz.bar"
				botanist.Garden = &garden.Garden{
					InternalDomain: &garden.Domain{Provider: "valid-provider"},
				}
				Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=true")).ToNot(HaveOccurred())
				botanist.Config = &config.GardenletConfiguration{
					SNI: &config.SNI{
						Ingress: &config.SNIIngress{
							Labels: map[string]string{"foo": "bar"},
						},
					},
				}
			})

			Context("successfully create a konnectivity-server interface", func() {
				BeforeEach(func() {
					c = fake.NewFakeClientWithScheme(scheme)

					mockClient.EXPECT().Client().Return(c)
				})

				It("returns non-nil konnectivity-server", func() {
					konnectivityServer, err := botanist.DefaultKonnectivityServer()
					Expect(err).NotTo(HaveOccurred())
					Expect(konnectivityServer).NotTo(BeNil())
				})

				It("should create managed resource", func() {
					konnectivityServer, err := botanist.DefaultKonnectivityServer()
					Expect(err).NotTo(HaveOccurred())
					Expect(konnectivityServer).NotTo(BeNil())

					konnectivityServer.SetSecrets(serverSecret)
					Expect(konnectivityServer.Deploy(ctx)).To(Succeed())

					Expect(c.Get(
						ctx,
						types.NamespacedName{Name: "konnectivity-server", Namespace: "bar"},
						&resourcesv1alpha1.ManagedResource{},
					)).To(Succeed())
				})
			})

			It("should return an error because the image cannot be found", func() {
				botanist.ImageVector = imagevector.ImageVector{}

				konnectivityServer, err := botanist.DefaultKonnectivityServer()
				Expect(konnectivityServer).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("#DeployKonnectivityServer", func() {
		var (
			mockServer *mockkonnectivity.MockKonnectivityServer
			fakeErr    = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			mockServer = mockkonnectivity.NewMockKonnectivityServer(ctrl)

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KonnectivityServer: mockServer,
					},
				},
			}

			mockServer.EXPECT().SetSecrets(serverSecret)
		})

		It("should set the secrets and deploy", func() {
			mockServer.EXPECT().Deploy(ctx).Times(1)
			Expect(botanist.DeployKonnectivityServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			mockServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployKonnectivityServer(ctx)).To(Equal(fakeErr))
		})
	})
})
