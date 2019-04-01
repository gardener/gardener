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
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/garden"
	. "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("shoot", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		shoot *Shoot
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		shoot = &Shoot{
			Info: &gardenv1beta1.Shoot{},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#IPVSEnabled", func() {
		It("should return false when KubeProxy is null", func() {
			shoot.Info.Spec.Kubernetes.KubeProxy = nil

			Expect(shoot.IPVSEnabled()).To(BeFalse())
		})

		It("should return false when KubeProxy.Mode is null", func() {
			shoot.Info.Spec.Kubernetes.KubeProxy = &gardenv1beta1.KubeProxyConfig{}
			Expect(shoot.IPVSEnabled()).To(BeFalse())
		})

		It("should return false when KubeProxy.Mode is not IPVS", func() {
			mode := gardenv1beta1.ProxyModeIPTables
			shoot.Info.Spec.Kubernetes.KubeProxy = &gardenv1beta1.KubeProxyConfig{
				Mode: &mode,
			}
			Expect(shoot.IPVSEnabled()).To(BeFalse())
		})

		It("should return true when KubeProxy.Mode is IPVS", func() {
			mode := gardenv1beta1.ProxyModeIPVS
			shoot.Info.Spec.Kubernetes.KubeProxy = &gardenv1beta1.KubeProxyConfig{
				Mode: &mode,
			}
			Expect(shoot.IPVSEnabled()).To(BeTrue())
		})
	})

	DescribeTable("#ConstructInternalClusterDomain",
		func(shootName, shootProject, internalDomain, expected string) {
			Expect(ConstructInternalClusterDomain(shootName, shootProject, internalDomain)).To(Equal(expected))
		},

		Entry("with internal domain key", "foo", "bar", "internal.nip.io", "api.foo.bar.internal.nip.io"),
		Entry("without internal domain key", "foo", "bar", "nip.io", "api.foo.bar.internal.nip.io"),
	)

	Describe("#ConstructExternalClusterDomain", func() {
		It("should return nil", func() {
			Expect(ConstructExternalClusterDomain(&gardenv1beta1.Shoot{})).To(BeNil())
		})

		It("should return the constructed domain", func() {
			var (
				domain         = "foo.bar.com"
				expectedDomain = fmt.Sprintf("api.%s", domain)
				shoot          = &gardenv1beta1.Shoot{
					Spec: gardenv1beta1.ShootSpec{
						DNS: gardenv1beta1.DNS{
							Domain: &domain,
						},
					},
				}
			)

			Expect(ConstructExternalClusterDomain(shoot)).To(Equal(&expectedDomain))
		})
	})

	var (
		defaultDomainProvider   = "default-domain-provider"
		defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
		defaultDomain           = &garden.DefaultDomain{
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
				shoot = &gardenv1beta1.Shoot{}
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

				shoot = &gardenv1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: gardenv1beta1.ShootSpec{
						DNS: gardenv1beta1.DNS{
							Provider:   &provider,
							Domain:     &domain,
							SecretName: &dnsSecretName,
						},
					},
				}
			)

			c.EXPECT().Get(ctx, dnsSecretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
				secret.Data = dnsSecretData
				return nil
			})

			externalDomain, err := ConstructExternalDomain(ctx, c, shoot, nil, nil)

			Expect(externalDomain).To(Equal(&ExternalDomain{
				Domain:     domain,
				Provider:   provider,
				SecretData: dnsSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the default domain secret", func() {
			var (
				ctx = context.TODO()

				shoot = &gardenv1beta1.Shoot{
					Spec: gardenv1beta1.ShootSpec{
						DNS: gardenv1beta1.DNS{
							Provider: &provider,
							Domain:   &domain,
						},
					},
				}
			)

			externalDomain, err := ConstructExternalDomain(ctx, c, shoot, nil, []*garden.DefaultDomain{defaultDomain})

			Expect(externalDomain).To(Equal(&ExternalDomain{
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
				shoot           = &gardenv1beta1.Shoot{
					Spec: gardenv1beta1.ShootSpec{
						DNS: gardenv1beta1.DNS{
							Provider: &provider,
							Domain:   &domain,
						},
					},
				}
			)

			externalDomain, err := ConstructExternalDomain(ctx, c, shoot, shootSecret, nil)

			Expect(externalDomain).To(Equal(&ExternalDomain{
				Domain:     domain,
				Provider:   provider,
				SecretData: shootSecretData,
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
