// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentrestriction_test

import (
	"context"
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	certutil "k8s.io/client-go/util/cert"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentrestriction"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Handler", func() {
	var (
		ctx     = context.Background()
		handler *Handler

		nodeAgentUser = authenticationv1.UserInfo{
			Username: "gardener.cloud:node-agent:machine:machine-1",
			Groups:   []string{"gardener.cloud:node-agents"},
		}
	)

	BeforeEach(func() {
		handler = &Handler{
			Logger:  logzap.New(logzap.WriteTo(GinkgoWriter)),
			Decoder: admission.NewDecoder(kubernetes.SeedScheme),
		}
	})

	Describe("Node", func() {
		var oldNode *corev1.Node

		BeforeEach(func() {
			oldNode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-1",
					Labels: map[string]string{
						"worker.gardener.cloud/gardener-node-agent-secret-name": "gardener-node-agent-pool-1",
						"kubernetes.io/hostname":                                "node-1",
						"node.kubernetes.io/role":                               "node",
					},
				},
			}
		})

		request := func(user authenticationv1.UserInfo, operation admissionv1.Operation, oldNode, newNode *corev1.Node) admission.Request {
			req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				UserInfo:  user,
				Operation: operation,
				Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
			}}

			if oldNode != nil {
				req.OldObject = runtime.RawExtension{Raw: encode(oldNode)}
			}
			if newNode != nil {
				req.Object = runtime.RawExtension{Raw: encode(newNode)}
			}

			return req
		}

		When("the request is not an update", func() {
			It("should allow the request", func() {
				newNode := oldNode.DeepCopy()
				newNode.Labels["kubernetes.io/hostname"] = "evil"

				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, nil, newNode)).Allowed).To(BeTrue())
			})
		})

		When("the user is not a gardener-node-agent", func() {
			It("should allow changing a protected label", func() {
				newNode := oldNode.DeepCopy()
				newNode.Labels["worker.gardener.cloud/gardener-node-agent-secret-name"] = "gardener-node-agent-pool-2"

				Expect(handler.Handle(ctx, request(authenticationv1.UserInfo{Username: "system:node:node-1", Groups: []string{"system:nodes"}}, admissionv1.Update, oldNode, newNode)).Allowed).To(BeTrue())
			})
		})

		When("the user is a gardener-node-agent", func() {
			It("should allow changing an unrelated label", func() {
				newNode := oldNode.DeepCopy()
				newNode.Labels["node.kubernetes.io/role"] = "worker"

				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode)).Allowed).To(BeTrue())
			})

			It("should allow changing an annotation", func() {
				newNode := oldNode.DeepCopy()
				metav1.SetMetaDataAnnotation(&newNode.ObjectMeta, "worker.gardener.cloud/kubernetes-version", "1.34")

				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode)).Allowed).To(BeTrue())
			})

			It("should deny changing the gardener-node-agent-secret-name label", func() {
				newNode := oldNode.DeepCopy()
				newNode.Labels["worker.gardener.cloud/gardener-node-agent-secret-name"] = "gardener-node-agent-pool-2"

				response := handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode))
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Message).To(ContainSubstring("worker.gardener.cloud/gardener-node-agent-secret-name"))
			})

			It("should deny changing the hostname label", func() {
				newNode := oldNode.DeepCopy()
				newNode.Labels["kubernetes.io/hostname"] = "node-2"

				response := handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode))
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Message).To(ContainSubstring("kubernetes.io/hostname"))
			})

			It("should deny removing a protected label", func() {
				newNode := oldNode.DeepCopy()
				delete(newNode.Labels, "kubernetes.io/hostname")

				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode)).Allowed).To(BeFalse())
			})

			It("should deny adding a protected label", func() {
				delete(oldNode.Labels, "kubernetes.io/hostname")
				newNode := oldNode.DeepCopy()
				newNode.Labels["kubernetes.io/hostname"] = "node-1"

				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, oldNode, newNode)).Allowed).To(BeFalse())
			})
		})
	})

	Describe("CertificateSigningRequest", func() {
		request := func(user authenticationv1.UserInfo, operation admissionv1.Operation, commonName string, organization []string, signerName string, usages []certificatesv1.KeyUsage) admission.Request {
			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
			Expect(err).NotTo(HaveOccurred())

			csrData, err := certutil.MakeCSR(privateKey, &pkix.Name{CommonName: commonName, Organization: organization}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				UserInfo:  user,
				Operation: operation,
				Resource:  metav1.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests"},
				Object: runtime.RawExtension{Raw: encode(&certificatesv1.CertificateSigningRequest{
					Spec: certificatesv1.CertificateSigningRequestSpec{
						Request:    csrData,
						SignerName: signerName,
						Usages:     usages,
					},
				})},
			}}
		}

		clientAuthUsages := []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}

		When("the user is not a gardener-node-agent", func() {
			It("should allow the request", func() {
				Expect(handler.Handle(ctx, request(authenticationv1.UserInfo{Username: "system:node:node-1", Groups: []string{"system:nodes"}}, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-2", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName, clientAuthUsages)).Allowed).To(BeTrue())
			})
		})

		When("the operation is not a create", func() {
			It("should allow the request", func() {
				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Update, "gardener.cloud:node-agent:machine:machine-2", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName, clientAuthUsages)).Allowed).To(BeTrue())
			})
		})

		When("the signer is not the kube-apiserver-client signer", func() {
			It("should allow the request", func() {
				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-2", []string{"gardener.cloud:node-agents"}, "kubernetes.io/kubelet-serving", clientAuthUsages)).Allowed).To(BeTrue())
			})
		})

		When("the user is a gardener-node-agent", func() {
			It("should allow a client certificate for the requesting gardener-node-agent", func() {
				Expect(handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-1", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName, clientAuthUsages)).Allowed).To(BeTrue())
			})

			It("should deny a client certificate for a different gardener-node-agent", func() {
				response := handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-2", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName, clientAuthUsages))
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Message).To(ContainSubstring("does not match the requesting user"))
			})

			It("should deny a client certificate with a different organization", func() {
				response := handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-1", []string{"system:nodes"}, certificatesv1.KubeAPIServerClientSignerName, clientAuthUsages))
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Message).To(ContainSubstring("organization"))
			})

			It("should deny a client certificate with unexpected key usages", func() {
				response := handler.Handle(ctx, request(nodeAgentUser, admissionv1.Create, "gardener.cloud:node-agent:machine:machine-1", []string{"gardener.cloud:node-agents"}, certificatesv1.KubeAPIServerClientSignerName, []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth}))
				Expect(response.Allowed).To(BeFalse())
				Expect(response.Result.Message).To(ContainSubstring("key usages"))
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	raw, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return raw
}
