// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	mockdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("addons", func() {

	var (
		ctrl *gomock.Controller

		scheme *runtime.Scheme
		client client.Client

		ingressDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx     = context.TODO()
		now     = time.Now()
		testErr = fmt.Errorf("test")

		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(dnsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()

		ingressDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		b = &Botanist{
			Operation: &operation.Operation{
				Config: &config.GardenletConfiguration{
					Controllers: &config.GardenletControllerConfiguration{
						Shoot: &config.ShootControllerConfiguration{
							DNSEntryTTLSeconds: pointer.Int64(ttl),
						},
					},
				},
				Shoot: &shoot.Shoot{
					SeedNamespace:         seedNamespace,
					ExternalClusterDomain: pointer.String(externalDomain),
					ExternalDomain: &garden.Domain{
						Domain:   externalDomain,
						Provider: externalProvider,
						Zone:     externalZone,
						SecretData: map[string][]byte{
							"external-foo": []byte("external-bar"),
						},
					},
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							DNS:              &shoot.DNS{},
							IngressDNSRecord: ingressDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{},
				Logger: logrus.NewEntry(logger.NewNopLogger()),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
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
			Status: gardencorev1beta1.ShootStatus{
				ClusterIdentity: pointer.String("shoot-cluster-identity"),
			},
		})

		renderer := cr.NewWithServerVersion(&version.Info{})
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, dnsv1alpha1.SchemeGroupVersion})
		mapper.Add(dnsv1alpha1.SchemeGroupVersion.WithKind("DNSOwner"), meta.RESTScopeRoot)
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(client, mapper))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")

		b.K8sSeedClient = fakeclientset.NewClientSetBuilder().
			WithClient(client).
			WithChartApplier(chartApplier).
			Build()

		cleanup = test.WithVar(&dnsrecord.TimeNow, func() time.Time { return now })
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Context("DefaultNginxIngressDNSEntry", func() {
		It("should delete the entry when calling Deploy", func() {
			Expect(client.Create(ctx, &dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{Name: common.ShootDNSIngressName, Namespace: seedNamespace},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultNginxIngressDNSEntry().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSEntry{}
			err := client.Get(ctx, types.NamespacedName{Name: common.ShootDNSIngressName, Namespace: seedNamespace}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultNginxIngressDNSOwner", func() {
		It("should delete the owner when calling Deploy", func() {
			Expect(client.Create(ctx, &dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{Name: seedNamespace + "-" + common.ShootDNSIngressName},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultNginxIngressDNSOwner().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSOwner{}
			err := client.Get(ctx, types.NamespacedName{Name: seedNamespace + "-" + common.ShootDNSIngressName}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("SetNginxIngressAddress", func() {
		It("does nothing when DNS is disabled", func() {
			b.Shoot.DisableDNS = true

			b.SetNginxIngressAddress(address, client)

			Expect(b.Shoot.Components.Extensions.DNS.NginxOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.NginxEntry).To(BeNil())
		})

		It("does nothing when nginx is disabled", func() {
			b.Shoot.GetInfo().Spec.Addons.NginxIngress.Enabled = false

			b.SetNginxIngressAddress(address, client)

			Expect(b.Shoot.Components.Extensions.DNS.NginxOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.NginxEntry).To(BeNil())
		})

		It("sets an owner and entry which create DNSOwner and DNSEntry", func() {
			ingressDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			ingressDNSRecord.EXPECT().SetValues([]string{address})

			b.SetNginxIngressAddress(address, client)

			Expect(b.Shoot.Components.Extensions.DNS.NginxOwner).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.NginxOwner.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.NginxEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.NginxEntry.Deploy(ctx)).ToNot(HaveOccurred())

			owner := &dnsv1alpha1.DNSOwner{}
			err := client.Get(ctx, types.NamespacedName{Name: seedNamespace + "-" + common.ShootDNSIngressName}, owner)
			Expect(err).ToNot(HaveOccurred())
			entry := &dnsv1alpha1.DNSEntry{}
			err = client.Get(ctx, types.NamespacedName{Name: common.ShootDNSIngressName, Namespace: seedNamespace}, entry)
			Expect(err).ToNot(HaveOccurred())

			Expect(owner).To(DeepDerivativeEqual(&dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:            seedNamespace + "-" + common.ShootDNSIngressName,
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSOwnerSpec{
					OwnerId: "shoot-cluster-identity-" + common.ShootDNSIngressName,
					Active:  pointer.Bool(true),
				},
			}))
			Expect(entry).To(DeepDerivativeEqual(&dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:            common.ShootDNSIngressName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSEntrySpec{
					DNSName: "*.ingress." + externalDomain,
					TTL:     pointer.Int64(ttl),
					Targets: []string{address},
				},
			}))

			Expect(b.Shoot.Components.Extensions.DNS.NginxOwner.Destroy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.NginxEntry.Destroy(ctx)).ToNot(HaveOccurred())

			owner = &dnsv1alpha1.DNSOwner{}
			err = client.Get(ctx, types.NamespacedName{Name: seedNamespace + "-" + common.ShootDNSIngressName}, owner)
			Expect(err).To(BeNotFoundError())
			entry = &dnsv1alpha1.DNSEntry{}
			err = client.Get(ctx, types.NamespacedName{Name: common.ShootDNSIngressName, Namespace: seedNamespace}, entry)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultIngressDNSRecord", func() {
		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			c := b.DefaultIngressDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			Expect(c.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := client.Get(ctx, types.NamespacedName{Name: shootName + "-" + common.ShootDNSIngressName, Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + common.ShootDNSIngressName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: externalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      shootName + "-" + DNSExternalName,
						Namespace: seedNamespace,
					},
					Zone:       pointer.String(externalZone),
					Name:       "*.ingress." + externalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        pointer.Int64(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSExternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + DNSExternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"external-foo": []byte("external-bar"),
				},
			}))
		})
	})

	Describe("#DeployOrDestroyIngressDNSRecord", func() {
		Context("deploy (DNS enabled)", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Deploy(ctx)
				ingressDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("restore (DNS enabled and restore operation)", func() {
			var shootState = &gardencorev1alpha1.ShootState{}

			BeforeEach(func() {
				b.SetShootState(shootState)
				b.Shoot.GetInfo().Status = gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				}
			})

			It("should call Restore and Wait and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Restore(ctx, shootState)
				ingressDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (DNS disabled)", func() {
			BeforeEach(func() {
				b.Shoot.DisableDNS = true
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				ingressDNSRecord.EXPECT().Destroy(ctx)
				ingressDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				ingressDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DestroyIngressDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			ingressDNSRecord.EXPECT().Destroy(ctx)
			ingressDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyIngressDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			ingressDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyIngressDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateIngressDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			ingressDNSRecord.EXPECT().Migrate(ctx)
			ingressDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateIngressDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			ingressDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateIngressDNSRecord(ctx)).To(MatchError(testErr))
		})
	})
})
