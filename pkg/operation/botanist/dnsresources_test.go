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

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("dnsrecord", func() {
	var (
		ctrl *gomock.Controller

		externalDNSRecord *mockdnsrecord.MockInterface
		internalDNSRecord *mockdnsrecord.MockInterface
		ingressDNSRecord  *mockdnsrecord.MockInterface
		ownerDNSRecord    *mockdnsrecord.MockInterface

		b *Botanist

		ctx = context.TODO()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		ingressDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		ownerDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		b = &Botanist{
			Operation: &operation.Operation{
				Shoot: &shoot.Shoot{
					ExternalClusterDomain: pointer.String(externalDomain),
					ExternalDomain: &gardenerutils.Domain{
						Provider: externalProvider,
					},
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ExternalDNSRecord: externalDNSRecord,
							InternalDNSRecord: internalDNSRecord,
							IngressDNSRecord:  ingressDNSRecord,
							OwnerDNSRecord:    ownerDNSRecord,
						},
					},
				},
				Seed: &seed.Seed{},

				Garden: &garden.Garden{
					InternalDomain: &gardenerutils.Domain{
						Provider: internalProvider,
					},
				},
				Logger: logr.Discard(),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				DNS: &gardencorev1beta1.DNS{
					Domain: pointer.String(externalDomain),
				},
				Addons: &gardencorev1beta1.Addons{
					NginxIngress: &gardencorev1beta1.NginxIngress{
						Addon: gardencorev1beta1.Addon{
							Enabled: true,
						},
					},
				},
			},
		})
		b.Seed.SetInfo(&gardencorev1beta1.Seed{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployOrDestroyExternalDNSRecord", func() {
		It("should deploy the external DNSRecord resource", func() {
			gomock.InOrder(
				externalDNSRecord.EXPECT().Deploy(ctx),
				externalDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#DeployOrDestroyIngressDNSRecord", func() {
		It("should deploy the ingress DNSRecord resource", func() {
			gomock.InOrder(
				ingressDNSRecord.EXPECT().Deploy(ctx),
				ingressDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#DeployOwnerDNSResources", func() {
		It("should deploy the owner DNSRecord resource", func() {
			gomock.InOrder(
				ownerDNSRecord.EXPECT().Deploy(ctx),
				ownerDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployOwnerDNSResources(ctx)).To(Succeed())
		})

		It("should delete the owner DNSRecord resource if owner checks are disabled", func() {
			b.Seed.GetInfo().Spec.Settings = &gardencorev1beta1.SeedSettings{
				OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
					Enabled: false,
				},
			}
			gomock.InOrder(
				ownerDNSRecord.EXPECT().Destroy(ctx),
				ownerDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DeployOwnerDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyInternalDNSRecord", func() {
		It("should delete the internal DNS record", func() {
			gomock.InOrder(
				internalDNSRecord.EXPECT().Destroy(ctx),
				internalDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyExternalDNSRecord", func() {
		It("should delete the external DNS record", func() {
			gomock.InOrder(
				externalDNSRecord.EXPECT().Destroy(ctx),
				externalDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyIngressDNSRecord", func() {
		It("should delete the ingress DNS record", func() {
			gomock.InOrder(
				ingressDNSRecord.EXPECT().Destroy(ctx),
				ingressDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyIngressDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyOwnerDNSResources", func() {
		It("should delete the owner DNSRecord resource", func() {
			gomock.InOrder(
				ownerDNSRecord.EXPECT().Destroy(ctx),
				ownerDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyOwnerDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateInternalDNSRecord", func() {
		It("should migrate the internal DNS record", func() {
			gomock.InOrder(
				internalDNSRecord.EXPECT().Migrate(ctx),
				internalDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateExternalDNSResources", func() {
		It("should migrate the external DNS record", func() {
			gomock.InOrder(
				externalDNSRecord.EXPECT().Migrate(ctx),
				externalDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateIngressDNSRecord", func() {
		It("should migrate the ingress DNS record", func() {
			gomock.InOrder(
				ingressDNSRecord.EXPECT().Migrate(ctx),
				ingressDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateIngressDNSRecord(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateOwnerDNSResources", func() {
		It("should migrate the owner DNSRecord resource", func() {
			gomock.InOrder(
				ownerDNSRecord.EXPECT().Migrate(ctx),
				ownerDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateOwnerDNSResources(ctx)).To(Succeed())
		})

		It("should delete the owner DNSRecord resource if owner checks are disabled", func() {
			b.Seed.GetInfo().Spec.Settings = &gardencorev1beta1.SeedSettings{
				OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
					Enabled: false,
				},
			}
			gomock.InOrder(
				ownerDNSRecord.EXPECT().Destroy(ctx),
				ownerDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.MigrateOwnerDNSResources(ctx)).To(Succeed())
		})
	})
})
