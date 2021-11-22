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
	shootNamespace = "bar"

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
		shootName     string
		seedNamespace string
		ctrl          *gomock.Controller

		scheme *runtime.Scheme
		c      client.Client

		externalDNSRecord *mockdnsrecord.MockInterface
		internalDNSRecord *mockdnsrecord.MockInterface

		b *Botanist

		ctx     = context.TODO()
		now     = time.Now()
		testErr = fmt.Errorf("test")

		cleanup func()
	)

	BeforeEach(func() {
		shootName = "foo"
		seedNamespace = "shoot--foo--bar"
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		cleanup = test.WithVar(&dnsrecord.TimeNow, func() time.Time { return now })
	})

	JustBeforeEach(func() {
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
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(c, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")
		b.K8sSeedClient = fakeclientset.NewClientSetBuilder().
			WithClient(c).
			WithChartApplier(chartApplier).
			Build()
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Context("DefaultExternalDNSRecord", func() {
		It("should create a component that creates the DNSRecord and its secret on Deploy", func() {
			r := b.DefaultExternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSExternalName, Namespace: seedNamespace}, dnsRecord)
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
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + DNSExternalName,
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
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + DNSExternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + DNSExternalName,
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
			r := b.DefaultInternalDNSRecord()
			r.SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			r.SetValues([]string{address})

			Expect(r.Deploy(ctx)).ToNot(HaveOccurred())

			dnsRecord := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, types.NamespacedName{Name: shootName + "-" + DNSInternalName, Namespace: seedNamespace}, dnsRecord)
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
						Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + DNSInternalName,
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
			err = c.Get(ctx, types.NamespacedName{Name: DNSRecordSecretPrefix + "-" + shootName + "-" + DNSInternalName, Namespace: seedNamespace}, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).To(DeepDerivativeEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            DNSRecordSecretPrefix + "-" + shootName + "-" + DNSInternalName,
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

			JustBeforeEach(func() {
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
			JustBeforeEach(func() {
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

			JustBeforeEach(func() {
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
			JustBeforeEach(func() {
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

	Describe("#CleanupOrphanedDNSRecordSecrets", func() {
		var orphanedInternalSecret *corev1.Secret
		var regularInternalSecret *corev1.Secret
		var orphanedExternalSecret *corev1.Secret
		var regularExternalSecret *corev1.Secret

		JustBeforeEach(func() {
			// create an internal secret which is not prefixed with 'dnsrecord-' and is of the form '<shootName>-internal'
			orphanedInternalSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      shootName + "-" + DNSInternalName,
				Namespace: seedNamespace,
			}}
			err := c.Create(ctx, orphanedInternalSecret)
			Expect(err).ToNot(HaveOccurred())

			// create a regular internal secret which is prefixed with 'dnsrecord-'
			regularInternalSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + DNSInternalName,
				Namespace: seedNamespace,
			}}
			err = c.Create(ctx, regularInternalSecret)
			Expect(err).ToNot(HaveOccurred())

			// create an internal secret which is not prefixed with 'dnsrecord-' and is of the form '<shootName>-internal'
			orphanedExternalSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      shootName + "-" + DNSExternalName,
				Namespace: seedNamespace,
			}}
			err = c.Create(ctx, orphanedExternalSecret)
			Expect(err).ToNot(HaveOccurred())

			// create a regular internal secret which is prefixed with 'dnsrecord-'
			regularExternalSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      DNSRecordSecretPrefix + "-" + shootName + "-" + DNSExternalName,
				Namespace: seedNamespace,
			}}
			err = c.Create(ctx, regularExternalSecret)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should clean up orphaned Secrets, but keep prefixed secrets", func() {
			Expect(b.CleanupOrphanedDNSRecordSecrets(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: orphanedInternalSecret.Name, Namespace: seedNamespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: orphanedExternalSecret.Name, Namespace: seedNamespace}, &corev1.Secret{})).To(BeNotFoundError())

			internalSecret := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKey{Name: regularInternalSecret.Name, Namespace: seedNamespace}, internalSecret)).To(Succeed())
			Expect(internalSecret).To(DeepDerivativeEqual(regularInternalSecret))

			Expect(b.CleanupOrphanedDNSRecordSecrets(ctx)).To(Succeed())
			externalSecret := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKey{Name: regularExternalSecret.Name, Namespace: seedNamespace}, externalSecret)).To(Succeed())
			Expect(externalSecret).To(DeepDerivativeEqual(regularExternalSecret))
		})

		Context("When the Shoot name is 'gardener'", func() {
			BeforeEach(func() {
				shootName = "gardener"
				seedNamespace = "shoot--gardener--bar"
			})

			It("should clean up the external orphaned secret, but keep the internal orphaned secret", func() {

				Expect(b.CleanupOrphanedDNSRecordSecrets(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: orphanedExternalSecret.Name, Namespace: seedNamespace}, &corev1.Secret{})).To(BeNotFoundError())

				internalSecret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Name: orphanedInternalSecret.Name, Namespace: seedNamespace}, internalSecret)).To(Succeed())
				Expect(internalSecret).To(DeepDerivativeEqual(orphanedInternalSecret))

			})
		})

		It("should not fail the clean up of orphaned Secret when there are none", func() {
			Expect(b.CleanupOrphanedDNSRecordSecrets(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: orphanedExternalSecret.Name, Namespace: seedNamespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(b.CleanupOrphanedDNSRecordSecrets(ctx)).To(Succeed())

		})
	})
})
