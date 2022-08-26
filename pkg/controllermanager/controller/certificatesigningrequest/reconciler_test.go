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

package certificatesigningrequest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("csrReconciler", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		restConfig *rest.Config

		csr                *certificatesv1.CertificateSigningRequest
		reconciler         reconcile.Reconciler
		privateKey         *rsa.PrivateKey
		certificateSubject *pkix.Name
	)

	BeforeEach(func() {
		restConfig = &rest.Config{}
		certificatesClient, _ := kubernetesclientset.NewForConfig(restConfig)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		privateKey, _ = secretutils.FakeGenerateKey(rand.Reader, 4096)
		csr = &certificatesv1.CertificateSigningRequest{
			TypeMeta: metav1.TypeMeta{Kind: "CertificateSigningRequest"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-csr",
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageClientAuth,
				},
				SignerName: certificatesv1.KubeAPIServerClientSignerName,
			},
		}

		reconciler = &Reconciler{Client: fakeClient, CertificatesClient: certificatesClient, CertificatesAPIVersion: "v1"}
	})

	It("should return nil because object not found", func() {
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{})).To(BeNotFoundError())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Final state", func() {
		BeforeEach(func() {
			Expect(fakeClient.Create(ctx, csr)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
		})

		It("should ignore the csr because certificate is already present in the status field", func() {
			patch := client.MergeFrom(csr.DeepCopy())
			csr.Status.Certificate = []byte("test-certificate")
			Expect(fakeClient.Patch(ctx, csr, patch)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore the csr because csr is already Approved", func() {
			patch := client.MergeFrom(csr.DeepCopy())
			csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
				Type: certificatesv1.CertificateApproved,
			})
			Expect(fakeClient.Patch(ctx, csr, patch)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("non seedclient csr", func() {
		BeforeEach(func() {
			Expect(fakeClient.Create(ctx, csr)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{})).To(Succeed())
		})

		It("should ignore the csr because csr does not match the requirements for a client certificate for a seed", func() {
			patch := client.MergeFrom(csr.DeepCopy())
			certificateSubject = &pkix.Name{
				Organization: []string{"foo"},
			}
			csrData, err := certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			csr.Spec.Request = csrData
			Expect(fakeClient.Patch(ctx, csr, patch)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: csr.Name}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
