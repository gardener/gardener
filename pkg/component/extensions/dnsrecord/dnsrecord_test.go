// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
)

const (
	secretName          = "dnsrecord-testsecret"
	name                = "foo"
	namespace           = "shoot--foo--bar"
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
		log     = logr.Discard()
		testErr = errors.New("test")

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
			Zone:              ptr.To(zone),
			DNSName:           dnsName,
			RecordType:        extensionsv1alpha1.DNSRecordTypeA,
			Values:            []string{address},
			TTL:               ptr.To(ttl),
			AnnotateOperation: true,
		}

		dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
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
				Zone:       ptr.To(zone),
				Name:       dnsName,
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{address},
				TTL:        ptr.To(ttl),
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
			&gardenerutils.TimeNow, mockNow.Do,
		)
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should deploy the DNSRecord resource and its secret", func() {
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))

			deployedSecret := &corev1.Secret{}
			err = c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, deployedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: secret.Data,
			}))
		})

		It("should only deploy the DNSRecord resource but not the secret", func() {
			values.SecretData = nil
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))

			Expect(c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should deploy the DNSRecord with operation annotation if it exists with desired spec and AnnotateOperation==true", func() {
			By("Create existing DNSRecord")
			existingDNS := dns.DeepCopy()
			delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
			Expect(c.Create(ctx, existingDNS)).To(Succeed())

			By("Deploy DNSRecord again")
			values.AnnotateOperation = true
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			By("Verify DNSRecord")
			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					},
					ResourceVersion: "2",
				},
				Spec: dns.Spec,
			}))
		})

		It("should deploy the DNSRecord with operation annotation if it doesn't exist yet", func() {
			By("Deploy DNSRecord")
			values.AnnotateOperation = false
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			By("Verify DNSRecord")
			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))

			deployedSecret := &corev1.Secret{}
			err = c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, deployedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedSecret).To(DeepEqual(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            secretName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: secret.Data,
			}))
		})

		It("should deploy the DNSRecord without operation annotation if it exists with desired spec", func() {
			By("Create existing DNSRecord")
			existingDNS := dns.DeepCopy()
			delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Format(time.RFC3339Nano))
			Expect(c.Create(ctx, existingDNS)).To(Succeed())

			By("Deploy DNSRecord again")
			values.AnnotateOperation = false
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			By("Verify DNSRecord")
			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: dns.Spec,
			}))
			Expect(deployedDNS.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
		})

		It("should deploy the DNSRecord with operation annotation if spec changed", func() {
			By("Create existing DNSRecord")
			existingDNS := dns.DeepCopy()
			delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
			Expect(c.Create(ctx, existingDNS)).To(Succeed())

			By("Deploy DNSRecord again with changed values")
			values.AnnotateOperation = false
			values.Values = []string{address, "8.8.8.8", "1.1.1.1"}
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			By("Verify DNSRecord")
			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())

			expectedSpec := dns.Spec
			expectedSpec.Values = []string{address, "8.8.8.8", "1.1.1.1"}

			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					},
					ResourceVersion: "2",
				},
				Spec: expectedSpec,
			}))
		})

		It("should deploy the DNSRecord with operation annotation if gardener timestamp is after status.lastOperation.lastUpdateTime", func() {
			expectedDNSRecord := &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: dns.Spec,
				Status: extensionsv1alpha1.DNSRecordStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							State:          gardencorev1beta1.LastOperationStateSucceeded,
							LastUpdateTime: metav1.NewTime(now.Truncate(time.Second)), // this is also truncated when read from the client later on in the test
						},
					},
				},
			}

			existingDNS := dns.DeepCopy()
			delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(time.Second*10).Format(time.RFC3339Nano))
			existingDNS.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.NewTime(now.UTC()),
			}

			Expect(c.Create(ctx, existingDNS)).To(Succeed())
			values.AnnotateOperation = false
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
				Name:      existingDNS.Name,
				Namespace: existingDNS.Namespace,
			}}
			err := c.Get(ctx, client.ObjectKeyFromObject(deployedDNS), deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
		})

		It("should deploy the DNSRecord with operation annotation if it is in error state", func() {
			expectedDNSRecord := &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					ResourceVersion: "2",
				},
				Spec: dns.Spec,
				Status: extensionsv1alpha1.DNSRecordStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							State: gardencorev1beta1.LastOperationStateError,
						},
					},
				},
			}

			existingDNS := dns.DeepCopy()
			delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
			metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
			existingDNS.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateError,
			}

			Expect(c.Create(ctx, existingDNS)).To(Succeed())
			values.AnnotateOperation = false
			dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
				Name:      existingDNS.Name,
				Namespace: existingDNS.Namespace,
			}}
			err := c.Get(ctx, client.ObjectKeyFromObject(deployedDNS), deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
		})

		It("should deploy the DNSRecord resource with ip stack annotation", func() {
			values.IPStack = "ipv5"

			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						"dns.gardener.cloud/ip-stack":      "ipv5",
					},
					ResourceVersion: "1",
				},
				Spec: dns.Spec,
			}))
		})

		It("should deploy the DNSRecord resource with extensionClassName and labels", func() {
			values.Class = ptr.To(extensionsv1alpha1.ExtensionClassGarden)
			values.Labels = map[string]string{"foo": "bar"}

			expectedSpec := dns.Spec
			expectedSpec.Class = ptr.To(extensionsv1alpha1.ExtensionClassGarden)

			Expect(dnsRecord.Deploy(ctx)).To(Succeed())

			deployedDNS := &extensionsv1alpha1.DNSRecord{}
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployedDNS).To(DeepEqual(&extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
						v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
					},
					Labels:          map[string]string{"foo": "bar"},
					ResourceVersion: "1",
				},
				Spec: expectedSpec,
			}))
		})

		It("should fail if creating the DNSRecord resource failed", func() {
			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(apierrors.NewNotFound(corev1.Resource("secrets"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(secret)).DoAndReturn(
				func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(secret))
					return nil
				})
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(dns), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("dnsrecords"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(dns)).DoAndReturn(
				func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(dns))
					return testErr
				})

			dnsRecord := dnsrecord.New(log, mc, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Deploy(ctx)).To(MatchError(testErr))
		})

		Context("When ReconcileOnlyOnChangeOrError is true", func() {
			var expectedDNSRecord *extensionsv1alpha1.DNSRecord

			BeforeEach(func() {
				values.ReconcileOnlyOnChangeOrError = true

				expectedDNSRecord = &extensionsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Annotations: map[string]string{
							v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
							v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
						},
					},
					Spec: dns.Spec,
				}
				expectedDNSRecord.ResourceVersion = "2"
			})

			It("should deploy the DNSRecord resource if the DNSRecord is not found", func() {
				expectedDNSRecord.ResourceVersion = "1"

				Expect(dnsRecord.Deploy(ctx)).To(Succeed())

				deployedDNS := &extensionsv1alpha1.DNSRecord{}
				err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
			})

			It("should deploy the DNSRecord resource if the DNSRecord is not Succeeded", func() {
				existingDNS := dns.DeepCopy()
				delete(existingDNS.Annotations, v1beta1constants.GardenerOperation)
				metav1.SetMetaDataAnnotation(&existingDNS.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
				existingDNS.Status.LastOperation = &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateError,
				}

				expectedDNSRecord.Status = extensionsv1alpha1.DNSRecordStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							State: gardencorev1beta1.LastOperationStateError,
						},
					},
				}

				Expect(c.Create(ctx, existingDNS)).To(Succeed())
				Expect(dnsRecord.Deploy(ctx)).To(Succeed())

				deployedDNS := &extensionsv1alpha1.DNSRecord{}
				err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
			})

			It("should not update the timestamp annotation if the DNSRecord exists with the same values", func() {
				delete(dns.Annotations, v1beta1constants.GardenerOperation)
				// set old timestamp (e.g. added on creation / earlier Deploy call)
				metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))

				expectedDNSRecord.Annotations = map[string]string{
					v1beta1constants.GardenerTimestamp: now.UTC().Add(-time.Second).Format(time.RFC3339Nano),
				}

				Expect(c.Create(ctx, dns)).To(Succeed())
				Expect(dnsRecord.Deploy(ctx)).To(Succeed())

				deployedDNS := &extensionsv1alpha1.DNSRecord{}
				err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
			})

			DescribeTable("should reconcile the DNSRecord if desired values differ from current state", func(modifyValues func(), modifyExpected func()) {
				delete(dns.Annotations, v1beta1constants.GardenerOperation)
				// set old timestamp (e.g. added on creation / earlier Deploy call)
				metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerTimestamp, now.UTC().Add(-time.Second).Format(time.RFC3339Nano))
				Expect(c.Create(ctx, dns)).To(Succeed())

				modifyValues()
				dnsRecord = dnsrecord.New(log, c, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
				modifyExpected()
				Expect(dnsRecord.Deploy(ctx)).To(Succeed())

				deployedDNS := &extensionsv1alpha1.DNSRecord{}
				err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployedDNS)
				Expect(err).NotTo(HaveOccurred())
				Expect(deployedDNS).To(DeepEqual(expectedDNSRecord))
			},
				Entry("secretName changes", func() { values.SecretName = "new-secret-name" }, func() { expectedDNSRecord.Spec.SecretRef.Name = "new-secret-name" }),
				Entry("zone changes", func() { values.Zone = ptr.To("new-zone") }, func() { expectedDNSRecord.Spec.Zone = ptr.To("new-zone") }),
				Entry("values changes", func() { values.Values = []string{"8.8.8.8"} }, func() { expectedDNSRecord.Spec.Values = []string{"8.8.8.8"} }),
				Entry("TTL changes", func() { values.TTL = ptr.To[int64](1337) }, func() { expectedDNSRecord.Spec.TTL = ptr.To[int64](1337) }),
				Entry("zone is nil", func() { values.Zone = nil }, func() { expectedDNSRecord.Spec.Zone = nil }),
			)
		})

	})

	Describe("#Wait", func() {
		It("should fail if the resource does not exist", func() {
			Expect(dnsRecord.Wait(ctx)).To(HaveOccurred())
		})

		It("should fail if the resource is not ready", func() {
			dns.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
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
			dns.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.Add(-time.Millisecond).UTC().Format(time.RFC3339Nano),
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
			dns.Annotations = map[string]string{
				v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano),
			}
			dns.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: metav1.Time{Time: now.UTC().Add(time.Second)},
			}
			Expect(c.Patch(ctx, dns, patch)).To(Succeed(), "patching dnsrecord succeeds")

			Expect(dnsRecord.Wait(ctx)).To(Succeed(), "dnsrecord is ready")
		})
	})

	Describe("#Destroy", func() {
		It("should update the DNSRecord secret", func() {
			dns := &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"confirmation.gardener.cloud/deletion": "true",
						v1beta1constants.GardenerTimestamp:     now.UTC().Format(time.RFC3339Nano),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					s.Data = map[string][]byte{
						"baz": []byte("bar"),
					}
					return nil
				})
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					Expect(s.Name).To(Equal(secret.Name))
					Expect(s.Namespace).To(Equal(secret.Namespace))
					Expect(s.Type).To(Equal(secret.Type))
					Expect(s.Data).To(Equal(secret.Data))
					return nil
				})
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{}), gomock.Any())
			mc.EXPECT().Delete(ctx, dns)

			dnsRecord := dnsrecord.New(log, mc, values, dnsrecord.DefaultInterval, dnsrecord.DefaultSevereThreshold, dnsrecord.DefaultTimeout)
			Expect(dnsRecord.Destroy(ctx)).To(Succeed())
		})

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
						v1beta1constants.GardenerTimestamp:     now.UTC().Format(time.RFC3339Nano),
					},
				},
			}

			mc := mockclient.NewMockClient(ctrl)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{}))
			mc.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any())
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
			shootState *gardencorev1beta1.ShootState
		)

		BeforeEach(func() {
			shootState = &gardencorev1beta1.ShootState{
				Spec: gardencorev1beta1.ShootStateSpec{
					Extensions: []gardencorev1beta1.ExtensionResourceState{
						{
							Kind:  extensionsv1alpha1.DNSRecordResource,
							Name:  ptr.To(name),
							State: state,
						},
					},
				},
			}
		})

		It("should properly restore the DNSRecord resource state", func() {
			mc := mockclient.NewMockClient(ctrl)
			mockStatusWriter := mockclient.NewMockStatusWriter(ctrl)

			mc.EXPECT().Status().Return(mockStatusWriter)

			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(apierrors.NewNotFound(corev1.Resource("secrets"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(secret)).DoAndReturn(
				func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(secret))
					return nil
				})

			metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationWaitForState)
			mc.EXPECT().Get(ctx, client.ObjectKeyFromObject(dns), gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{})).
				Return(apierrors.NewNotFound(extensionsv1alpha1.Resource("dnsrecords"), name))
			mc.EXPECT().Create(ctx, test.HasObjectKeyOf(dns)).DoAndReturn(
				func(_ context.Context, actual client.Object, _ ...client.CreateOption) error {
					Expect(actual).To(DeepEqual(dns))
					return nil
				})

			// Restore state
			dnsWithState := dns.DeepCopy()
			dnsWithState.Status.State = state
			test.EXPECTStatusPatch(ctx, mockStatusWriter, dnsWithState, dns, types.MergePatchType)

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
