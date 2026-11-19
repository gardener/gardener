// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentrestriction_test

import (
	"crypto/rand"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NodeAgentRestriction tests", func() {
	var node *corev1.Node

	BeforeEach(func() {
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "node-",
				Labels: map[string]string{
					"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool-1",
					"kubernetes.io/hostname":                                "foo-machine",
				},
			},
		}
		Expect(testClient.Create(ctx, node)).To(Succeed())

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, node)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	Describe("gardener-node-agent updating its Node", func() {
		It("should forbid changing the gardener-node-agent-secret-name label", func() {
			patch := client.MergeFrom(node.DeepCopy())
			node.Labels["worker.gardener.cloud/gardener-node-agent-secret-name"] = "gardener-node-agent-pool-2"

			Expect(testClientNodeAgent.Patch(ctx, node, patch)).To(BeForbiddenError())
		})

		It("should forbid changing the hostname label", func() {
			patch := client.MergeFrom(node.DeepCopy())
			node.Labels["kubernetes.io/hostname"] = "bar-machine"

			Expect(testClientNodeAgent.Patch(ctx, node, patch)).To(BeForbiddenError())
		})

		It("should forbid removing a protected label", func() {
			patch := client.MergeFrom(node.DeepCopy())
			delete(node.Labels, "kubernetes.io/hostname")

			Expect(testClientNodeAgent.Patch(ctx, node, patch)).To(BeForbiddenError())
		})

		It("should allow changing an unrelated label", func() {
			patch := client.MergeFrom(node.DeepCopy())
			metav1.SetMetaDataLabel(&node.ObjectMeta, "node.kubernetes.io/role", "worker")

			Expect(testClientNodeAgent.Patch(ctx, node, patch)).To(Succeed())
		})

		It("should allow changing an annotation", func() {
			patch := client.MergeFrom(node.DeepCopy())
			metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/kubernetes-version", "1.34")

			Expect(testClientNodeAgent.Patch(ctx, node, patch)).To(Succeed())
		})
	})

	Describe("another user updating a Node", func() {
		It("should allow changing a protected label", func() {
			patch := client.MergeFrom(node.DeepCopy())
			node.Labels["worker.gardener.cloud/gardener-node-agent-secret-name"] = "gardener-node-agent-pool-2"

			Expect(testClient.Patch(ctx, node, patch)).To(Succeed())
		})
	})

	Describe("gardener-node-agent creating a CertificateSigningRequest", func() {
		var createCertificateSigningRequest func(commonName string, organization []string, signerName string) *certificatesv1.CertificateSigningRequest

		BeforeEach(func() {
			createCertificateSigningRequest = func(commonName string, organization []string, signerName string) *certificatesv1.CertificateSigningRequest {
				privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
				Expect(err).NotTo(HaveOccurred())

				request, err := certutil.MakeCSR(privateKey, &pkix.Name{CommonName: commonName, Organization: organization}, nil, nil)
				Expect(err).NotTo(HaveOccurred())

				return &certificatesv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "csr-"},
					Spec: certificatesv1.CertificateSigningRequestSpec{
						Request:    request,
						SignerName: signerName,
						Usages:     []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth},
					},
				}
			}
		})

		It("should allow a client certificate for the requesting gardener-node-agent", func() {
			Expect(testClientNodeAgent.Create(ctx, createCertificateSigningRequest("gardener.cloud:node-agent:machine:"+machineName, []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName))).To(Succeed())
		})

		It("should forbid a client certificate for a different gardener-node-agent", func() {
			Expect(testClientNodeAgent.Create(ctx, createCertificateSigningRequest("gardener.cloud:node-agent:machine:other-machine", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName))).To(BeForbiddenError())
		})

		It("should forbid a client certificate with a different organization", func() {
			Expect(testClientNodeAgent.Create(ctx, createCertificateSigningRequest("gardener.cloud:node-agent:machine:"+machineName, []string{"system:nodes"}, certificatesv1.KubeAPIServerClientSignerName))).To(BeForbiddenError())
		})

		It("should allow a certificate for a different signer", func() {
			Expect(testClientNodeAgent.Create(ctx, createCertificateSigningRequest("system:node:"+machineName, []string{"system:nodes"}, certificatesv1.KubeletServingSignerName))).To(Succeed())
		})
	})

	Describe("another user creating a CertificateSigningRequest", func() {
		It("should allow a client certificate for an arbitrary gardener-node-agent", func() {
			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
			Expect(err).NotTo(HaveOccurred())

			request, err := certutil.MakeCSR(privateKey, &pkix.Name{CommonName: "gardener.cloud:node-agent:machine:other-machine", Organization: []string{"gardener.cloud:node-agents"}}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(testClient.Create(ctx, &certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "csr-"},
				Spec: certificatesv1.CertificateSigningRequestSpec{
					Request:    request,
					SignerName: certificatesv1.KubeAPIServerClientSignerName,
					Usages:     []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth},
				},
			})).To(Succeed())
		})
	})
})
