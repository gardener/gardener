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
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

var _ = Describe("#SyncWithExistingGardenerInstallation", func() {
	var (
		err   error
		ca    = testutils.GenerateCACertificate("test")
		caCrt = string(ca.CertificatePEM)
		caKey = string(ca.PrivateKeyPEM)

		tlsServingCert       = testutils.GenerateTLSServingCertificate(&ca)
		tlsServingCertString = string(tlsServingCert.CertificatePEM)
		tlsServingKeyString  = string(tlsServingCert.PrivateKeyPEM)

		caEtcdTLS      = testutils.GenerateCACertificate("gardener.cloud:system:etcd-virtual")
		caEtcdString   = string(caEtcdTLS.CertificatePEM)
		etcdClientCert = testutils.GenerateClientCertificate(&caEtcdTLS)
		etcdCertString = string(etcdClientCert.CertificatePEM)
		etcdKeyString  = string(etcdClientCert.PrivateKeyPEM)

		testOperation operation

		// encryption config
		encryptionConfigBytes []byte
		encryptionConfig      = &apiserverconfigv1.EncryptionConfiguration{
			TypeMeta: metav1.TypeMeta{},
			Resources: []apiserverconfigv1.ResourceConfiguration{
				{
					Resources: []string{"test"},
				},
			},
		}
		scheme = runtime.NewScheme()
	)

	apiserverconfigv1.AddToScheme(scheme)
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	codec := versioning.NewDefaultingCodecForScheme(
		scheme,
		serializer,
		serializer,
		apiserverconfigv1.SchemeGroupVersion,
		apiserverconfigv1.SchemeGroupVersion)

	// mocking
	var (
		ctx                 = context.TODO()
		ctrl                *gomock.Controller
		mockGardenClient    *mockclient.MockClient
		mockRuntimeClient   *mockclient.MockClient
		runtimeClient       kubernetes.Interface
		virtualGardenClient kubernetes.Interface
	)

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockGardenClient = mockclient.NewMockClient(ctrl)
		mockRuntimeClient = mockclient.NewMockClient(ctrl)

		runtimeClient = fake.NewClientSetBuilder().WithClient(mockRuntimeClient).Build()
		virtualGardenClient = fake.NewClientSetBuilder().WithClient(mockGardenClient).Build()

		encryptionConfigBytes, err = runtime.Encode(codec, encryptionConfig)
		Expect(err).ToNot(HaveOccurred())

		testOperation = operation{
			log:                 logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient:       runtimeClient,
			virtualGardenClient: &virtualGardenClient,
			imports: &imports.Imports{
				GardenerAPIServer: imports.GardenerAPIServer{
					ComponentConfiguration: imports.APIServerComponentConfiguration{
						CA:  &imports.CA{},
						TLS: &imports.TLSServer{},
					},
				},
				GardenerControllerManager: &imports.GardenerControllerManager{
					ComponentConfiguration: &imports.ControllerManagerComponentConfiguration{
						TLS: &imports.TLSServer{},
					},
				},
				GardenerAdmissionController: &imports.GardenerAdmissionController{
					Enabled: true,
					ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
						CA:  &imports.CA{},
						TLS: &imports.TLSServer{},
					},
				},
			},
			// initialized when creating operation
			exports: exports.Exports{
				GardenerAPIServerCA:                   exports.Certificate{},
				GardenerAPIServerTLSServing:           exports.Certificate{},
				GardenerAdmissionControllerCA:         &exports.Certificate{},
				GardenerAdmissionControllerTLSServing: &exports.Certificate{},
				GardenerControllerManagerTLSServing:   exports.Certificate{},
			},
		}
	})

	It("should get the Gardener API Server CA from the APIService", func() {
		mockGardenClient.EXPECT().Get(ctx, kutil.Key(fmt.Sprintf("%s.%s", gardencorev1beta1.SchemeGroupVersion.Version, gardencorev1beta1.SchemeGroupVersion.Group)), gomock.AssignableToTypeOf(&apiregistrationv1.APIService{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&apiregistrationv1.APIService{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: apiregistrationv1.APIServiceSpec{
						CABundle: []byte(caCrt),
					},
				}).DeepCopyInto(obj.(*apiregistrationv1.APIService))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "landscaper-controlplane-apiserver-ca-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.key": []byte(caCrt),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "gardener-apiserver-cert"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"etcd-client-ca.crt":     []byte(caEtcdString),
						"etcd-client.crt":        []byte(etcdCertString),
						"etcd-client.key":        []byte(etcdKeyString),
						"gardener-apiserver.crt": []byte(tlsServingCertString),
						"gardener-apiserver.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockGardenClient.EXPECT().Get(ctx, kutil.Key("gardener-admission-controller"), gomock.AssignableToTypeOf(&admissionregistrationv1.MutatingWebhookConfiguration{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Webhooks: []admissionregistrationv1.MutatingWebhook{
						{
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								CABundle: []byte(caCrt),
							},
						},
					},
				}).DeepCopyInto(obj.(*admissionregistrationv1.MutatingWebhookConfiguration))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "landscaper-controlplane-admission-controller-ca-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"ca.key": []byte(caKey),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "gardener-admission-controller-cert"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
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

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "gardener-controller-manager-cert"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"gardener-controller-manager.crt": []byte(tlsServingCertString),
						"gardener-controller-manager.key": []byte(tlsServingKeyString),
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		mockRuntimeClient.EXPECT().Get(ctx, kutil.Key("garden", "gardener-apiserver-encryption-config"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
			func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
				(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Data: map[string][]byte{
						"encryption-config.yaml": encryptionConfigBytes,
					},
				}).DeepCopyInto(obj.(*corev1.Secret))
				return nil
			},
		)

		Expect(testOperation.SyncWithExistingGardenerInstallation(ctx)).ToNot(HaveOccurred())
		Expect(testOperation.imports.EtcdCABundle).ToNot(BeNil())
		Expect(testOperation.imports.EtcdClientCert).ToNot(BeNil())

		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.Encryption).ToNot(BeNil())

		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key).ToNot(BeNil())
	})
})
