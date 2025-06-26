// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"crypto/rsa"
	"io"
	"net"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

const name = "e2e-test-bastion"

// VerifyBastion tests the Bastion functionality of a shoot cluster.
func VerifyBastion(s *ShootContext) {
	GinkgoHelper()

	Describe("Bastion", Label("bastion"), func() {
		var (
			bastion *operationsv1alpha1.Bastion
			closers []io.Closer
		)

		BeforeAll(func() {
			bastion = &operationsv1alpha1.Bastion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name + "-" + s.Shoot.Name,
					Namespace: s.Shoot.Namespace,
				},
				Spec: operationsv1alpha1.BastionSpec{
					ShootRef: corev1.LocalObjectReference{
						Name: s.Shoot.Name,
					},
					Ingress: []operationsv1alpha1.BastionIngressPolicy{{
						IPBlock: networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
						},
					}},
				},
			}

			DeferCleanup(func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return s.GardenClient.Delete(ctx, bastion)
				}).Should(Or(Succeed(), BeNotFoundError()))
			}, NodeTimeout(time.Minute))

			// Close the remaining open connections (if any) on best effort basis.
			closers = nil
			DeferCleanup(func() {
				for _, closer := range closers {
					// We ignore the error, as closing could result in "use of closed network connection".
					_ = closer.Close()
				}
			})
		})

		var nodeSSHKey []byte
		It("should fetch the shoot SSH key", func(ctx SpecContext) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      gardenerutils.ComputeShootProjectResourceName(s.Shoot.Name, gardenerutils.ShootProjectSecretSuffixSSHKeypair),
				Namespace: s.Shoot.Namespace,
			}}
			Eventually(ctx, s.GardenKomega.Get(secret)).Should(Succeed())

			nodeSSHKey = secret.Data[secretsutils.DataKeyRSAPrivateKey]
		}, SpecTimeout(time.Minute))

		var nodeName, nodeAddr string
		It("should pick a shoot Node with InternalIP", func(ctx SpecContext) {
			var nodes []corev1.Node
			Eventually(ctx, s.ShootKomega.ObjectList(&corev1.NodeList{})).Should(
				HaveField("Items", ContainElement(
					HaveField("Status.Addresses", ContainElement(
						HaveField("Type", corev1.NodeInternalIP),
					)),
					&nodes,
				)),
			)

			nodeName = nodes[0].Name
			AddReportEntry("node-name", nodeName)

			nodeAddresses := nodes[0].Status.Addresses
			nodeInternalIP := nodeAddresses[slices.IndexFunc(nodeAddresses, func(address corev1.NodeAddress) bool {
				return address.Type == corev1.NodeInternalIP
			})].Address
			AddReportEntry("node-internal-ip", nodeInternalIP)

			nodeAddr = net.JoinHostPort(nodeInternalIP, "22")
		}, SpecTimeout(time.Minute))

		It("should ensure there are no other Bastions", func(ctx SpecContext) {
			// For now, the local setup supports only a single Bastion at a time because there is only one LoadBalancer IP.
			// With this step, we try to avoid creating multiple Bastions at the same time that interfere with each other.
			bastionList := &operationsv1alpha1.BastionList{}
			Eventually(ctx, s.GardenKomega.ObjectList(bastionList)).
				WithPolling(5 * time.Second).
				Should(HaveField("Items", BeEmpty()))
		}, SpecTimeout(5*time.Minute))

		var bastionSSHKey *rsa.PrivateKey
		It("should create the Bastion", func(ctx SpecContext) {
			sshKeyInterface, err := (&secretsutils.RSASecretConfig{
				Name:       name,
				Bits:       4096,
				UsedForSSH: true,
			}).Generate()
			Expect(err).NotTo(HaveOccurred())
			sshKey := sshKeyInterface.(*secretsutils.RSAKeys)

			bastionSSHKey = sshKey.PrivateKey

			bastion.Spec.SSHPublicKey = string(sshKey.SecretData()[secretsutils.DataKeySSHAuthorizedKeys])

			Eventually(ctx, func() error {
				err := s.GardenClient.Create(ctx, bastion)
				if apierrors.IsAlreadyExists(err) {
					return StopTrying(err.Error())
				}
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		var bastionAddr string
		It("should get ready", func(ctx SpecContext) {
			Eventually(ctx, s.GardenKomega.Object(bastion)).Should(And(
				HaveField("Status.Conditions", ConsistOf(And(
					HaveField("Type", operationsv1alpha1.BastionReady),
					HaveField("Status", gardencorev1beta1.ConditionTrue),
				))),
				HaveField("Status.Ingress", Not(BeNil())),
			))

			bastionHost := bastion.Status.Ingress.IP
			if bastionHost == "" {
				bastionHost = bastion.Status.Ingress.Hostname
			}
			AddReportEntry("bastion-host", bastionHost)
			bastionAddr = net.JoinHostPort(bastionHost, "22")
		}, SpecTimeout(5*time.Minute))

		var bastionConnection *sshutils.Connection
		It("should connect to the Bastion", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				bastionConnection, err = sshutils.Dial(ctx, bastionAddr,
					sshutils.WithUser("gardener"), sshutils.WithPrivateKey(bastionSSHKey),
				)
				return err
			}).Should(Succeed())
			closers = append(closers, bastionConnection)
		}, SpecTimeout(time.Minute))

		It("should connect to the Node via Bastion", func(ctx SpecContext) {
			var nodeConnection *sshutils.Connection
			Eventually(ctx, func() error {
				var err error
				nodeConnection, err = sshutils.Dial(ctx, nodeAddr,
					sshutils.WithProxyConnection(bastionConnection),
					sshutils.WithUser("gardener"), sshutils.WithPrivateKeyBytes(nodeSSHKey),
				)
				return err
			}).Should(Succeed())
			closers = append(closers, nodeConnection)

			Eventually(ctx, execute(nodeConnection, "hostname")).Should(gbytes.Say(nodeName))
		}, SpecTimeout(time.Minute))

		It("should delete the Bastion", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				return s.GardenClient.Delete(ctx, bastion)
			}).Should(Or(Succeed(), BeNotFoundError()))
		}, SpecTimeout(time.Minute))

		It("should get deleted", func(ctx SpecContext) {
			Eventually(ctx, s.GardenKomega.Get(bastion)).Should(BeNotFoundError())
		}, SpecTimeout(time.Minute))
	})
}

func execute(c *sshutils.Connection, command string) *gbytes.Buffer {
	GinkgoHelper()

	combinedBuffer := gbytes.NewBuffer()
	Expect(c.RunWithStreams(
		nil,
		io.MultiWriter(combinedBuffer, gexec.NewPrefixedWriter("[out] ", GinkgoWriter)),
		io.MultiWriter(combinedBuffer, gexec.NewPrefixedWriter("[err] ", GinkgoWriter)),
		command,
	)).To(Succeed())

	return combinedBuffer
}
