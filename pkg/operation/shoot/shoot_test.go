// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"
	"net"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/garden"
	. "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("shoot", func() {
	Context("shoot", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			shoot *Shoot
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			shoot = &Shoot{}
			shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#ToNetworks", func() {
			var shoot *gardencorev1beta1.Shoot

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: gardencorev1beta1.Networking{
							Pods:     pointer.String("10.0.0.0/24"),
							Services: pointer.String("20.0.0.0/24"),
						},
					},
				}
			})

			It("returns correct network", func() {
				result, err := ToNetworks(shoot)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(PointTo(Equal(Networks{
					Pods: &net.IPNet{
						IP:   []byte{10, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					},
					Services: &net.IPNet{
						IP:   []byte{20, 0, 0, 0},
						Mask: []byte{255, 255, 255, 0},
					},
					APIServer: []byte{20, 0, 0, 1},
					CoreDNS:   []byte{20, 0, 0, 10},
				})))
			})

			DescribeTable("#ConstructInternalClusterDomain", func(mutateFunc func(s *gardencorev1beta1.Shoot)) {
				mutateFunc(shoot)
				result, err := ToNetworks(shoot)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			},

				Entry("services is nil", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Services = nil }),
				Entry("pods is nil", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Pods = nil }),
				Entry("services is invalid", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = pointer.String("foo")
				}),
				Entry("pods is invalid", func(s *gardencorev1beta1.Shoot) { s.Spec.Networking.Pods = pointer.String("foo") }),
				Entry("apiserver cannot be calculated", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = pointer.String("10.0.0.0/32")
				}),
				Entry("coreDNS cannot be calculated", func(s *gardencorev1beta1.Shoot) {
					s.Spec.Networking.Services = pointer.String("10.0.0.0/29")
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

		DescribeTable("#ConstructInternalClusterDomain",
			func(shootName, shootProject, internalDomain, expected string) {
				Expect(ConstructInternalClusterDomain(shootName, shootProject, &garden.Domain{Domain: internalDomain})).To(Equal(expected))
			},

			Entry("with internal domain key", "foo", "bar", "internal.nip.io", "foo.bar.internal.nip.io"),
			Entry("without internal domain key", "foo", "bar", "nip.io", "foo.bar.internal.nip.io"),
		)

		Describe("#ConstructExternalClusterDomain", func() {
			It("should return nil", func() {
				Expect(ConstructExternalClusterDomain(&gardencorev1beta1.Shoot{})).To(BeNil())
			})

			It("should return the constructed domain", func() {
				var (
					domain = "foo.bar.com"
					shoot  = &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: &domain,
							},
						},
					}
				)

				Expect(ConstructExternalClusterDomain(shoot)).To(Equal(&domain))
			})
		})

		var (
			defaultDomainProvider   = "default-domain-provider"
			defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
			defaultDomain           = &garden.Domain{
				Domain:     "bar.com",
				Provider:   defaultDomainProvider,
				SecretData: defaultDomainSecretData,
			}
		)

		Describe("#ConstructExternalDomain", func() {
			var (
				namespace = "default"
				provider  = "my-dns-provider"
				domain    = "foo.bar.com"
			)

			It("returns nil because no external domain is used", func() {
				var (
					ctx   = context.TODO()
					shoot = &gardencorev1beta1.Shoot{}
				)

				externalDomain, err := ConstructExternalDomain(ctx, c, shoot, nil, nil)

				Expect(externalDomain).To(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the referenced secret", func() {
				var (
					ctx = context.TODO()

					dnsSecretName = "my-secret"
					dnsSecretData = map[string][]byte{"foo": []byte("bar")}
					dnsSecretKey  = kutil.Key(namespace, dnsSecretName)

					shoot = &gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
						},
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: &domain,
								Providers: []gardencorev1beta1.DNSProvider{
									{
										Type:       &provider,
										SecretName: &dnsSecretName,
										Primary:    pointer.Bool(true),
									},
								},
							},
						},
					}
				)

				c.EXPECT().Get(ctx, dnsSecretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.Data = dnsSecretData
					return nil
				})

				externalDomain, err := ConstructExternalDomain(ctx, c, shoot, nil, nil)

				Expect(externalDomain).To(Equal(&garden.Domain{
					Domain:     domain,
					Provider:   provider,
					SecretData: dnsSecretData,
				}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the default domain secret", func() {
				var (
					ctx = context.TODO()

					shoot = &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: &domain,
								Providers: []gardencorev1beta1.DNSProvider{
									{
										Type: &provider,
									},
								},
							},
						},
					}
				)

				externalDomain, err := ConstructExternalDomain(ctx, c, shoot, nil, []*garden.Domain{defaultDomain})

				Expect(externalDomain).To(Equal(&garden.Domain{
					Domain:     domain,
					Provider:   defaultDomainProvider,
					SecretData: defaultDomainSecretData,
				}))
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the shoot secret", func() {
				var (
					ctx = context.TODO()

					shootSecretData = map[string][]byte{"foo": []byte("bar")}
					shootSecret     = &corev1.Secret{Data: shootSecretData}
					shoot           = &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: &domain,
								Providers: []gardencorev1beta1.DNSProvider{
									{
										Type:    &provider,
										Primary: pointer.Bool(true),
									},
								},
							},
						},
					}
				)

				externalDomain, err := ConstructExternalDomain(ctx, c, shoot, shootSecret, nil)

				Expect(externalDomain).To(Equal(&garden.Domain{
					Domain:     domain,
					Provider:   provider,
					SecretData: shootSecretData,
				}))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("#ComputeInClusterAPIServerAddress", func() {
			seedNamespace := "foo"
			s := &Shoot{SeedNamespace: seedNamespace}

			It("should return <service-name>", func() {
				Expect(s.ComputeInClusterAPIServerAddress(true)).To(Equal(v1beta1constants.DeploymentNameKubeAPIServer))
			})

			It("should return <service-name>.<namespace>.svc", func() {
				Expect(s.ComputeInClusterAPIServerAddress(false)).To(Equal(v1beta1constants.DeploymentNameKubeAPIServer + "." + seedNamespace + ".svc"))
			})
		})

		Describe("#ComputeOutOfClusterAPIServerAddress", func() {
			It("should return the apiserver address as DNS is disabled", func() {
				s := &Shoot{DisableDNS: true}
				apiServerAddress := "abcd"

				Expect(s.ComputeOutOfClusterAPIServerAddress(apiServerAddress, false)).To(Equal(apiServerAddress))
			})

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

				Expect(s.ComputeOutOfClusterAPIServerAddress("", false)).To(Equal("api." + internalDomain))
			})

			It("should return the internal domain as requested (shoot's external domain is not unmanaged)", func() {
				internalDomain := "foo"
				s := &Shoot{
					InternalClusterDomain: internalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{})

				Expect(s.ComputeOutOfClusterAPIServerAddress("", true)).To(Equal("api." + internalDomain))
			})

			It("should return the external domain as requested (shoot's external domain is not unmanaged)", func() {
				externalDomain := "foo"
				s := &Shoot{
					ExternalClusterDomain: &externalDomain,
				}
				s.SetInfo(&gardencorev1beta1.Shoot{})

				Expect(s.ComputeOutOfClusterAPIServerAddress("", false)).To(Equal("api." + externalDomain))
			})
		})
	})

	Describe("#ComputeRequiredExtensions", func() {
		const (
			backupProvider       = "backupprovider"
			seedProvider         = "seedprovider"
			shootProvider        = "providertype"
			networkingType       = "networkingtype"
			extensionType1       = "extension1"
			extensionType2       = "extension2"
			extensionType3       = "extension3"
			oscType              = "osctype"
			containerRuntimeType = "containerruntimetype"
			dnsProviderType1     = "dnsprovider1"
			dnsProviderType2     = "dnsprovider2"
			dnsProviderType3     = "dnsprovider3"
		)

		var (
			shoot                      *gardencorev1beta1.Shoot
			seed                       *gardencorev1beta1.Seed
			controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList
			internalDomain             *garden.Domain
			externalDomain             *garden.Domain
		)

		BeforeEach(func() {
			controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
				Items: []gardencorev1beta1.ControllerRegistration{
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: extensionsv1alpha1.ContainerRuntimeResource,
									Type: extensionType3,
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: extensionsv1alpha1.ExtensionResource,
									Type: extensionType1,
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:            extensionsv1alpha1.ExtensionResource,
									Type:            extensionType2,
									GloballyEnabled: pointer.Bool(true),
								},
							},
						},
					},
				},
			}
			internalDomain = &garden.Domain{Provider: dnsProviderType1}
			externalDomain = &garden.Domain{Provider: dnsProviderType2}
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.SeedBackup{
						Provider: backupProvider,
					},
					Provider: gardencorev1beta1.SeedProvider{
						Type: seedProvider,
					},
					Settings: &gardencorev1beta1.SeedSettings{
						ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
							Enabled: true,
						},
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: shootProvider,
						Workers: []gardencorev1beta1.Worker{
							{
								Machine: gardencorev1beta1.Machine{
									Image: &gardencorev1beta1.ShootMachineImage{
										Name: oscType,
									},
								},
								CRI: &gardencorev1beta1.CRI{
									ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{
										{Type: containerRuntimeType},
									},
								},
							},
						},
					},
					Networking: gardencorev1beta1.Networking{
						Type: networkingType,
					},
					Extensions: []gardencorev1beta1.Extension{
						{Type: extensionType1},
					},
					DNS: &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{
							{Type: pointer.String(dnsProviderType3)},
						},
					},
				},
			}
		})

		Context("when not using DNSRecords", func() {
			It("should compute the correct list of required extensions", func() {
				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, false)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType1),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (no seed backup)", func() {
				seed.Spec.Backup = nil

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, false)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType1),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (seed disables DNS)", func() {
				seed.Spec.Settings.ShootDNS.Enabled = false

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, false)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (shoot explicitly disables globally enabled extension)", func() {
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, gardencorev1beta1.Extension{
					Type:     extensionType2,
					Disabled: pointer.Bool(true),
				})

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, false)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType1),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
				)))
			})
		})

		Context("when using DNSRecords", func() {
			It("should compute the correct list of required extensions", func() {
				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, true)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (no seed backup)", func() {
				seed.Spec.Backup = nil

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, true)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (seed disables DNS)", func() {
				seed.Spec.Settings.ShootDNS.Enabled = false

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, true)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType2),
				)))
			})

			It("should compute the correct list of required extensions (shoot explicitly disables globally enabled extension)", func() {
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, gardencorev1beta1.Extension{
					Type:     extensionType2,
					Disabled: pointer.Bool(true),
				})

				result := ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain, true)

				Expect(result).To(Equal(sets.NewString(
					extensions.Id(extensionsv1alpha1.BackupBucketResource, backupProvider),
					extensions.Id(extensionsv1alpha1.BackupEntryResource, backupProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, seedProvider),
					extensions.Id(extensionsv1alpha1.ControlPlaneResource, shootProvider),
					extensions.Id(extensionsv1alpha1.InfrastructureResource, shootProvider),
					extensions.Id(extensionsv1alpha1.NetworkResource, networkingType),
					extensions.Id(extensionsv1alpha1.WorkerResource, shootProvider),
					extensions.Id(extensionsv1alpha1.ExtensionResource, extensionType1),
					extensions.Id(extensionsv1alpha1.OperatingSystemConfigResource, oscType),
					extensions.Id(extensionsv1alpha1.ContainerRuntimeResource, containerRuntimeType),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType1),
					extensions.Id(extensionsv1alpha1.DNSRecordResource, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType2),
					extensions.Id(dnsv1alpha1.DNSProviderKind, dnsProviderType3),
				)))
			})
		})
	})
})
