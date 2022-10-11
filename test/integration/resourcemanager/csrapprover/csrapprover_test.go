// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csrapprover_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"net"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Kubelet Server CertificateSigningRequest Approver Controller tests", func() {
	var (
		privateKey         *rsa.PrivateKey
		certificateSubject *pkix.Name

		ip1      = "1.2.3.4"
		ip2      = "5.6.7.8"
		ips      []net.IP
		dnsName1 = "foo.bar"
		dnsName2 = "bar.baz"
		dnsNames []string

		csr *certificatesv1.CertificateSigningRequest
	)

	BeforeEach(func() {
		privateKey, _ = secretutils.FakeGenerateKey(rand.Reader, 4096)
		certificateSubject = &pkix.Name{
			CommonName:   userName,
			Organization: []string{user.NodesGroup},
		}

		ips = []net.IP{net.ParseIP(ip1), net.ParseIP(ip2)}
		dnsNames = []string{dnsName1, dnsName2}

		csr = &certificatesv1.CertificateSigningRequest{
			// Username, UID, Groups will be injected by API server.
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageClientAuth,
				},
				SignerName: certificatesv1.KubeletServingSignerName,
			},
		}
	})

	JustBeforeEach(func() {
		By("Generate CSR data")
		csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsNames, ips)
		Expect(err).NotTo(HaveOccurred())
		csr.Spec.Request = csrData

		By("Create CertificateSigningRequest")
		Expect(testClient.Create(ctx, csr)).To(Succeed())
		log.Info("Created CertificateSigningRequest for test", "certificateSigningRequest", client.ObjectKeyFromObject(csr))

		DeferCleanup(func() {
			By("Delete CertificateSigningRequest")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, csr))).To(Succeed())
		})
	})

	Context("non kubelet server certificate", func() {
		BeforeEach(func() {
			csr.Spec.SignerName = certificatesv1.KubeAPIServerClientSignerName
		})

		It("should ignore the CSR and do nothing", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
				g.Expect(csr.Status.Conditions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("kubelet server certificate", func() {
		Context("constraints fulfilled", func() {
			BeforeEach(func() {
				By("Create Node")
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nodeName,
						Labels: map[string]string{testID: testRunID},
					},
				}
				Expect(testClient.Create(ctx, node)).To(Succeed())
				log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

				DeferCleanup(func() {
					By("Delete Node")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
				})

				By("Create Machine")
				machine := &machinev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine",
						Namespace: testNamespace.Name,
						Labels: map[string]string{
							testID: testRunID,
							"node": node.Name,
						},
					},
				}
				Expect(testClient.Create(ctx, machine)).To(Succeed())
				log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

				DeferCleanup(func() {
					By("Delete Machine")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())
				})

				By("Patch node's addresses in status")
				patch := client.MergeFrom(node.DeepCopy())
				node.Status.Addresses = []corev1.NodeAddress{
					{Type: corev1.NodeHostName, Address: dnsName1},
					{Type: corev1.NodeHostName, Address: dnsName2},
					{Type: corev1.NodeInternalIP, Address: ip1},
					{Type: corev1.NodeInternalIP, Address: ip2},
				}
				Expect(testClient.Status().Patch(ctx, node, patch)).To(Succeed())
			})

			It("should approve the CSR", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
					g.Expect(csr.Status.Conditions).To(ContainElement(And(
						HaveField("Type", certificatesv1.CertificateApproved),
						HaveField("Reason", "RequestApproved"),
						HaveField("Message", "Approving kubelet server certificate CSR (all checks passed)"),
					)))
				}).Should(Succeed())
			})
		})

		Context("constraints violated", func() {
			runTest := func(expectedReason string) {
				It("should deny the CSR", func() {
					Eventually(func(g Gomega) {
						g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
						g.Expect(csr.Status.Conditions).To(ContainElement(And(
							HaveField("Type", certificatesv1.CertificateDenied),
							HaveField("Reason", "RequestDenied"),
							HaveField("Message", And(
								ContainSubstring("Denying kubelet server certificate CSR"),
								ContainSubstring(expectedReason),
							)),
						)))
					}).Should(Succeed())
				})
			}

			Context("username not prefixed with system:node:", func() {
				BeforeEach(func() {
					clientWithAdminUsername, err := client.New(restConfig, client.Options{Scheme: scheme})
					Expect(err).NotTo(HaveOccurred())

					DeferCleanup(test.WithVar(
						&testClient, clientWithAdminUsername,
					))
				})

				runTest("is not prefixed with")
			})

			Context("SANs don't contain any DNS names or IP addresses", func() {
				BeforeEach(func() {
					dnsNames, ips = nil, nil
				})

				runTest("no DNS names or IP addresses in the SANs found")
			})

			Context("common name does not match username", func() {
				BeforeEach(func() {
					certificateSubject.CommonName = "some-other-name"
				})

				runTest("common name in CSR does not match username")
			})

			Context("organization is empty", func() {
				BeforeEach(func() {
					certificateSubject.Organization = nil
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("organization contains too many items", func() {
				BeforeEach(func() {
					certificateSubject.Organization = []string{"foo", "bar"}
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("organization does not contain nodes group", func() {
				BeforeEach(func() {
					certificateSubject.Organization = []string{"foo"}
				})

				runTest("organization in CSR does not match nodes group")
			})

			Context("node object not found", func() {
				runTest("could not find node object with name")
			})

			Context("machine object not found", func() {
				BeforeEach(func() {
					By("Create Node")
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nodeName,
							Labels: map[string]string{testID: testRunID},
						},
					}
					Expect(testClient.Create(ctx, node)).To(Succeed())
					log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

					DeferCleanup(func() {
						By("Delete Node")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
					})
				})

				runTest("Expected exactly one machine in namespace")
			})

			Context("too many machine objects found", func() {
				BeforeEach(func() {
					By("Create Node")
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nodeName,
							Labels: map[string]string{testID: testRunID},
						},
					}
					Expect(testClient.Create(ctx, node)).To(Succeed())
					log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

					DeferCleanup(func() {
						By("Delete Node")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
					})

					machine := &machinev1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine",
							Namespace: testNamespace.Name,
							Labels: map[string]string{
								testID: testRunID,
								"node": node.Name,
							},
						},
					}
					machine2 := machine.DeepCopy()
					machine2.Name = "machine2"

					By("Create Machine1")
					Expect(testClient.Create(ctx, machine)).To(Succeed())
					log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

					By("Create Machine2")
					Expect(testClient.Create(ctx, machine2)).To(Succeed())
					log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine2))

					DeferCleanup(func() {
						By("Delete Machine1")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())
						By("Delete Machine2")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine2))).To(Succeed())
					})
				})

				runTest("Expected exactly one machine in namespace")
			})

			Context("DNS names do not match Hostname addresses", func() {
				BeforeEach(func() {
					By("Create Node")
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nodeName,
							Labels: map[string]string{testID: testRunID},
						},
					}
					Expect(testClient.Create(ctx, node)).To(Succeed())
					log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

					DeferCleanup(func() {
						By("Delete Node")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
					})

					machine := &machinev1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine",
							Namespace: testNamespace.Name,
							Labels: map[string]string{
								testID: testRunID,
								"node": node.Name,
							},
						},
					}

					By("Create Machine")
					Expect(testClient.Create(ctx, machine)).To(Succeed())
					log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

					DeferCleanup(func() {
						By("Delete Machine")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())
					})
				})

				runTest("DNS names in CSR do not match Hostname addresses in node object")
			})

			Context("DNS names do not match Hostname addresses", func() {
				BeforeEach(func() {
					By("Create Node")
					node := &corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nodeName,
							Labels: map[string]string{testID: testRunID},
						},
					}
					Expect(testClient.Create(ctx, node)).To(Succeed())
					log.Info("Created Node for test", "node", client.ObjectKeyFromObject(node))

					DeferCleanup(func() {
						By("Delete Node")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
					})

					machine := &machinev1alpha1.Machine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine",
							Namespace: testNamespace.Name,
							Labels: map[string]string{
								testID: testRunID,
								"node": node.Name,
							},
						},
					}

					By("Create Machine")
					Expect(testClient.Create(ctx, machine)).To(Succeed())
					log.Info("Created Machine for test", "machine", client.ObjectKeyFromObject(machine))

					DeferCleanup(func() {
						By("Delete Machine")
						Expect(client.IgnoreNotFound(testClient.Delete(ctx, machine))).To(Succeed())
					})

					By("Patch node's addresses in status")
					patch := client.MergeFrom(node.DeepCopy())
					node.Status.Addresses = []corev1.NodeAddress{
						{Type: corev1.NodeHostName, Address: dnsName1},
						{Type: corev1.NodeHostName, Address: dnsName2},
					}
					Expect(testClient.Status().Patch(ctx, node, patch)).To(Succeed())
				})

				runTest("IP addresses in CSR do not match InternalIP addresses in node object")
			})
		})
	})
})
