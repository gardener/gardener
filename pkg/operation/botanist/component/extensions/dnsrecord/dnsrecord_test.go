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

package dnsrecord_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	name                = "foo"
	namespace           = "shoot--foo--bar"
	secretName          = "testsecret"
	extensionType       = "provider"
	zone                = "zone"
	dnsName             = "foo.bar.external.example.com"
	address             = "1.2.3.4"
	ttl           int64 = 300
)

var _ = Describe("DNSRecord", func() {
	var (
		ctrl *gomock.Controller

		c client.Client

		values    *dnsrecord.Values
		dnsRecord dnsrecord.Interface

		dns    *extensionsv1alpha1.DNSRecord
		secret *corev1.Secret

		ctx     = context.TODO()
		now     = time.Now()
		log     = logger.NewNopLogger()
		testErr = fmt.Errorf("test")

		fakeOps *retryfake.Ops
		mockNow *mocktime.MockNow
		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
		c = fake.NewClientBuilder().WithScheme(scheme).Build()

		values = &dnsrecord.Values{
			Name:       name,
			Namespace:  namespace,
			SecretName: secretName,
			Type:       extensionType,
			SecretData: map[string][]byte{
				"foo": []byte("bar"),
			},
			Zone:       pointer.String(zone),
			DNSName:    dnsName,
			RecordType: extensionsv1alpha1.DNSRecordTypeA,
			Values:     []string{address},
			TTL:        pointer.Int64(ttl),
		}
		dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				},
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: extensionType,
				},
				SecretRef: corev1.SecretReference{
					Name:      secretName,
					Namespace: namespace,
				},
				Zone:       pointer.String(zone),
				Name:       dnsName,
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{address},
				TTL:        pointer.Int64(ttl),
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 1}
		mockNow = mocktime.NewMockNow(ctrl)
		mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
		cleanup = test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
			&dnsrecord.TimeNow, mockNow.Do,
			&extensions.TimeNow, mockNow.Do,
			&gutil.TimeNow, mockNow.Do,
		)
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should deploy the DNSRecord resource and its secret", func() {
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			dns := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dns)
			Expect(err).NotTo(HaveOccurred())
			Expect(dns).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.DNSRecordResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))

			secret := &corev1.Secret{}
			err = c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).To(DeepEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: secret.Data,
			}))
		})

		It("should fail if creating the DNSRecord resource failed", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(apierrors.NewNotFound(corev1.Resource("secrets"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(secret)).DoAndReturn(
				func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(secret))
					return nil
				})
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(dns), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("dnsrecords"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(dns)).DoAndReturn(
				func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(dns))
					return testErr
				})

			dnsRecord := dnsrecord.New(log, mc, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(MatchError(testErr))
		})

		It("should deploy the DNSRecord resource if ReconcileOnce is true and the DNSRecord is not found", func() {
			values.ReconcileOnce = true
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)

			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			dns := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dns)
			Expect(err).NotTo(HaveOccurred())
			Expect(dns).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.DNSRecordResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))
		})

		It("should update the timestamp annotation if ReconcileOnce is true and the DNSRecord is found", func() {
			values.ReconcileOnce = true
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			delete(dns.Annotations, v1beta1constants.GardenerOperation)
			// set old timestamp (e.g. added on creation / earlier Deploy call)
			metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).String())
			Expect(c.Create(ctx, dns)).To(Succeed())

			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			dns := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dns)
			Expect(err).NotTo(HaveOccurred())
			Expect(dns).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				TypeMeta: metav1.TypeMeta{
					APIVersion: extensionsv1alpha1.SchemeGroupVersion.String(),
					Kind:       extensionsv1alpha1.DNSRecordResource,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "2",
					Annotations: map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				},
				Spec: dns.Spec,
			}))
		})
	})

	Describe("#Wait", func() {
		It("should fail if the resource does not exist", func() {
			Expect(dnsRecord.Wait(ctx)).To(HaveOccurred())
		})

		It("should fail if the resource is not ready", func() {
			dns.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}
			dns.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			Expect(c.Create(ctx, dns)).To(Succeed(), "creating dnsrecord succeeds")

			Expect(dnsRecord.Wait(ctx)).To(HaveOccurred(), "dnsrecord is not ready")
		})

		It("should fail if we haven't observed the latest timestamp annotation", func() {
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			patch := client.MergeFrom(dns.DeepCopy())
			dns.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().String(),
			}
			dns.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, dns, patch)).To(Succeed(), "patching dnsrecord succeeds")

			Expect(dnsRecord.Wait(ctx)).NotTo(Succeed(), "dnsrecord is ready but the timestamp is old")
		})

		It("should succeed if the resource is ready", func() {
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			patch := client.MergeFrom(dns.DeepCopy())
			dns.ObjectMeta.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}
			dns.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
			Expect(c.Patch(ctx, dns, patch)).To(Succeed(), "patching dnsrecord succeeds")

			Expect(dnsRecord.Wait(ctx)).To(Succeed(), "dnsrecord is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should succeed if the resource does not exist", func() {
			Expect(dnsRecord.Destroy(ctx)).To(Succeed())
		})

		It("should delete the DNSRecord resource", func() {
			Expect(c.Create(ctx, dns)).To(Succeed(), "creating dnsrecord succeeds")

			Expect(dnsRecord.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &extensionsv1alpha1.DNSRecord{})).To(BeNotFoundError())
		})

		It("should fail if deleting the DNSRecord resource failed", func() {
			dns := &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"confirmation.gardener.cloud/deletion": "true",
						v1beta1constants.GardenerTimestamp:     now.UTC().String(),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{}), gomock.Any())
			mc.EXPECT().Delete(ctx, dns).Return(testErr)

			dnsRecord := dnsrecord.New(log, mc, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Destroy(ctx)).To(MatchError(testErr))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should succeed if the resource does not exist", func() {
			Expect(dnsRecord.WaitCleanup(ctx)).To(Succeed())
		})

		It("should fail if the resource still exists", func() {
			timeNow := metav1.Now()
			dns.DeletionTimestamp = &timeNow
			Expect(c.Create(ctx, dns)).To(Succeed())

			Expect(dnsRecord.WaitCleanup(ctx)).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		var (
			state      = &runtime.RawExtension{Raw: []byte(`{"dummy":"state"}`)}
			shootState *gardencorev1alpha1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1alpha1.ShootState{
				Spec: gardencorev1alpha1.ShootStateSpec{
					Extensions: []gardencorev1alpha1.ExtensionResourceState{
						{
							Kind:  extensionsv1alpha1.DNSRecordResource,
							Name:  pointer.String(name),
							State: state,
						},
					},
				},
			}
		})

		It("should properly restore the DNSRecord resource state", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Status().Return(mc)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(apierrors.NewNotFound(corev1.Resource("secrets"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(secret)).DoAndReturn(
				func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(secret))
					return nil
				})

			metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationWaitForState)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(dns), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("dnsrecords"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(dns)).DoAndReturn(
				func(ctx context.Context, actual client.Object, opts ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(dns))
					return nil
				})

			// Restore state
			dnsWithState := dns.DeepCopy()
			dnsWithState.Status.State = state
			test.EXPECTPatch(ctx, mc, dnsWithState, dns, types.MergePatchType)

			// Annotate with restore annotation
			dnsWithRestore := dnsWithState.DeepCopy()
			metav1.SetMetaDataAnnotation(&dnsWithRestore.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore)
			test.EXPECTPatch(ctx, mc, dnsWithRestore, dnsWithState, types.MergePatchType)

			dnsRecord := dnsrecord.New(log, mc, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Restore(ctx, shootState)).To(Succeed())
		})
	})

	Describe("#Migrate", func() {
		It("should succeed if the resource does not exist", func() {
			Expect(dnsRecord.Migrate(ctx)).To(Succeed())
		})

		It("should migrate the DNSRecord resource", func() {
			Expect(c.Create(ctx, dns)).To(Succeed(), "creating dnsrecord succeeds")

			Expect(dnsRecord.Migrate(ctx)).To(Succeed())

			dns := &extensionsv1alpha1.DNSRecord{}
			Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, dns)).To(Succeed())
			Expect(dns.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate))
		})
	})

	Describe("#WaitMigrate", func() {
		It("should succeed if the resource does not exist", func() {
			Expect(dnsRecord.WaitMigrate(ctx)).To(Succeed())
		})

		It("should fail if resource is not yet migrated", func() {
			dns.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}
			dns.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, dns)).To(Succeed(), "creating dnsrecord succeeds")

			Expect(dnsRecord.WaitMigrate(ctx)).To(HaveOccurred(), "dnsrecord is not migrated")
		})

		It("should succeed if the resource is migrated", func() {
			dns.Status.LastError = nil
			dns.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
				Type:  gardencorev1beta1.LastOperationTypeMigrate,
			}

			Expect(c.Create(ctx, dns)).To(Succeed(), "creating dnsrecord succeeds")

			Expect(dnsRecord.WaitMigrate(ctx)).To(Succeed(), "dnsrecord is migrated")
		})
	})
})
