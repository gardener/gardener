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
	"time"

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
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	shootName      = "foo"
	shootNamespace = "bar"
	seedNamespace  = "shoot--foo--bar"

	externalDomain   = "foo.bar.external.example.com"
	externalProvider = "external-provider"
	externalZone     = "external-zone"

	internalDomain   = "foo.bar.internal.example.com"
	internalProvider = "internal-provider"
	internalZone     = "internal-zone"

	address       = "1.2.3.4"
	ttl     int64 = 300
)

var _ = Describe("dnsrecord", func() {
	var (
		ctrl *gomock.Controller

		scheme *runtime.Scheme
		client client.Client

		externalDNSRecord *mockdnsrecord.MockInterface
		internalDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx     = context.TODO()
		now     = time.Now()
		testErr = fmt.Errorf("test")

		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		client = fake.NewClientBuilder().WithScheme(scheme).Build()

		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

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
					InternalClusterDomain: internalDomain,
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ExternalDNSRecord: externalDNSRecord,
							InternalDNSRecord: internalDNSRecord,
						},
					},
				},
				Garden: &garden.Garden{
					InternalDomain: &garden.Domain{
						Domain:   internalDomain,
						Provider: internalProvider,
						Zone:     internalZone,
						SecretData: map[string][]byte{
							"internal-foo": []byte("internal-bar"),
						},
					},
				},
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
			},
		})

		renderer := cr.NewWithServerVersion(&version.Info{})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(client, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
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

	Context("DefaultExternalDNSRecord", func() {
		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			c := b.DefaultExternalDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			Expect(c.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := client.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSExternalName, Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + DNSExternalName,
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
					Name:       "api." + externalDomain,
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

	Context("DefaultInternalDNSRecord", func() {
		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			c := b.DefaultInternalDNSRecord()
			c.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			c.SetValues([]string{address})

			Expect(c.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := client.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSInternalName, Namespace: seedNamespace}, dnsRecord)
			Expect(err).ToNot(HaveOccurred())
			Expect(dnsRecord).To(DeepDerivativeEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "extensions.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + DNSInternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: internalProvider,
					},
					SecretRef: corev1.SecretReference{
						Name:      shootName + "-" + DNSInternalName,
						Namespace: seedNamespace,
					},
					Zone:       pointer.String(internalZone),
					Name:       "api." + internalDomain,
					RecordType: extensionsv1alpha1.DNSRecordTypeA,
					Values:     []string{address},
					TTL:        pointer.Int64(ttl),
				},
			}))

			secret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSInternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            shootName + "-" + DNSInternalName,
					Namespace:       seedNamespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"internal-foo": []byte("internal-bar"),
				},
			}))
		})
	})

	Describe("#DeployOrDestroyExternalDNSRecord", func() {
		Context("deploy (DNS enabled)", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Deploy(ctx)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
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
				externalDNSRecord.EXPECT().Restore(ctx, shootState)
				externalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				externalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (DNS disabled)", func() {
			BeforeEach(func() {
				b.Shoot.DisableDNS = true
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				externalDNSRecord.EXPECT().Destroy(ctx)
				externalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DeployOrDestroyInternalDNSRecord", func() {
		Context("deploy (DNS enabled)", func() {
			It("should call Deploy and Wait and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Deploy(ctx)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Deploy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Deploy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
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
				internalDNSRecord.EXPECT().Restore(ctx, shootState)
				internalDNSRecord.EXPECT().Wait(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Restore and fail if it failed", func() {
				internalDNSRecord.EXPECT().Restore(ctx, shootState).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})

		Context("destroy (DNS disabled)", func() {
			BeforeEach(func() {
				b.Shoot.DisableDNS = true
			})

			It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
				internalDNSRecord.EXPECT().Destroy(ctx)
				internalDNSRecord.EXPECT().WaitCleanup(ctx)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(Succeed())
			})

			It("should call Destroy and fail if it failed", func() {
				internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
				Expect(b.DeployOrDestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
			})
		})
	})

	Describe("#DestroyExternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Destroy(ctx)
			externalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			externalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#DestroyInternalDNSRecord", func() {
		It("should call Destroy and WaitCleanup and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Destroy(ctx)
			internalDNSRecord.EXPECT().WaitCleanup(ctx)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Destroy and fail if it failed", func() {
			internalDNSRecord.EXPECT().Destroy(ctx).Return(testErr)
			Expect(b.DestroyInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateExternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			externalDNSRecord.EXPECT().Migrate(ctx)
			externalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			externalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateExternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#MigrateInternalDNSRecord", func() {
		It("should call Migrate and WaitMigrate and succeed if they succeeded", func() {
			internalDNSRecord.EXPECT().Migrate(ctx)
			internalDNSRecord.EXPECT().WaitMigrate(ctx)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(Succeed())
		})

		It("should call Migrate and fail if it failed", func() {
			internalDNSRecord.EXPECT().Migrate(ctx).Return(testErr)
			Expect(b.MigrateInternalDNSRecord(ctx)).To(MatchError(testErr))
		})
	})
})
