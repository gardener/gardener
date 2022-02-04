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

package controller

import (
	"context"
	"fmt"

	testutils "github.com/gardener/gardener/landscaper/common/test-utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

var _ = Describe("#FetchAndValidateConfigurationFromSecretReferences", func() {
	var (
		testOperation operation

		ca          = testutils.GenerateCACertificate("test")
		caCrt       = string(ca.CertificatePEM)
		privatKeyCA = string(ca.PrivateKeyPEM)

		tlsServingCert       = testutils.GenerateTLSServingCertificate(&ca)
		tlsServingCertString = string(tlsServingCert.CertificatePEM)
		tlsServingKeyString  = string(tlsServingCert.PrivateKeyPEM)

		caEtcdTLS      = testutils.GenerateCACertificate("gardener.cloud:system:etcd-virtual")
		caEtcdString   = string(caEtcdTLS.CertificatePEM)
		etcdClientCert = testutils.GenerateClientCertificate(&caEtcdTLS)
		etcdCertString = string(etcdClientCert.CertificatePEM)
		etcdKeyString  = string(etcdClientCert.PrivateKeyPEM)

		caSecretRef = corev1.SecretReference{
			Name:      "externally-managed-ca-secret",
			Namespace: "garden",
		}

		tlsSecretRef = corev1.SecretReference{
			Name:      "externally-managed-tls-secret",
			Namespace: "garden",
		}

		etcdSecretRef = corev1.SecretReference{
			Name:      "externally-managed-etcd-secret",
			Namespace: "garden",
		}
	)

	// mocking
	var (
		ctx               = context.TODO()
		ctrl              *gomock.Controller
		mockRuntimeClient *mockclient.MockClient
		runtimeClient     kubernetes.Interface
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockRuntimeClient = mockclient.NewMockClient(ctrl)
		runtimeClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()

		testOperation = operation{
			log:           logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient: runtimeClient,
			imports: &imports.Imports{
				Etcd: imports.Etcd{
					EtcdSecretRef: &etcdSecretRef,
				},
				GardenerAPIServer: imports.GardenerAPIServer{
					ComponentConfiguration: imports.APIServerComponentConfiguration{
						CA: &imports.CA{
							SecretRef: &caSecretRef,
						},
						TLS: &imports.TLSServer{
							SecretRef: &tlsSecretRef,
						},
					},
				},
				GardenerControllerManager: &imports.GardenerControllerManager{
					ComponentConfiguration: &imports.ControllerManagerComponentConfiguration{
						TLS: &imports.TLSServer{
							SecretRef: &tlsSecretRef,
						},
					},
				},
				GardenerAdmissionController: &imports.GardenerAdmissionController{
					Enabled: true,
					ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
						CA: &imports.CA{
							SecretRef: &caSecretRef,
						},
						TLS: &imports.TLSServer{
							SecretRef: &tlsSecretRef,
						},
					},
				},
			},
		}
	})

	It("should successfully validate secret references", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(2)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(etcdSecretRef.Namespace, etcdSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt":  []byte(caEtcdString),
						"tls.crt": []byte(etcdCertString),
						"tls.key": []byte(etcdKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte(tlsServingCertString),
						"tls.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(3)

		Expect(testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)).ToNot(HaveOccurred())

		Expect(testOperation.imports.EtcdCABundle).ToNot(BeNil())
		Expect(testOperation.imports.EtcdClientCert).ToNot(BeNil())

		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key).ToNot(BeNil())
	})

	It("should fail - CA certificate is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte("invalid"),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(1)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to validate Gardener API Server CA certificate"))
	})

	It("should fail - CA private key is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte("invalid"),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(1)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to validate Gardener API Server CA certificate"))
	})

	It("should fail - etcd CA is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(1)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte(tlsServingCertString),
						"tls.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(etcdSecretRef.Namespace, etcdSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt":  []byte("invalid"),
						"tls.crt": []byte(etcdCertString),
						"tls.key": []byte(etcdKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("the configured etcd CA certificate configured in the secret (%s/%s) is erroneous", etcdSecretRef.Namespace, etcdSecretRef.Name)))
	})

	It("should fail - etcd TLS cert is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(1)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte(tlsServingCertString),
						"tls.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(etcdSecretRef.Namespace, etcdSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt":  []byte(caEtcdString),
						"tls.crt": []byte("invalid"),
						"tls.key": []byte(etcdKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("the configured etcd client certificate configured in the secret (%s/%s) is erroneous", etcdSecretRef.Namespace, etcdSecretRef.Name)))
	})

	It("should fail - etcd private key is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		).Times(1)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte(tlsServingCertString),
						"tls.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(etcdSecretRef.Namespace, etcdSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt":  []byte(caEtcdString),
						"tls.crt": []byte(etcdCertString),
						"tls.key": []byte("invalid"),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("the configured etcd client key configured in the secret (%s/%s) is erroneous", etcdSecretRef.Namespace, etcdSecretRef.Name)))
	})

	It("should fail - TLS cert is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte("invalid"),
						"tls.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to validate Gardener API Server TLS certificates"))
	})

	It("should fail - TLS private key is not valid", func() {
		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(caSecretRef.Namespace, caSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.crt": []byte(caCrt),
						"ca.key": []byte(privatKeyCA),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key(tlsSecretRef.Namespace, tlsSecretRef.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"tls.crt": []byte(tlsServingCertString),
						"tls.key": []byte("invalid"),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		err := testOperation.FetchAndValidateConfigurationFromSecretReferences(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to validate Gardener API Server TLS certificates"))
	})
})
