// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/extensions/bastion"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bastion", func() {
	const (
		name      = "test"
		namespace = "shoot--foo--bar"
		provider  = "local"
	)

	var (
		ctx = context.Background()

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		k                 komega.Komega

		b *Bastion

		bastion *extensionsv1alpha1.Bastion
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithStatusSubresource(&extensionsv1alpha1.Bastion{}).
			Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		k = komega.New(fakeClient)

		DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))

		b = New(logr.Discard(), fakeClient, fakeSecretManager, &Values{
			Name:      name,
			Namespace: namespace,
			Provider:  provider,
		})
		b.Clock = testclock.NewFakePassiveClock(time.Date(2025, 06, 25, 0, 0, 0, 0, time.UTC))
		b.WaitInterval = time.Millisecond
		b.WaitSevereThreshold = 250 * time.Millisecond
		b.WaitTimeout = 500 * time.Millisecond

		bastion = &extensionsv1alpha1.Bastion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should deploy the SSH key pair and Bastion resource", func() {
			Expect(b.Deploy(ctx)).To(Succeed())
			Expect(k.Get(bastion)()).To(Succeed())

			Expect(k.ObjectList(&corev1.SecretList{}, client.InNamespace(namespace), client.MatchingLabels{"name": "bastion-" + name + "-ssh-keypair"})()).To(
				HaveField("Items", ConsistOf(
					HaveField("Data", And(
						HaveKey(secretsutils.DataKeyRSAPrivateKey),
						HaveKey(secretsutils.DataKeySSHAuthorizedKeys),
					)),
				)),
			)

			Expect(bastion.Annotations).To(HaveKeyWithValue("gardener.cloud/operation", "reconcile"))
			Expect(bastion.Annotations).To(HaveKeyWithValue("gardener.cloud/timestamp", "2025-06-25T00:00:00Z"))

			Expect(bastion.Spec.DefaultSpec).To(Equal(extensionsv1alpha1.DefaultSpec{Type: provider}))
			Expect(bastion.Spec.Ingress).To(ConsistOf(
				extensionsv1alpha1.BastionIngressPolicy{
					IPBlock: networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
				},
				extensionsv1alpha1.BastionIngressPolicy{
					IPBlock: networkingv1.IPBlock{CIDR: "::/0"},
				},
			))
			Expect(string(bastion.Spec.UserData)).To(Equal(`#!/bin/bash -eu

id gardener || useradd gardener -mU
mkdir -p /home/gardener/.ssh
echo "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDJXpm5HiBLMIX/M7TrES/pEaYNSdc4cBlVyTilWFj8h3yPbOOtWUFmnWSUBEr8y3UE86ZEpC2DvndQJ7BiOlF1OHLOyJwrPgWWDibvannttaz/CL6PY3lFzR4X3xHL/8VjoZuzlUlWLPtZJ8ShdpIURgiS/4IooBD0nSSSJjO2LLP6n+5IPuwg4BWSyPgzn8P7gZW2olX7hpJ1Si2i556EnV/CZz9lOxzMxcCctxXoE/03QZfltQFb6z8dIwud0TL4ZLJ7Up2AtmKXMCh2a161B0tgI5dmyK990J4XyWwuMtX+i4Az4XDAzlBtTWL6JhGpWTwCnLOz1Yy+4CnyarlR" > /home/gardener/.ssh/authorized_keys
chown gardener:gardener /home/gardener/.ssh/authorized_keys
systemctl start ssh
`))
		})

		It("should use the configured CIDRs", func() {
			b.Values.IngressCIDRs = []string{"1.2.3.4/32", "4.3.2.1/32", "2001:db8::1/128"}

			Expect(b.Deploy(ctx)).To(Succeed())

			Expect(k.Get(bastion)()).To(Succeed())

			Expect(bastion.Spec.Ingress).To(ConsistOf(
				extensionsv1alpha1.BastionIngressPolicy{
					IPBlock: networkingv1.IPBlock{CIDR: "1.2.3.4/32"},
				},
				extensionsv1alpha1.BastionIngressPolicy{
					IPBlock: networkingv1.IPBlock{CIDR: "4.3.2.1/32"},
				},
				extensionsv1alpha1.BastionIngressPolicy{
					IPBlock: networkingv1.IPBlock{CIDR: "2001:db8::1/128"},
				},
			))
		})
	})

	Describe("#Wait", func() {
		When("there is no Bastion resource", func() {
			It("should return a not found error", func() {
				Expect(b.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})
		})

		When("the Bastion has been deployed", func() {
			BeforeEach(func() {
				Expect(b.Deploy(ctx)).To(Succeed())
			})

			When("the Bastion has an error", func() {
				BeforeEach(func() {
					Expect(k.UpdateStatus(bastion, func() {
						bastion.Status.LastError = &gardencorev1beta1.LastError{
							Description: "infra error",
						}
					})()).To(Succeed())
				})

				It("should return the Bastion error", func() {
					Expect(b.Wait(ctx)).To(MatchError(ContainSubstring("infra error")))
				})
			})

			When("the Bastion is ready", func() {
				BeforeEach(func() {
					Expect(k.Update(bastion, func() {
						delete(bastion.Annotations, "gardener.cloud/operation")
					})()).To(Succeed())

					Expect(k.UpdateStatus(bastion, func() {
						bastion.Status.LastOperation = &gardencorev1beta1.LastOperation{
							LastUpdateTime: metav1.NewTime(b.Clock.Now().Add(time.Second)),
							State:          "Succeeded",
						}
						bastion.Status.Ingress = &corev1.LoadBalancerIngress{
							IP: "1.1.1.1",
						}
					})()).To(Succeed())
				})

				It("should return an SSH connection error", func() {
					b.SSHDial = func(_ context.Context, addr string, _ ...sshutils.Option) (*sshutils.Connection, error) {
						Expect(addr).To(Equal("1.1.1.1:22"))
						return nil, fmt.Errorf("connection refused")
					}

					Expect(b.Wait(ctx)).To(MatchError(ContainSubstring("connection refused")))
				})

				It("should succeed and store connection", func() {
					conn := &sshutils.Connection{}
					b.SSHDial = func(_ context.Context, addr string, _ ...sshutils.Option) (*sshutils.Connection, error) {
						Expect(addr).To(Equal("1.1.1.1:22"))
						return conn, nil
					}

					Expect(b.Wait(ctx)).To(Succeed())
					Expect(b.Connection).To(BeIdenticalTo(conn))
				})

				When("the Bastion has an ingress hostname instead of IP", func() {
					BeforeEach(func() {
						Expect(k.UpdateStatus(bastion, func() {
							bastion.Status.Ingress = &corev1.LoadBalancerIngress{
								Hostname: "bastion.example.com",
							}
						})()).To(Succeed())
					})

					It("should succeed and store connection", func() {
						conn := &sshutils.Connection{}
						b.SSHDial = func(_ context.Context, addr string, _ ...sshutils.Option) (*sshutils.Connection, error) {
							Expect(addr).To(Equal("bastion.example.com:22"))
							return conn, nil
						}

						Expect(b.Wait(ctx)).To(Succeed())
						Expect(b.Connection).To(BeIdenticalTo(conn))
					})
				})

				When("the Bastion is missing an ingress IP", func() {
					BeforeEach(func() {
						Expect(k.UpdateStatus(bastion, func() {
							bastion.Status.Ingress = nil
						})()).To(Succeed())
					})

					It("should return an error", func() {
						Expect(b.Wait(ctx)).To(MatchError(ContainSubstring("missing ingress status")))
					})
				})
			})
		})
	})

	Describe("#Destroy", func() {
		When("the Bastion has not been deployed", func() {
			It("should do nothing", func() {
				Expect(b.Destroy(ctx)).To(Succeed())
			})
		})

		When("the Bastion has been deployed", func() {
			BeforeEach(func() {
				Expect(b.Deploy(ctx)).To(Succeed())
			})

			It("should delete all resources", func() {
				Expect(b.Destroy(ctx)).To(Succeed())

				Expect(k.Get(bastion)()).To(BeNotFoundError())
				Expect(k.ObjectList(&corev1.SecretList{})()).To(HaveField("Items", BeEmpty()))
			})

			When("the connection has been established", func() {
				var conn *fakeSSHConn

				BeforeEach(func() {
					conn = &fakeSSHConn{}

					b.Connection = &sshutils.Connection{
						Client: &ssh.Client{
							Conn: conn,
						},
					}
				})

				It("should close the connection", func() {
					Expect(b.Destroy(ctx)).To(Succeed())
					Expect(conn.closed).To(BeTrue())
				})

				It("should successfully destroy the Bastion even if closing the connection fails", func() {
					conn.err = fmt.Errorf("fake")

					Expect(b.Destroy(ctx)).To(Succeed())
					Expect(conn.closed).To(BeTrue())
					Expect(k.Get(bastion)()).To(BeNotFoundError())
				})
			})
		})
	})

	Describe("#WaitCleanup", func() {
		When("the Bastion deletion has an error", func() {
			BeforeEach(func() {
				Expect(b.Deploy(ctx)).To(Succeed())
				Expect(k.UpdateStatus(bastion, func() {
					bastion.Status.LastError = &gardencorev1beta1.LastError{
						Description: "deletion error",
					}
				})()).To(Succeed())
			})

			It("should return the deletion error", func() {
				Expect(b.WaitCleanup(ctx)).To(MatchError(ContainSubstring("deletion error")))
			})
		})

		When("the Bastion is gone", func() {
			It("should succeed", func() {
				Expect(b.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

type fakeSSHConn struct {
	ssh.Conn

	closed bool
	err    error
}

func (c *fakeSSHConn) Close() error {
	c.closed = true
	return c.err
}
