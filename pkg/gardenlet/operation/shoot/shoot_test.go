// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("shoot", func() {
	Context("shoot", func() {
		var shoot *Shoot

		BeforeEach(func() {
			shoot = &Shoot{}
			shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		Describe("#ToNetworks", func() {
			var shoot *gardencorev1beta1.Shoot

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: &gardencorev1beta1.Networking{
							Pods:     ptr.To("10.0.0.0/24"),
							Services: ptr.To("20.0.0.0/24"),
							Nodes:    ptr.To("30.0.0.0/24"),
						},
					},
				}
			})

			It("returns correct network", func() {
				result, err := ToNetworks(shoot, false)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(PointTo(Equal(Networks{
					Pods: []net.IPNet{{
						IP:   []byte{10, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					}},
					Services: []net.IPNet{{
						IP:   []byte{20, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					}},
					Nodes: []net.IPNet{{
						IP:   []byte{30, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					}},
					APIServer: []net.IP{[]byte{20, 0, 0, 1}},
					CoreDNS:   []net.IP{[]byte{20, 0, 0, 10}},
				})))
			})

			It("returns correct network (workerless Shoot)", func() {
				shoot.Spec.Networking.Pods = nil
				shoot.Spec.Networking.Nodes = nil
				result, err := ToNetworks(shoot, true)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(PointTo(Equal(Networks{
					Pods: nil,
					Services: []net.IPNet{{
						IP:   []byte{20, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					}},
					APIServer: []net.IP{[]byte{20, 0, 0, 1}},
					CoreDNS:   []net.IP{[]byte{20, 0, 0, 10}},
				})))
			})

			It("returns err if serviceCIDR is nil (workerless Shoot)", func() {
				shoot.Spec.Networking.Pods = nil
				shoot.Spec.Networking.Services = nil
				result, err := ToNetworks(shoot, true)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("returns correct joined networks if shoot status is set", func() {
				shoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
					Pods:     []string{"11.0.0.0/24", "12.0.0.0/24", "10.0.0.0/24"},
					Services: []string{"20.0.0.0/24", "2001:db8::/64"},
					Nodes:    []string{"30.0.0.0/24", "2001:db8::/64"},
				}
				shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4}
				result, err := ToNetworks(shoot, false)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(PointTo(Equal(Networks{
					Pods: []net.IPNet{
						{
							IP:   []byte{10, 0, 0, 0},
							Mask: []byte{255, 255, 255, 0},
						},
						{
							IP:   []byte{11, 0, 0, 0},
							Mask: []byte{255, 255, 255, 0},
						},
						{
							IP:   []byte{12, 0, 0, 0},
							Mask: []byte{255, 255, 255, 0},
						},
					},
					Services: []net.IPNet{
						{
							IP:   []byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
							Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0},
						},
						{
							IP:   []byte{20, 0, 0, 0},
							Mask: []byte{255, 255, 255, 0},
						},
					},
					Nodes: []net.IPNet{
						{
							IP:   []byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
							Mask: []byte{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0},
						},
						{
							IP:   []byte{30, 0, 0, 0},
							Mask: []byte{255, 255, 255, 0},
						},
					},
					APIServer: []net.IP{[]byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, []byte{20, 0, 0, 1}},
					CoreDNS:   []net.IP{[]byte{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10}, []byte{20, 0, 0, 10}},
				})))
			})

			DescribeTable("#ConstructInternalClusterDomain", func(mutateFunc func(s *gardencorev1beta1.Shoot)) {
				mutateFunc(shoot)
				result, err := ToNetworks(shoot, false)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			},

				Entry("services is nil", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Services = nil }),
				Entry("pods is nil", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Pods = nil }),
				Entry("services is invalid", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = ptr.To("foo")
				}),
				Entry("pods is invalid", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Pods = ptr.To("foo") }),
				Entry("apiserver cannot be calculated", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = ptr.To("10.0.0.0/32")
				}),
				Entry("coreDNS cannot be calculated", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = ptr.To("10.0.0.0/29")
				}),
			)
		})

		Describe("#IPVSEnabled", func() {
			It("should return false when KubeProxy is null", func() {
				shoot.GetInfo().Spec.Kubernetes.KubeProxy = nil
				Expect(shoot.IPVSEnabled()).To(BeFalse())
			})

			It("should return false when KubeProxy.Mode is null", func() {
				shoot.GetInfo().Spec.Kubernetes.KubeProxy = &gardencorev1beta1.KubeProxyConfig{}
				Expect(shoot.IPVSEnabled()).To(BeFalse())
			})

			It("should return false when KubeProxy.Mode is not IPVS", func() {
				mode := gardencorev1beta1.ProxyModeIPTables
				shoot.GetInfo().Spec.Kubernetes.KubeProxy = &gardencorev1beta1.KubeProxyConfig{
					Mode: &mode,
				}
				Expect(shoot.IPVSEnabled()).To(BeFalse())
			})

			It("should return true when KubeProxy.Mode is IPVS", func() {
				mode := gardencorev1beta1.ProxyModeIPVS
				shoot.GetInfo().Spec.Kubernetes.KubeProxy = &gardencorev1beta1.KubeProxyConfig{
					Mode: &mode,
				}
				Expect(shoot.IPVSEnabled()).To(BeTrue())
			})
		})

		Describe("#ComputeInClusterAPIServerAddress", func() {
			When("shoot is not autonomous", func() {
				controlPlaneNamespace := "foo"
				s := &Shoot{ControlPlaneNamespace: controlPlaneNamespace}

				It("should return <service-name>", func() {
					Expect(s.ComputeInClusterAPIServerAddress(true)).To(Equal(v1beta1constants.DeploymentNameKubeAPIServer))
				})

				It("should return <service-name>.<namespace>.svc", func() {
					Expect(s.ComputeInClusterAPIServerAddress(false)).To(Equal(v1beta1constants.DeploymentNameKubeAPIServer + "." + controlPlaneNamespace + ".svc"))
				})
			})

			When("shoot is autonomous", func() {
				s := &Shoot{ControlPlaneNamespace: "kube-system"}

				It("should return kubernetes.default.svc", func() {
					Expect(s.ComputeInClusterAPIServerAddress(true)).To(Equal("kubernetes.default.svc"))
					Expect(s.ComputeInClusterAPIServerAddress(false)).To(Equal("kubernetes.default.svc"))
				})
			})
		})

		Describe("#ComputeOutOfClusterAPIServerAddress", func() {
			It("should return the internal domain as shoot's external domain is unmanaged", func() {
				unmanaged := "unmanaged"
				internalDomain := "foo"
				s := &Shoot{
					InternalClusterDomain: internalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Providers: []gardencorev1beta1.DNSProvider{
								{Type: &unmanaged},
							},
						},
					},
				})

				Expect(s.ComputeOutOfClusterAPIServerAddress(false)).To(Equal("api." + internalDomain))
			})

			It("should return the internal domain as requested (shoot's external domain is not unmanaged)", func() {
				internalDomain := "foo"
				s := &Shoot{
					InternalClusterDomain: internalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{})

				Expect(s.ComputeOutOfClusterAPIServerAddress(true)).To(Equal("api." + internalDomain))
			})

			It("should return the internal domain when shoot's external domain is not set (even if not requested)", func() {
				internalDomain := "foo"
				s := &Shoot{
					InternalClusterDomain: internalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{})

				Expect(s.ComputeOutOfClusterAPIServerAddress(false)).To(Equal("api." + internalDomain))
			})

			It("should return the external domain as requested (shoot's external domain is not unmanaged)", func() {
				externalDomain := "foo"
				s := &Shoot{
					ExternalClusterDomain: &externalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{})

				Expect(s.ComputeOutOfClusterAPIServerAddress(false)).To(Equal("api." + externalDomain))
			})
		})

		Describe("#IsAutonomous", func() {
			It("should return true", func() {
				shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{
								Name:         "control-plane",
								ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							}},
						},
					},
				})
				Expect(shoot.IsAutonomous()).To(BeTrue())
			})

			It("should return false", func() {
				shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Provider: gardencorev1beta1.Provider{
							Workers: []gardencorev1beta1.Worker{{
								Name: "worker",
							}},
						},
					},
				})
				Expect(shoot.IsAutonomous()).To(BeFalse())
			})
		})

		Describe("#RunsControlPlane", func() {
			It("should return true", func() {
				Expect((&Shoot{ControlPlaneNamespace: "kube-system"}).RunsControlPlane()).To(BeTrue())
			})

			It("should return false", func() {
				Expect((&Shoot{ControlPlaneNamespace: "shoot--foo--bar"}).RunsControlPlane()).To(BeFalse())
			})
		})
	})
})
