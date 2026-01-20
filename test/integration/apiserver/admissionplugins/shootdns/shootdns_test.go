// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootdns_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ShootDNS tests", func() {
	var (
		shoot *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfileName:       &cloudProfile.Name,
				CredentialsBindingName: ptr.To(testCredentialsBinding.Name),
				DNS: &gardencorev1beta1.DNS{
					Domain: ptr.To("test.local.gardener.cloud"),
					Providers: []gardencorev1beta1.DNSProvider{
						{
							Type:       ptr.To("provider-type"),
							Primary:    ptr.To(false),
							SecretName: &testSecret.Name,
							CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
								APIVersion: "v1",
								Kind:       "Secret",
								Name:       testSecret.Name,
							},
						},
					},
				},
				Region: "region",
				Provider: gardencorev1beta1.Provider{
					Type: "provider-type",
					Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Minimum: 2,
							Maximum: 2,
							Machine: gardencorev1beta1.Machine{
								Type: "large",
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    "some-OS",
									Version: ptr.To("1.1.1"),
								},
							},
						},
					},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"},
				Networking: &gardencorev1beta1.Networking{
					Type:     ptr.To("foo-networking"),
					Pods:     ptr.To("100.128.0.0/11"),
					Services: ptr.To("100.72.0.0/13"),
				},
			},
		}
	})

	Context("checkFunctionlessDNSProviders", func() {
		Context("Create", func() {
			It("should allow shoot creation with non-primary DNS provider and configured credentials", func() {
				By("Create Shoot")
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(Succeed())

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
					}).Should(BeNotFoundError())
				})
			})

			It("should allow shoot creation when shoot.spec.dns is not set", func() {
				shoot.Spec.DNS = nil
				By("Create Shoot")
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(Succeed())

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
					}).Should(BeNotFoundError())
				})
			})

			It("should allow shoot creation when dnsProvider has secretName set and credentialsRef unset", func() {
				shoot.Spec.DNS.Providers[0].CredentialsRef = nil
				By("Create Shoot")
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(Succeed())

				By("Ensure credentialsRef has been synced with secretName")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				}).Should(Succeed())

				Expect(shoot.Spec.DNS.Providers[0].CredentialsRef).To(Equal(&autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       testSecret.Name,
				}))

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
					}).Should(BeNotFoundError())
				})
			})

			It("should allow shoot creation when dnsProvider has secretName unset and credentialsRef set", func() {
				shoot.Spec.DNS.Providers[0].SecretName = nil
				By("Create Shoot")
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(Succeed())

				By("Ensure credentialsRef has been synced with secretName")
				Eventually(func() error {
					return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
				}).Should(Succeed())

				Expect(shoot.Spec.DNS.Providers[0].SecretName).To(Equal(&testSecret.Name))

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
					Eventually(func() error {
						return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
					}).Should(BeNotFoundError())
				})
			})

			It("should forbid shoot creation when dnsProvider is not primary and has no credentials configured", func() {
				shoot.Spec.DNS.Providers[0] = gardencorev1beta1.DNSProvider{
					Primary: ptr.To(false),
					Type:    ptr.To("provider-type"),
				}
				By("Create Shoot")
				Expect(testClient.Create(ctx, shoot)).To(And(
					HaveOccurred(),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"ErrStatus": MatchFields(IgnoreExtras, Fields{
							"Code":    Equal(int32(http.StatusUnprocessableEntity)),
							"Message": ContainSubstring("Required value: non-primary DNS providers must specify `type` and `credentialsRef`"),
						})},
					)),
				))

				DeferCleanup(func() {
					// Only when the shoot is created the name is set and it should be deleted.
					if shoot.Name != "" {
						By("Delete Shoot")
						Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
						Eventually(func() error {
							return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
						}).Should(BeNotFoundError())
					}
				})
			})

			It("should forbid shoot creation when dnsProvider is not primary, has no type, credentialsRef is unset but secretName is set", func() {
				shoot.Spec.DNS.Providers[0] = gardencorev1beta1.DNSProvider{
					Primary:    ptr.To(false),
					Type:       nil,
					SecretName: &testSecret.Name,
				}
				By("Create Shoot")
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).Should(And(
					HaveOccurred(),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"ErrStatus": MatchFields(IgnoreExtras, Fields{
							"Code":    Equal(int32(http.StatusUnprocessableEntity)),
							"Message": ContainSubstring("Required value: type must be set when credentialsRef is set"),
						})},
					)),
				))

				DeferCleanup(func() {
					// Only when the shoot is created the name is set and it should be deleted.
					if shoot.Name != "" {
						By("Delete Shoot")
						Expect(testClient.Delete(ctx, shoot)).To(Or(Succeed(), BeNotFoundError()))
						Eventually(func() error {
							return testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)
						}).Should(BeNotFoundError())
					}
				})
			})
		})

		Context("Update", func() {
			BeforeEach(func() {
				By("Create Seed")
				seed := &gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: testID + "-",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Region: "region",
							Type:   "provider-type",
						},
						Ingress: &gardencorev1beta1.Ingress{
							Domain: "seed.example.com",
							Controller: gardencorev1beta1.IngressController{
								Kind: "nginx",
							},
						},
						DNS: gardencorev1beta1.SeedDNS{
							Provider: &gardencorev1beta1.SeedDNSProvider{
								Type: "provider",
								CredentialsRef: &corev1.ObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "some-secret",
									Namespace:  "some-namespace",
								},
							},
							Internal: &gardencorev1beta1.SeedDNSProviderConfig{
								Type:   "provider",
								Domain: "local.example.com",
								CredentialsRef: corev1.ObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "some-secret",
									Namespace:  "some-namespace",
								},
							},
						},
						Settings: &gardencorev1beta1.SeedSettings{
							Scheduling: &gardencorev1beta1.SeedSettingScheduling{Visible: true},
						},
						Networks: gardencorev1beta1.SeedNetworks{
							Pods:     "10.0.0.0/16",
							Services: "10.1.0.0/16",
							Nodes:    ptr.To("10.2.0.0/16"),
						},
					},
				}
				Expect(testClient.Create(ctx, seed)).To(Succeed())
				log.Info("Created Seed for test", "seed", client.ObjectKeyFromObject(seed))

				DeferCleanup(func() {
					By("Delete Seed")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, seed))).To(Succeed())
				})

				By("Pre-create the shoot with spec.seedName set otherwise the validation would be skipped")
				shoot.Spec.SeedName = &seed.Name
				Eventually(func() error {
					return testClient.Create(ctx, shoot)
				}).To(Succeed())

				DeferCleanup(func() {
					By("Delete Shoot")
					Expect(client.IgnoreNotFound(testClient.Delete(ctx, shoot))).To(Succeed())
				})
			})

			It("should allow shoot update with non-primary DNS provider and configured credentials", func() {
				shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: ptr.To(false)}
				By("Update Shoot")
				Eventually(func() error {
					return testClient.Update(ctx, shoot)
				}).Should(Succeed())

			})

			It("should forbid shoot update with non-primary DNS provider and not configured credentials", func() {
				shoot.Spec.DNS.Providers = append(shoot.Spec.DNS.Providers, gardencorev1beta1.DNSProvider{
					Primary: ptr.To(false),
					Type:    ptr.To("another-provider-type"),
				})
				By("Update Shoot")
				Eventually(func() error {
					return testClient.Update(ctx, shoot)
				}).Should(And(
					HaveOccurred(),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"ErrStatus": MatchFields(IgnoreExtras, Fields{
							"Code":    Equal(int32(http.StatusUnprocessableEntity)),
							"Message": ContainSubstring("Required value: non-primary DNS providers must specify `type` and `credentialsRef`"),
						})},
					)),
				))
			})

			It("should allow shoot update with non-primary DNS provider and only secretName set", func() {
				shoot.Spec.DNS.Providers = append(shoot.Spec.DNS.Providers, gardencorev1beta1.DNSProvider{
					Primary:    ptr.To(false),
					Type:       ptr.To("another-provider-type"),
					SecretName: &testSecret.Name,
				})
				By("Update Shoot")
				Eventually(func() error {
					return testClient.Update(ctx, shoot)
				}).Should(Succeed())
			})

			It("should allow shoot update with non-primary DNS provider and only credentialsRef set", func() {
				shoot.Spec.DNS.Providers = append(shoot.Spec.DNS.Providers, gardencorev1beta1.DNSProvider{
					Primary: ptr.To(false),
					Type:    ptr.To("another-provider-type"),
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       testSecret.Name,
					},
				})
				By("Update Shoot")
				Eventually(func() error {
					return testClient.Update(ctx, shoot)
				}).Should(Succeed())
			})

			It("should allow shoot patch to try to unset credentialsRef", func() {
				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.DNS.Providers[0].CredentialsRef = nil

				By("Patch Shoot")
				Eventually(func() error {
					return testClient.Patch(ctx, shoot, patch)
				}).Should(Succeed())
			})
		})
	})
})
