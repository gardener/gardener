// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	"context"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/etcd/etcd"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("TLS", func() {
	var (
		ctx        = context.Background()
		namespace  = "test-namespace"
		fakeClient client.Client
		sm         secretsmanager.Interface
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
	})

	Describe("#GenerateServerCertificate", func() {
		It("should generate a server certificate without IP", func() {
			secret, err := GenerateServerCertificate(ctx, sm, testRole, []string{"etcd-main-local"}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("etcd-server-" + testRole))
			Expect(secret.Data).NotTo(BeEmpty())
		})

		It("should generate a server certificate with IP suffix", func() {
			ip := net.ParseIP("10.0.0.1")
			secret, err := GenerateServerCertificate(ctx, sm, testRole, []string{"etcd-main-local"}, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("etcd-server-" + testRole + "-10.0.0.1"))
		})

		It("should not include IP suffix when IP is nil", func() {
			secret, err := GenerateServerCertificate(ctx, sm, testRole, []string{"etcd-main-local"}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Name).To(HavePrefix("etcd-server-" + testRole))
			Expect(secret.Name).NotTo(ContainSubstring("-10."))
		})
	})

	Describe("#GenerateClientCertificate", func() {
		It("should generate a client certificate", func() {
			secret, err := GenerateClientCertificate(ctx, sm)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix(SecretNameClient))
			Expect(secret.Data).NotTo(BeEmpty())
		})
	})

	Describe("#GenerateServerAndClientCertificates", func() {
		When("etcd CA secret is present", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.SecretNameCAETCD,
					Namespace: namespace,
				}})).To(Succeed())
			})

			It("should return CA, server, and client secrets", func() {
				caSecret, serverSecret, clientSecret, err := GenerateServerAndClientCertificates(ctx, sm, testRole, []string{"etcd-main-local"}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(caSecret).NotTo(BeNil())
				Expect(caSecret.Name).To(Equal(v1beta1constants.SecretNameCAETCD))
				Expect(serverSecret).NotTo(BeNil())
				Expect(clientSecret).NotTo(BeNil())
			})
		})

		When("etcd CA secret is missing", func() {
			It("should return an error", func() {
				_, _, _, err := GenerateServerAndClientCertificates(ctx, sm, testRole, []string{"etcd-main-local"}, nil)
				Expect(err).To(MatchError(ContainSubstring(v1beta1constants.SecretNameCAETCD)))
			})
		})
	})

	Describe("#GeneratePeerCertificate", func() {
		It("should generate a peer certificate without IP", func() {
			secret, err := GeneratePeerCertificate(ctx, sm, testRole, []string{"etcd-main-peer"}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("etcd-peer-server-" + testRole))
			Expect(secret.Data).NotTo(BeEmpty())
		})

		It("should generate a peer certificate with IP suffix", func() {
			ip := net.ParseIP("192.168.1.1")
			secret, err := GeneratePeerCertificate(ctx, sm, testRole, []string{"etcd-main-peer"}, ip)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Name).To(HavePrefix("etcd-peer-server-" + testRole + "-192.168.1.1"))
		})
	})

	Describe("#ClientServiceDNSNames", func() {
		It("should return standard DNS names for a non-static-pod etcd", func() {
			Expect(ClientServiceDNSNames("etcd-main", namespace, false)).To(And(
				ContainElements(
					"etcd-main-local",
					"etcd-main-client",
					"etcd-main-client."+namespace,
					"etcd-main-client."+namespace+".svc",
					"etcd-main-client."+namespace+".svc.cluster.local",
				),
				Not(ContainElement("localhost")),
			))
		})

		It("should include localhost when running as static pod", func() {
			Expect(ClientServiceDNSNames("etcd-main", namespace, true)).To(ContainElement("localhost"))
		})

		It("should include wildcard peer DNS names", func() {
			Expect(ClientServiceDNSNames("etcd-main", namespace, false)).To(ContainElement("*.etcd-main-peer"))
		})
	})
})
