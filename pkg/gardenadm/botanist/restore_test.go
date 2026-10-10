// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"crypto/rand"
	"crypto/x509/pkix"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Restore", func() {
	const (
		priorNodeName      = "prior-node"
		oscSecretName      = "gardener-node-agent-control-plane-abc123"
		oscSecretNamespace = "kube-system"
	)

	var (
		b *GardenadmBotanist

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			Build()
		fakeClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		b = &GardenadmBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					Logger:        logr.Discard(),
					SeedClientSet: fakeClientSet,
					Shoot:         &shootpkg.Shoot{ControlPlaneNamespace: oscSecretNamespace},
				},
			},
		}
	})

	Describe("#DeletePriorNode", func() {
		It("should error when priorNodeName is empty", func(ctx SpecContext) {
			Expect(b.DeletePriorNode(ctx, fakeClient, "")).To(MatchError(ContainSubstring("must not be empty")))
		})

		It("should not fail if the prior Node does not exist", func(ctx SpecContext) {
			Expect(b.DeletePriorNode(ctx, fakeClient, priorNodeName)).To(Succeed())
		})

		It("should delete the prior Node", func(ctx SpecContext) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: priorNodeName}}
			other := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "other-node"}}
			Expect(fakeClient.Create(ctx, node)).To(Succeed())
			Expect(fakeClient.Create(ctx, other)).To(Succeed())

			Expect(b.DeletePriorNode(ctx, fakeClient, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(other), other)).To(Succeed())
		})
	})

	Describe("#ForceDeletePriorNodePods", func() {
		podOnNode := func(name, namespace, nodeName string) *corev1.Pod {
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       corev1.PodSpec{NodeName: nodeName},
			}
		}

		It("should error when priorNodeName is empty", func(ctx SpecContext) {
			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, "")).To(MatchError(ContainSubstring("must not be empty")))
		})

		It("should not fail if there are no Pods running on the prior Node", func(ctx SpecContext) {
			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, priorNodeName)).To(Succeed())
		})

		It("should delete all Pods running on the prior Node", func(ctx SpecContext) {
			pod1 := podOnNode("pod-1", "kube-system", priorNodeName)
			pod2 := podOnNode("pod-2", "default", priorNodeName)
			pod3 := podOnNode("pod-3", "some-namespace", priorNodeName)
			pod4 := podOnNode("pod-4", "kube-system", "other-node")
			pod5 := podOnNode("pod-5", "kube-system", "")
			Expect(fakeClient.Create(ctx, pod1)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod2)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod3)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod4)).To(Succeed())
			Expect(fakeClient.Create(ctx, pod5)).To(Succeed())

			Expect(b.ForceDeletePriorNodePods(ctx, fakeClient, priorNodeName)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod2), pod2)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod3), pod3)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod4), pod4)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pod5), pod5)).To(Succeed())
		})
	})

	Describe("#DeleteStaleOperatingSystemConfigSecret", func() {
		BeforeEach(func() {
			b.operatingSystemConfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: oscSecretName, Namespace: oscSecretNamespace},
			}
		})

		It("should delete the stale OperatingSystemConfig Secret restored from the ETCD snapshot", func(ctx SpecContext) {
			restoredSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: oscSecretName, Namespace: oscSecretNamespace},
				Data:       map[string][]byte{"osc.yaml": []byte("managed-content")},
			}
			Expect(fakeClient.Create(ctx, restoredSecret)).To(Succeed())

			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(restoredSecret), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should succeed when the Secret is absent (IgnoreNotFound)", func(ctx SpecContext) {
			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(Succeed())
		})

		It("should do nothing when the OperatingSystemConfig Secret was not computed yet", func(ctx SpecContext) {
			b.operatingSystemConfigSecret = nil

			Expect(b.DeleteStaleOperatingSystemConfigSecret(ctx, fakeClient)).To(Succeed())
		})
	})

	Describe("#FinalizeGardenerNodeAgentManagedResource", func() {
		managedResource := func(finalizers ...string) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       botanistpkg.GardenerNodeAgentManagedResourceName,
					Namespace:  oscSecretNamespace,
					Finalizers: finalizers,
				},
			}
		}

		It("should succeed when the ManagedResource is absent", func(ctx SpecContext) {
			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())
		})

		It("should delete the ManagedResource without finalizers", func(ctx SpecContext) {
			mr := managedResource()
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		})

		It("should remove the finalizer so the ManagedResource is deleted even without a running gardener-resource-manager", func(ctx SpecContext) {
			mr := managedResource("resources.gardener.cloud/gardener-resource-manager")
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(b.FinalizeGardenerNodeAgentManagedResource(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(mr), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteNodeAgentCertificateSigningRequests", func() {
		csrWithCommonName := func(name, signerName, commonName string) *certificatesv1.CertificateSigningRequest {
			privateKey, err := secretsutils.FakeGenerateKey(rand.Reader, 4096)
			Expect(err).NotTo(HaveOccurred())

			csrData, err := certutil.MakeCSR(privateKey, &pkix.Name{CommonName: commonName}, nil, nil)
			Expect(err).NotTo(HaveOccurred())

			return &certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{Name: name},
				Spec: certificatesv1.CertificateSigningRequestSpec{
					Request:    csrData,
					SignerName: signerName,
				},
			}
		}

		It("should succeed when there are no CertificateSigningRequests", func(ctx SpecContext) {
			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())
		})

		It("should delete the gardener-node-agent CertificateSigningRequest", func(ctx SpecContext) {
			csr := csrWithCommonName("node-agent-csr", certificatesv1.KubeAPIServerClientSignerName, v1beta1constants.NodeAgentUserNamePrefix+"machine-1")
			Expect(fakeClient.Create(ctx, csr)).To(Succeed())

			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{})).To(BeNotFoundError())
		})

		It("should delete all gardener-node-agent CertificateSigningRequests", func(ctx SpecContext) {
			csr1 := csrWithCommonName("node-agent-csr-1", certificatesv1.KubeAPIServerClientSignerName, v1beta1constants.NodeAgentUserNamePrefix+"machine-1")
			csr2 := csrWithCommonName("node-agent-csr-2", certificatesv1.KubeAPIServerClientSignerName, v1beta1constants.NodeAgentUserNamePrefix+"machine-2")
			Expect(fakeClient.Create(ctx, csr1)).To(Succeed())
			Expect(fakeClient.Create(ctx, csr2)).To(Succeed())

			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr1), &certificatesv1.CertificateSigningRequest{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr2), &certificatesv1.CertificateSigningRequest{})).To(BeNotFoundError())
		})

		It("should not delete a CertificateSigningRequest with a different signer", func(ctx SpecContext) {
			csr := csrWithCommonName("other-signer-csr", "example.com/other-signer", v1beta1constants.NodeAgentUserNamePrefix+"machine-1")
			Expect(fakeClient.Create(ctx, csr)).To(Succeed())

			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{})).To(Succeed())
		})

		It("should not delete a CertificateSigningRequest whose CommonName lacks the gardener-node-agent prefix", func(ctx SpecContext) {
			csr := csrWithCommonName("foreign-csr", certificatesv1.KubeAPIServerClientSignerName, "gardener.cloud:system:some-other-user")
			Expect(fakeClient.Create(ctx, csr)).To(Succeed())

			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(csr), &certificatesv1.CertificateSigningRequest{})).To(Succeed())
		})

		It("should delete only the matching CertificateSigningRequests and keep the others", func(ctx SpecContext) {
			nodeAgentCSR := csrWithCommonName("node-agent-csr", certificatesv1.KubeAPIServerClientSignerName, v1beta1constants.NodeAgentUserNamePrefix+"machine-1")
			otherSignerCSR := csrWithCommonName("other-signer-csr", "example.com/other-signer", v1beta1constants.NodeAgentUserNamePrefix+"machine-2")
			foreignCSR := csrWithCommonName("foreign-csr", certificatesv1.KubeAPIServerClientSignerName, "gardener.cloud:system:some-other-user")
			Expect(fakeClient.Create(ctx, nodeAgentCSR)).To(Succeed())
			Expect(fakeClient.Create(ctx, otherSignerCSR)).To(Succeed())
			Expect(fakeClient.Create(ctx, foreignCSR)).To(Succeed())

			Expect(b.DeleteNodeAgentCertificateSigningRequests(ctx, fakeClient)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(nodeAgentCSR), &certificatesv1.CertificateSigningRequest{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(otherSignerCSR), &certificatesv1.CertificateSigningRequest{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(foreignCSR), &certificatesv1.CertificateSigningRequest{})).To(Succeed())
		})
	})
})
