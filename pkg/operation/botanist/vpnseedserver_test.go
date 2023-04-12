// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
	mockvpnseedserver "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver/mock"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("VPNSeedServer", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{
			Garden: &garden.Garden{},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultVPNSeedServer", func() {
		var kubernetesClient *kubernetesmock.MockInterface

		BeforeEach(func() {
			kubernetesClient = kubernetesmock.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.SeedClientSet = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{
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
			botanist.Seed = &seed.Seed{
				KubernetesVersion: semver.MustParse("1.22.3"),
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
		})

		It("should successfully create a vpn seed server interface", func() {
			defer test.WithFeatureGate(features.DefaultFeatureGate, features.APIServerSNI, true)()
			kubernetesClient.EXPECT().Client()
			kubernetesClient.EXPECT().Version()
			botanist.ImageVector = imagevector.ImageVector{{Name: images.ImageNameVpnSeedServer}, {Name: images.ImageNameApiserverProxy}}

			vpnSeedServer, err := botanist.DefaultVPNSeedServer()
			Expect(vpnSeedServer).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("should correctly set the deployment replicas",
			func(hibernated, highAvailable bool, expectedReplicas int) {
				defer test.WithFeatureGate(features.DefaultFeatureGate, features.APIServerSNI, true)()
				kubernetesClient.EXPECT().Client()
				kubernetesClient.EXPECT().Version()
				botanist.ImageVector = imagevector.ImageVector{{Name: images.ImageNameVpnSeedServer}, {Name: images.ImageNameApiserverProxy}}
				botanist.Shoot.HibernationEnabled = hibernated
				if highAvailable {
					botanist.Shoot.VPNHighAvailabilityEnabled = highAvailable
					botanist.Shoot.VPNHighAvailabilityNumberOfSeedServers = 2
				}

				vpnSeedServer, err := botanist.DefaultVPNSeedServer()
				Expect(vpnSeedServer).NotTo(BeNil())
				Expect(vpnSeedServer.GetValues().Replicas).To(Equal(int32(expectedReplicas)))
				Expect(err).NotTo(HaveOccurred())
			},

			Entry("non-HA & awake", false, false, 1),
			Entry("non-HA & hibernated", true, false, 0),
			Entry("HA & awake", false, true, 2),
			Entry("HA & hibernated", true, true, 0),
		)

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

			secretNameDH     = v1beta1constants.GardenRoleOpenVPNDiffieHellman
			secretChecksumDH = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

			namespaceUID = types.UID("1234")
		)

		BeforeEach(func() {
			vpnSeedServer = mockvpnseedserver.NewMockInterface(ctrl)

			botanist.StoreSecret(secretNameDH, &corev1.Secret{})
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						VPNSeedServer: vpnSeedServer,
					},
				},
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
					UID: "1234",
				},
			}
		})

		BeforeEach(func() {
			vpnSeedServer.EXPECT().SetSecrets(vpnseedserver.Secrets{
				DiffieHellmanKey: component.Secret{Name: secretNameDH, Checksum: secretChecksumDH},
			})
			vpnSeedServer.EXPECT().SetSeedNamespaceObjectUID(namespaceUID)
		})

		It("should set the secrets and SNI config and deploy", func() {
			vpnSeedServer.EXPECT().Deploy(ctx)
			Expect(botanist.DeployVPNServer(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			vpnSeedServer.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployVPNServer(ctx)).To(Equal(fakeErr))
		})
	})
})
