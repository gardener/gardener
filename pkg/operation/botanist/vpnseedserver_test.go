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
	"net"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	mockvpnseedserver "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("VPNSeedServer", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVPNSeedServer", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
				DisableDNS: true,
				Networks: &shootpkg.Networks{
					Services: &net.IPNet{IP: net.IP{10, 0, 0, 1}, Mask: net.CIDRMask(10, 24)},
					Pods:     &net.IPNet{IP: net.IP{10, 0, 0, 2}, Mask: net.CIDRMask(10, 24)},
				},
			}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Nodes: pointer.String("10.0.0.0/24"),
					},
				},
			})
			botanist.Config = &config.GardenletConfiguration{
				SNI: &config.SNI{
					Ingress: &config.SNIIngress{
						Namespace: pointer.String("test-ns"),
						Labels: map[string]string{
							"istio": "foo-bar",
						},
					},
				},
			}
		})

		It("should successfully create a vpn seed server interface", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true)()
			kubernetesClient.EXPECT().Client()
			kubernetesClient.EXPECT().Version()
			botanist.ImageVector = imagevector.ImageVector{{Name: charts.ImageNameVpnSeedServer}, {Name: charts.ImageNameApiserverProxy}}

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the images cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployVPNSeedServer", func() {
		var (
			vpnSeedServer *mockvpnseedserver.MockInterface

			ctx     = context.TODO()
			fakeErr = fmt.Errorf("fake err")

			secretNameTLSAuth     = vpnseedserver.VpnSeedServerTLSAuth
			secretChecksumTLSAuth = "1234"
			secretNameServer      = vpnseedserver.DeploymentName
			secretChecksumServer  = "5678"
			secretNameDH          = v1beta1constants.GardenRoleOpenVPNDiffieHellman
			secretChecksumDH      = "9012"

			namespaceUID = types.UID("1234")
		)

		BeforeEach(func() {
			vpnSeedServer = mockvpnseedserver.NewMockInterface(ctrl)

			botanist.StoreCheckSum(secretNameTLSAuth, secretChecksumTLSAuth)
			botanist.StoreCheckSum(secretNameServer, secretChecksumServer)
			botanist.StoreCheckSum(secretNameDH, secretChecksumDH)
			botanist.StoreSecret(secretNameTLSAuth, &corev1.Secret{})
			botanist.StoreSecret(secretNameServer, &corev1.Secret{})
			botanist.StoreSecret(secretNameDH, &corev1.Secret{})
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						VPNSeedServer: vpnSeedServer,
					},
				},
				ReversedVPNEnabled: true,
			}
			botanist.Config = &config.GardenletConfiguration{
				SNI: &config.SNI{
					Ingress: &config.SNIIngress{
						Namespace: pointer.String("test-ns"),
						Labels: map[string]string{
							"istio": "foo-bar",
						},
					},
				},
			}
			botanist.SeedNamespaceObject = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("1234"),
				},
			}
		})

		BeforeEach(func() {
			vpnSeedServer.EXPECT().SetSecrets(vpnseedserver.Secrets{
				TLSAuth:          component.Secret{Name: secretNameTLSAuth, Checksum: secretChecksumTLSAuth},
				Server:           component.Secret{Name: vpnseedserver.DeploymentName, Checksum: secretChecksumServer},
				DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH},
			})
			vpnSeedServer.EXPECT().SetSeedNamespaceObjectUID(namespaceUID)
			vpnSeedServer.EXPECT().SetSNIConfig(botanist.Config.SNI)
		})

		It("should set the secrets and SNI config and deploy", func() {
			vpnSeedServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNServer(ctx)).To(Succeed())
		})

		It("should set the secrets and the ExposureClass handler config and deploy", func() {
			botanist.ExposureClassHandler = &config.ExposureClassHandler{
				Name: "test",
				SNI:  &config.SNI{},
			}

			vpnSeedServer.EXPECT().SetExposureClassHandlerName(botanist.ExposureClassHandler.Name)
			vpnSeedServer.EXPECT().SetSNIConfig(botanist.ExposureClassHandler.SNI)

			vpnSeedServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			vpnSeedServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNServer(ctx)).To(Equal(fakeErr))
		})
	})
})
