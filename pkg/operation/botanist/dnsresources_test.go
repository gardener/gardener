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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord/mock"
	mockcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/mock"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("dnsrecord", func() {
	var (
		ctrl *gomock.Controller

		externalDNSOwner    *mockcomponent.MockDeployWaiter
		externalDNSProvider *mockcomponent.MockDeployWaiter
		externalDNSEntry    *mockcomponent.MockDeployWaiter
		externalDNSRecord   *mockdnsrecord.MockInterface
		internalDNSOwner    *mockcomponent.MockDeployWaiter
		internalDNSProvider *mockcomponent.MockDeployWaiter
		internalDNSEntry    *mockcomponent.MockDeployWaiter
		internalDNSRecord   *mockdnsrecord.MockInterface
		ingressDNSOwner     *mockcomponent.MockDeployWaiter
		ingressDNSEntry     *mockcomponent.MockDeployWaiter
		ingressDNSRecord    *mockdnsrecord.MockInterface

		b *Botanist

		ctx = context.TODO()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		externalDNSOwner = mockcomponent.NewMockDeployWaiter(ctrl)
		externalDNSProvider = mockcomponent.NewMockDeployWaiter(ctrl)
		externalDNSEntry = mockcomponent.NewMockDeployWaiter(ctrl)
		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSOwner = mockcomponent.NewMockDeployWaiter(ctrl)
		internalDNSProvider = mockcomponent.NewMockDeployWaiter(ctrl)
		internalDNSEntry = mockcomponent.NewMockDeployWaiter(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		ingressDNSOwner = mockcomponent.NewMockDeployWaiter(ctrl)
		ingressDNSEntry = mockcomponent.NewMockDeployWaiter(ctrl)
		ingressDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		b = &Botanist{
			Operation: &operation.Operation{
				Shoot: &shoot.Shoot{
					ExternalClusterDomain: pointer.String(externalDomain),
					ExternalDomain: &garden.Domain{
						Provider: externalProvider,
					},
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							DNS: &shoot.DNS{
								ExternalOwner:    externalDNSOwner,
								ExternalProvider: externalDNSProvider,
								ExternalEntry:    externalDNSEntry,
								InternalOwner:    internalDNSOwner,
								InternalProvider: internalDNSProvider,
								InternalEntry:    internalDNSEntry,
								NginxOwner:       ingressDNSOwner,
								NginxEntry:       ingressDNSEntry,
							},
							ExternalDNSRecord: externalDNSRecord,
							InternalDNSRecord: internalDNSRecord,
							IngressDNSRecord:  ingressDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{
					InternalDomain: &garden.Domain{
						Provider: internalProvider,
					},
				},
				Logger: logrus.NewEntry(logger.NewNopLogger()),
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
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployInternalDNSResources", func() {
		It("should delete the DNSOwner, DNSProvider, and DNSEntry resources, and then deploy the DNSRecord resource if the feature gate is enabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, true)()
			gomock.InOrder(
				internalDNSOwner.EXPECT().Destroy(ctx),
				internalDNSOwner.EXPECT().WaitCleanup(ctx),
				internalDNSProvider.EXPECT().Destroy(ctx),
				internalDNSProvider.EXPECT().WaitCleanup(ctx),
				internalDNSEntry.EXPECT().Destroy(ctx),
				internalDNSEntry.EXPECT().WaitCleanup(ctx),
				internalDNSRecord.EXPECT().Deploy(ctx),
				internalDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployInternalDNSResources(ctx)).To(Succeed())
		})

		It("should migrate and delete the DNSRecord resource, and then deploy the DNSOwner, DNSProvider, and DNSEntry resources if the feature gate is disabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, false)()
			gomock.InOrder(
				internalDNSRecord.EXPECT().Migrate(ctx),
				internalDNSRecord.EXPECT().WaitMigrate(ctx),
				internalDNSRecord.EXPECT().Destroy(ctx),
				internalDNSRecord.EXPECT().WaitCleanup(ctx),
				internalDNSOwner.EXPECT().Deploy(ctx),
				internalDNSOwner.EXPECT().Wait(ctx),
				internalDNSProvider.EXPECT().Deploy(ctx),
				internalDNSProvider.EXPECT().Wait(ctx),
				internalDNSEntry.EXPECT().Deploy(ctx),
				internalDNSEntry.EXPECT().Wait(ctx),
			)
			Expect(b.DeployInternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DeployExternalDNSResources", func() {
		It("should delete the DNSOwner and DNSEntry resources, and then deploy the DNSProvider and DNSRecord resources if the feature gate is enabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, true)()
			gomock.InOrder(
				externalDNSOwner.EXPECT().Destroy(ctx),
				externalDNSOwner.EXPECT().WaitCleanup(ctx),
				externalDNSEntry.EXPECT().Destroy(ctx),
				externalDNSEntry.EXPECT().WaitCleanup(ctx),
				externalDNSProvider.EXPECT().Deploy(ctx),
				externalDNSProvider.EXPECT().Wait(ctx),
				externalDNSRecord.EXPECT().Deploy(ctx),
				externalDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployExternalDNSResources(ctx)).To(Succeed())
		})

		It("should migrate and delete the DNSRecord resource, and then deploy the DNSOwner, DNSProvider, and DNSEntry resources if the feature gate is disabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, false)()
			gomock.InOrder(
				externalDNSRecord.EXPECT().Migrate(ctx),
				externalDNSRecord.EXPECT().WaitMigrate(ctx),
				externalDNSRecord.EXPECT().Destroy(ctx),
				externalDNSRecord.EXPECT().WaitCleanup(ctx),
				externalDNSOwner.EXPECT().Deploy(ctx),
				externalDNSOwner.EXPECT().Wait(ctx),
				externalDNSProvider.EXPECT().Deploy(ctx),
				externalDNSProvider.EXPECT().Wait(ctx),
				externalDNSEntry.EXPECT().Deploy(ctx),
				externalDNSEntry.EXPECT().Wait(ctx),
			)
			Expect(b.DeployExternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DeployIngressDNSResources", func() {
		It("should delete the DNSOwner and DNSEntry resources, and then deploy the DNSRecord resource if the feature gate is enabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, true)()
			gomock.InOrder(
				ingressDNSOwner.EXPECT().Destroy(ctx),
				ingressDNSOwner.EXPECT().WaitCleanup(ctx),
				ingressDNSEntry.EXPECT().Destroy(ctx),
				ingressDNSEntry.EXPECT().WaitCleanup(ctx),
				ingressDNSRecord.EXPECT().Deploy(ctx),
				ingressDNSRecord.EXPECT().Wait(ctx),
			)
			Expect(b.DeployIngressDNSResources(ctx)).To(Succeed())
		})

		It("should migrate and delete the DNSRecord resource, and then deploy the DNSOwner and DNSEntry resources if the feature gate is disabled", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.UseDNSRecords, false)()
			gomock.InOrder(
				ingressDNSRecord.EXPECT().Migrate(ctx),
				ingressDNSRecord.EXPECT().WaitMigrate(ctx),
				ingressDNSRecord.EXPECT().Destroy(ctx),
				ingressDNSRecord.EXPECT().WaitCleanup(ctx),
				ingressDNSOwner.EXPECT().Deploy(ctx),
				ingressDNSOwner.EXPECT().Wait(ctx),
				ingressDNSEntry.EXPECT().Deploy(ctx),
				ingressDNSEntry.EXPECT().Wait(ctx),
			)
			Expect(b.DeployIngressDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyInternalDNSResources", func() {
		It("should delete all internal DNS resources so that the DNS record is deleted", func() {
			gomock.InOrder(
				internalDNSEntry.EXPECT().Destroy(ctx),
				internalDNSEntry.EXPECT().WaitCleanup(ctx),
				internalDNSProvider.EXPECT().Destroy(ctx),
				internalDNSProvider.EXPECT().WaitCleanup(ctx),
				internalDNSOwner.EXPECT().Destroy(ctx),
				internalDNSOwner.EXPECT().WaitCleanup(ctx),
				internalDNSRecord.EXPECT().Destroy(ctx),
				internalDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyInternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyExternalDNSResources", func() {
		It("should delete all external DNS resources so that the DNS record is deleted", func() {
			gomock.InOrder(
				externalDNSEntry.EXPECT().Destroy(ctx),
				externalDNSEntry.EXPECT().WaitCleanup(ctx),
				externalDNSProvider.EXPECT().Destroy(ctx),
				externalDNSProvider.EXPECT().WaitCleanup(ctx),
				externalDNSOwner.EXPECT().Destroy(ctx),
				externalDNSOwner.EXPECT().WaitCleanup(ctx),
				externalDNSRecord.EXPECT().Destroy(ctx),
				externalDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyExternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#DestroyIngressDNSResources", func() {
		It("should delete all ingress DNS resources so that the DNS record is deleted", func() {
			gomock.InOrder(
				ingressDNSEntry.EXPECT().Destroy(ctx),
				ingressDNSEntry.EXPECT().WaitCleanup(ctx),
				ingressDNSOwner.EXPECT().Destroy(ctx),
				ingressDNSOwner.EXPECT().WaitCleanup(ctx),
				ingressDNSRecord.EXPECT().Destroy(ctx),
				ingressDNSRecord.EXPECT().WaitCleanup(ctx),
			)
			Expect(b.DestroyIngressDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateInternalDNSResources", func() {
		It("should migrate or delete all internal DNS resources so that the DNS record is not deleted", func() {
			gomock.InOrder(
				internalDNSOwner.EXPECT().Destroy(ctx),
				internalDNSOwner.EXPECT().WaitCleanup(ctx),
				internalDNSProvider.EXPECT().Destroy(ctx),
				internalDNSProvider.EXPECT().WaitCleanup(ctx),
				internalDNSEntry.EXPECT().Destroy(ctx),
				internalDNSEntry.EXPECT().WaitCleanup(ctx),
				internalDNSRecord.EXPECT().Migrate(ctx),
				internalDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateInternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateExternalDNSResources", func() {
		It("should migrate or delete all external DNS resources so that the DNS record is not deleted", func() {
			gomock.InOrder(
				externalDNSOwner.EXPECT().Destroy(ctx),
				externalDNSOwner.EXPECT().WaitCleanup(ctx),
				externalDNSProvider.EXPECT().Destroy(ctx),
				externalDNSProvider.EXPECT().WaitCleanup(ctx),
				externalDNSEntry.EXPECT().Destroy(ctx),
				externalDNSEntry.EXPECT().WaitCleanup(ctx),
				externalDNSRecord.EXPECT().Migrate(ctx),
				externalDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateExternalDNSResources(ctx)).To(Succeed())
		})
	})

	Describe("#MigrateIngressDNSResources", func() {
		It("should migrate or delete all ingress DNS resources so that the DNS record is not deleted", func() {
			gomock.InOrder(
				ingressDNSOwner.EXPECT().Destroy(ctx),
				ingressDNSOwner.EXPECT().WaitCleanup(ctx),
				ingressDNSEntry.EXPECT().Destroy(ctx),
				ingressDNSEntry.EXPECT().WaitCleanup(ctx),
				ingressDNSRecord.EXPECT().Migrate(ctx),
				ingressDNSRecord.EXPECT().WaitMigrate(ctx),
			)
			Expect(b.MigrateIngressDNSResources(ctx)).To(Succeed())
		})
	})
})
