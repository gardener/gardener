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

	testutils "github.com/gardener/gardener/landscaper/common/test-utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

var _ = Describe("#UpdateSecretReferences", func() {
	var (
		ca    = testutils.GenerateCACertificate("test")
		caCrt = string(ca.CertificatePEM)
		caKey = string(ca.PrivateKeyPEM)

		tlsServingCert       = testutils.GenerateTLSServingCertificate(&ca)
		tlsServingCertString = string(tlsServingCert.CertificatePEM)
		tlsServingKeyString  = string(tlsServingCert.PrivateKeyPEM)

		testOperation          operation
		defaultSecretReference corev1.SecretReference
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

		defaultSecretReference = corev1.SecretReference{
			Name:      "guinea-pig",
			Namespace: "garden",
		}

		testOperation = operation{
			log:           logrus.NewEntry(logger.NewNopLogger()),
			runtimeClient: runtimeClient,
			imports: &imports.Imports{
				GardenerAPIServer: imports.GardenerAPIServer{
					ComponentConfiguration: imports.APIServerComponentConfiguration{
						CA: &imports.CA{
							Crt:       &caCrt,
							Key:       &caKey,
							SecretRef: &defaultSecretReference,
						},
						TLS: &imports.TLSServer{
							Crt:       &tlsServingCertString,
							Key:       &tlsServingKeyString,
							SecretRef: &defaultSecretReference,
						},
					},
				},
				GardenerControllerManager: &imports.GardenerControllerManager{
					ComponentConfiguration: &imports.ControllerManagerComponentConfiguration{
						TLS: &imports.TLSServer{
							Crt:       &tlsServingCertString,
							Key:       &tlsServingKeyString,
							SecretRef: &defaultSecretReference,
						},
					},
				},
				GardenerAdmissionController: &imports.GardenerAdmissionController{
					Enabled: true,
					ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
						CA: &imports.CA{
							Crt:       &caCrt,
							Key:       &caKey,
							SecretRef: &defaultSecretReference,
						},
						TLS: &imports.TLSServer{
							Crt:       &tlsServingCertString,
							Key:       &tlsServingKeyString,
							SecretRef: &defaultSecretReference,
						},
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
		mockRuntimeClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
			Do(func(ctx context.Context, obj *corev1.Secret, _ client.Patch, _ ...client.PatchOption) {
				Expect(obj.Name).To(Equal(defaultSecretReference.Name))
				Expect(obj.Namespace).To(Equal(defaultSecretReference.Namespace))
				Expect(obj.Data).To(Equal(map[string][]byte{
					"ca.crt": []byte(caCrt),
					"ca.key": []byte(caKey),
				}))
			}).Times(2)

		mockRuntimeClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
			Do(func(ctx context.Context, obj *corev1.Secret, _ client.Patch, _ ...client.PatchOption) {
				Expect(obj.Name).To(Equal(defaultSecretReference.Name))
				Expect(obj.Namespace).To(Equal(defaultSecretReference.Namespace))
				Expect(obj.Data).To(Equal(map[string][]byte{
					"tls.crt": []byte(tlsServingCertString),
					"tls.key": []byte(tlsServingKeyString),
				}))
			}).Times(3)

		Expect(testOperation.UpdateSecretReferences(ctx)).ToNot(HaveOccurred())
	})
})
