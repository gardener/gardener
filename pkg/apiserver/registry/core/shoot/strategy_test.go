// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apiserver/registry/core/shoot"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.Background()
		strategy rest.RESTCreateUpdateStrategy
	)

	BeforeEach(func() {
		strategy = NewStrategy(0)
	})

	Describe("#Validate", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: core.ShootSpec{
					CloudProfileName: ptr.To("aws-profile"),
					Region:           "eu-west-1",
					Kubernetes: core.Kubernetes{
						Version: "1.31.2",
					},
					Provider: core.Provider{
						Type:    "provider",
						Workers: []core.Worker{},
					},
				},
			}
		})

		It("should allow an empty worker list", func() {
			Expect(strategy.Validate(ctx, shoot)).To(BeEmpty())
		})
	})

	Describe("#PrepareForCreate", func() {
		Context("cloudProfile field fallback", func() {
			var (
				shoot *core.Shoot
			)

			BeforeEach(func() {
				shoot = &core.Shoot{}
			})

			It("should fill cloudProfile field with fallback if empty", func() {
				shoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForCreate(ctx, shoot)

				Expect(*shoot.Spec.CloudProfileName).To(Equal("foo"))
				Expect(shoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "foo",
				}))
			})

			It("should fill cloudProfileName field with fallback if empty and CloudProfile is used", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}
				strategy.PrepareForCreate(ctx, shoot)

				Expect(*shoot.Spec.CloudProfileName).To(Equal("bar"))
				Expect(shoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}))
			})

			It("should override cloudProfileName field on conflicting entry with cloudProfile", func() {
				shoot.Spec.CloudProfileName = ptr.To("foo")
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}
				strategy.PrepareForCreate(ctx, shoot)

				Expect(*shoot.Spec.CloudProfileName).To(Equal("bar"))
				Expect(shoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}))
			})

			It("should unset cloudProfileName field if NamespacedCloudProfile is referenced", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				shoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForCreate(ctx, shoot)

				Expect(shoot.Spec.CloudProfileName).To(BeNil())
				Expect(shoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}))
			})
		})

		Context("DNS Provider Credentials", func() {
			// TODO(vpnachev): Remove this context once support for Kubernetes 1.34 is dropped.
			It("should sync Secret credentialsRef to secretName", func() {
				shoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret-1",
								},
							}},
						},
					},
				}

				strategy.PrepareForCreate(ctx, shoot)
				Expect(shoot.Spec.DNS.Providers[0].SecretName).To(Equal(ptr.To("secret-1")))
			})

			It("should not sync WorkloadIdentity credentialsRef to secretName", func() {
				shoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "security.gardener.cloud/v1alpha1",
									Kind:       "WorkloadIdentity",
									Name:       "workload-identity-1",
								},
							}},
						},
					},
				}

				strategy.PrepareForCreate(ctx, shoot)
				Expect(shoot.Spec.DNS.Providers[0].SecretName).To(BeNil())
			})
		})
	})

	Describe("#PrepareForUpdate", func() {
		var (
			oldShoot *core.Shoot
			newShoot *core.Shoot
		)

		BeforeEach(func() {
			oldShoot = &core.Shoot{}
			newShoot = oldShoot.DeepCopy()
		})

		Context("cloudProfile field removal", func() {
			It("should fill cloudProfile field with fallback if empty", func() {
				newShoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(*newShoot.Spec.CloudProfileName).To(Equal("foo"))
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "foo",
				}))
			})

			It("should fill cloudProfileName field with fallback if empty and CloudProfile is used", func() {
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(*newShoot.Spec.CloudProfileName).To(Equal("bar"))
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}))
			})

			It("should override cloudProfileName field on conflicting entry with cloudProfile", func() {
				newShoot.Spec.CloudProfileName = ptr.To("foo")
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(*newShoot.Spec.CloudProfileName).To(Equal("bar"))
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "bar",
				}))
			})

			It("should unset cloudProfileName field if NamespacedCloudProfile is referenced", func() {
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				newShoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.CloudProfileName).To(BeNil())
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}))
			})

			It("should not mutate shoots being deleted (cloud profile sync)", func() {
				oldShoot.Spec.CloudProfileName = ptr.To("profile")
				oldShoot.DeletionTimestamp = ptr.To(metav1.Now())
				newShoot = oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec).To(Equal(oldShoot.Spec))
			})
		})

		DescribeTable("should sync encrypted resources from status.encryptedResources to status.credentials.encryptionAtRest.resources",
			func(status core.ShootStatus, expected core.ShootStatus) {
				oldShoot.Status = status
				newShoot.Status = status

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Status).To(Equal(expected))
			},
			Entry("no encrypted resources", core.ShootStatus{}, core.ShootStatus{}),
			Entry("with encrypted resources",
				core.ShootStatus{
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
				core.ShootStatus{
					Credentials: &core.ShootCredentials{
						EncryptionAtRest: &core.EncryptionAtRest{
							Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
						},
					},
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
			),
			Entry("should overwrite", core.ShootStatus{
				Credentials: &core.ShootCredentials{
					EncryptionAtRest: &core.EncryptionAtRest{
						Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
					},
				},
				EncryptedResources: []string{"configmaps"},
			},
				core.ShootStatus{
					Credentials: &core.ShootCredentials{
						EncryptionAtRest: &core.EncryptionAtRest{
							Resources: []string{"configmaps"},
						},
					},
					EncryptedResources: []string{"configmaps"},
				},
			),
		)

		Context("seedName change", func() {
			BeforeEach(func() {
				oldShoot = &core.Shoot{
					Spec: core.ShootSpec{
						SeedName: ptr.To("seed"),
					},
				}
				newShoot = oldShoot.DeepCopy()
			})

			It("should not allow change of seedName on shoot spec update", func() {
				newShoot.Spec.SeedName = ptr.To("new-seed")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.SeedName).To(Equal(oldShoot.Spec.SeedName))
			})
		})

		Context("generation increment", func() {
			var (
				oldShoot *core.Shoot
				newShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = &core.Shoot{}
				newShoot = oldShoot.DeepCopy()
			})

			DescribeTable("standard tests",
				func(mutateNewShoot func(*core.Shoot), shouldIncreaseGeneration bool) {
					if mutateNewShoot != nil {
						mutateNewShoot(newShoot)
					}

					strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

					expectedGeneration := oldShoot.Generation
					if shouldIncreaseGeneration {
						expectedGeneration++
					}

					Expect(newShoot.Generation).To(Equal(expectedGeneration))
				},

				Entry("no change",
					nil,
					false,
				),
				Entry("only label change",
					func(s *core.Shoot) { s.Labels = map[string]string{"foo": "bar"} },
					false,
				),
				Entry("some spec change",
					func(s *core.Shoot) { s.Spec.Region = "foo" },
					true,
				),
				Entry("deletion timestamp gets set",
					func(s *core.Shoot) {
						deletionTimestamp := metav1.Now()
						s.DeletionTimestamp = &deletionTimestamp
					},
					true,
				),
				Entry("force-deletion annotation",
					func(s *core.Shoot) {
						metav1.SetMetaDataAnnotation(&s.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
					},
					true,
				),
			)

			Context("confine spec update rollout", func() {
				DescribeTable("confine spec update rollout",
					func(confineSpecUpdateRolloutOld, confineSpecUpdateRolloutNew *bool, mutateOldShoot, mutateNewShoot func(*core.Shoot), shouldIncreaseGeneration bool) {
						if confineSpecUpdateRolloutOld != nil {
							oldShoot.Spec.Maintenance = &core.Maintenance{ConfineSpecUpdateRollout: confineSpecUpdateRolloutOld}
						}
						if confineSpecUpdateRolloutNew != nil {
							newShoot.Spec.Maintenance = &core.Maintenance{ConfineSpecUpdateRollout: confineSpecUpdateRolloutNew}
						}

						if mutateOldShoot != nil {
							mutateOldShoot(oldShoot)
						}
						if mutateNewShoot != nil {
							mutateNewShoot(newShoot)
						}

						strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

						expectedGeneration := oldShoot.Generation
						if shouldIncreaseGeneration {
							expectedGeneration++
						}

						Expect(newShoot.Generation).To(Equal(expectedGeneration))
					},

					Entry("confineSpecUpdateRollout true->false",
						ptr.To(true), ptr.To(false),
						nil, nil,
						true,
					),
					Entry("confineSpecUpdateRollout false->true",
						ptr.To(false), ptr.To(true),
						nil, nil,
						false,
					),
					Entry("confineSpecUpdateRollout nil->false w/ additional spec change",
						nil, ptr.To(false),
						nil, func(s *core.Shoot) { s.Spec.Region = "foo" },
						true,
					),
					Entry("confineSpecUpdateRollout true->true w/ additional spec change",
						ptr.To(true), ptr.To(true),
						nil, func(s *core.Shoot) { s.Spec.Region = "foo" },
						false,
					),

					// exceptional cases: spec.hibernation.enabled changes even if confineSpecUpdateRollout is true
					Entry("hibernation nil -> nil",
						ptr.To(true), ptr.To(true),
						nil, nil,
						false,
					),
					Entry("hibernation nil -> false",
						ptr.To(true), ptr.To(true),
						nil, func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						false,
					),
					Entry("hibernation nil -> true",
						ptr.To(true), ptr.To(true),
						nil, func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						true,
					),

					Entry("hibernation enabled nil -> false",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						false,
					),
					Entry("hibernation enabled nil -> true",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						true,
					),
					Entry("hibernation enabled nil -> hibernation nil",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						nil,
						false,
					),

					Entry("hibernation enabled true -> true",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						false,
					),
					Entry("hibernation enabled true -> false",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						true,
					),
					Entry("hibernation enabled true -> nil",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						true,
					),
					Entry("hibernation enabled true -> hibernation nil",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						nil,
						true,
					),

					Entry("hibernation enabled false -> true",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)} },
						true,
					),
					Entry("hibernation enabled false -> false",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						false,
					),
					Entry("hibernation enabled false -> nil",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						false,
					),
					Entry("hibernation enabled false -> hibernation nil",
						ptr.To(true), ptr.To(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)} },
						nil,
						false,
					),
				)
			})

			DescribeTable("operation annotations",
				func(operationAnnotation string, mutateOldShoot func(*core.Shoot), shouldIncreaseGeneration bool, mutatedAnnotation []string) {
					oldShoot := &core.Shoot{
						Spec: core.ShootSpec{
							Provider: core.Provider{
								Workers: []core.Worker{
									{
										Name: "worker",
									},
								},
							},
						},
						Status: core.ShootStatus{
							LastOperation: &core.LastOperation{},
						},
					}

					if mutateOldShoot != nil {
						mutateOldShoot(oldShoot)
					}

					newShoot := oldShoot.DeepCopy()
					newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: operationAnnotation}

					strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

					expectedGeneration := oldShoot.Generation
					if shouldIncreaseGeneration {
						expectedGeneration++
					}
					Expect(newShoot.Generation).To(Equal(expectedGeneration))

					if mutatedAnnotation != nil {
						Expect(newShoot.Annotations).To(HaveKey(v1beta1constants.GardenerOperation))
						Expect(utils.SplitAndTrimString(newShoot.Annotations[v1beta1constants.GardenerOperation], v1beta1constants.GardenerOperationsSeparator)).To(ConsistOf(mutatedAnnotation))
					} else {
						Expect(newShoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
					}
				},

				Entry("retry; last operation is failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateFailed },
					true,
					nil,
				),
				Entry("retry; last operation is not failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateSucceeded },
					false,
					[]string{v1beta1constants.ShootOperationRetry},
				),
				Entry("retry; last operation is not set",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation = nil },
					false,
					[]string{v1beta1constants.ShootOperationRetry},
				),
				Entry("reconcile",
					v1beta1constants.GardenerOperationReconcile,
					nil,
					true,
					nil,
				),

				Entry("rotate-credentials-start",
					v1beta1constants.OperationRotateCredentialsStart,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsStart},
				),
				Entry("rotate-credentials-start-without-workers-rollout",
					v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout},
				),
				Entry("rotate-credentials-complete",
					v1beta1constants.OperationRotateCredentialsComplete,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsComplete},
				),

				Entry("rotate-ssh-keypair (ssh enabled)",
					v1beta1constants.ShootOperationRotateSSHKeypair,
					nil,
					true,
					[]string{v1beta1constants.ShootOperationRotateSSHKeypair},
				),
				Entry("rotate-ssh-keypair (ssh is not enabled)",
					v1beta1constants.ShootOperationRotateSSHKeypair,
					func(s *core.Shoot) { s.Spec.Provider.Workers = nil },
					false,
					nil,
				),
				Entry("rotate-observability-credentials",
					v1beta1constants.OperationRotateObservabilityCredentials,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateObservabilityCredentials},
				),

				Entry("rotate-etcd-encryption-key",
					v1beta1constants.OperationRotateETCDEncryptionKey,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateETCDEncryptionKey},
				),
				Entry("rotate-etcd-encryption-key-start",
					v1beta1constants.OperationRotateETCDEncryptionKeyStart,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateETCDEncryptionKeyStart},
				),
				Entry("rotate-etcd-encryption-key-complete",
					v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateETCDEncryptionKeyComplete},
				),

				Entry("rotate-ca-start",
					v1beta1constants.OperationRotateCAStart,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCAStart},
				),
				Entry("rotate-ca-start-without-workers-rollout",
					v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCAStartWithoutWorkersRollout},
				),
				Entry("rotate-ca-complete",
					v1beta1constants.OperationRotateCAComplete,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCAComplete},
				),

				Entry("rotate-serviceaccount-key-start",
					v1beta1constants.OperationRotateServiceAccountKeyStart,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateServiceAccountKeyStart},
				),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout",
					v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout},
				),
				Entry("rotate-serviceaccount-key-complete",
					v1beta1constants.OperationRotateServiceAccountKeyComplete,
					nil,
					true,
					[]string{v1beta1constants.OperationRotateServiceAccountKeyComplete},
				),

				Entry("rotate-rollout-workers",
					v1beta1constants.OperationRotateRolloutWorkers+"=foo",
					nil,
					true,
					[]string{v1beta1constants.OperationRotateRolloutWorkers + "=foo"},
				),

				Entry("force-in-place-update",
					v1beta1constants.ShootOperationForceInPlaceUpdate,
					nil,
					false,
					[]string{v1beta1constants.ShootOperationForceInPlaceUpdate},
				),

				Entry("reconcile and rotate-etcd-encryption-key",
					fmt.Sprintf("%s;%s", v1beta1constants.GardenerOperationReconcile, v1beta1constants.OperationRotateETCDEncryptionKey),
					nil,
					true,
					[]string{v1beta1constants.OperationRotateETCDEncryptionKey},
				),

				Entry("remove operations covered by rotate-credentials-start",
					fmt.Sprintf("%s;%s;%s;%s;%s;%s;%s;%s", v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCAStart,
						v1beta1constants.OperationRotateServiceAccountKeyStart, v1beta1constants.OperationRotateETCDEncryptionKey, v1beta1constants.OperationRotateETCDEncryptionKeyStart,
						v1beta1constants.OperationRotateObservabilityCredentials, v1beta1constants.ShootOperationRotateSSHKeypair, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout),
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout},
				),

				Entry("remove operations covered by rotate-credentials-start-without-workers-rollout",
					fmt.Sprintf("%s;%s;%s;%s;%s;%s;%s;%s", v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
						v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout, v1beta1constants.OperationRotateETCDEncryptionKey, v1beta1constants.OperationRotateETCDEncryptionKeyStart,
						v1beta1constants.OperationRotateObservabilityCredentials, v1beta1constants.ShootOperationRotateSSHKeypair, v1beta1constants.OperationRotateCAStart),
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout, v1beta1constants.OperationRotateCAStart},
				),

				Entry("remove operations covered by rotate-credentials-complete",
					fmt.Sprintf("%s;%s;%s;%s;%s", v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.OperationRotateCAComplete, v1beta1constants.OperationRotateServiceAccountKeyComplete,
						v1beta1constants.OperationRotateETCDEncryptionKeyComplete, v1beta1constants.ShootOperationRotateSSHKeypair),
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.ShootOperationRotateSSHKeypair},
				),

				Entry("remove duplicate operations",
					fmt.Sprintf("%s;%s;%s;%s;%s", v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.ShootOperationRotateSSHKeypair,
						v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.ShootOperationRotateSSHKeypair),
					nil,
					true,
					[]string{v1beta1constants.OperationRotateCredentialsComplete, v1beta1constants.ShootOperationRotateSSHKeypair},
				),

				Entry("reconcile and rotate-ssh-keypair (ssh is not enabled)",
					fmt.Sprintf("%s;%s", v1beta1constants.GardenerOperationReconcile, v1beta1constants.ShootOperationRotateSSHKeypair),
					func(s *core.Shoot) { s.Spec.Provider.Workers = nil },
					true,
					nil,
				),
			)
		})

		Context("DNS Provider Credentials", func() {
			// TODO(vpnachev): Remove this context once support for Kubernetes 1.34 is dropped.
			It("should sync Secret credentialsRef to secretName and increase generation", func() {
				oldShoot := &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(Equal(ptr.To("secret-1")))
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			It("should not sync WorkloadIdentity credentialsRef to secretName and generation should stay the same", func() {
				oldShoot := &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "security.gardener.cloud/v1alpha1",
									Kind:       "WorkloadIdentity",
									Name:       "workload-identity-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(BeNil())
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})
		})
	})

	Describe("#Canonicalize", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{}
		})

		Context("seed names", func() {
			It("should correctly add the seed labels", func() {
				metav1.SetMetaDataLabel(&shoot.ObjectMeta, "foo", "bar")
				metav1.SetMetaDataLabel(&shoot.ObjectMeta, "name.seed.gardener.cloud/foo", "true")
				shoot.Spec.SeedName = ptr.To("spec-seed")
				shoot.Status.SeedName = ptr.To("status-seed")

				strategy.Canonicalize(shoot)

				Expect(shoot.Labels).To(Equal(map[string]string{
					"foo":                                  "bar",
					"name.seed.gardener.cloud/spec-seed":   "true",
					"name.seed.gardener.cloud/status-seed": "true",
				}))
			})
		})

		Context("maxEmptyBulkDelete", func() {
			It("should set spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete to nil", func() {
				shoot.Spec.Kubernetes.ClusterAutoscaler = &core.ClusterAutoscaler{MaxEmptyBulkDelete: ptr.To[int32](10)}
				strategy.Canonicalize(shoot)
				Expect(shoot.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete).To(BeNil())
			})
		})

		Context("self-hosted shoots", func() {
			It("should correctly add the self-hosted label", func() {
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, core.Worker{ControlPlane: &core.WorkerControlPlane{}})

				strategy.Canonicalize(shoot)

				Expect(shoot.Labels).To(Equal(map[string]string{
					"shoot.gardener.cloud/self-hosted": "true",
				}))
			})

			It("should correctly remove the self-hosted label", func() {
				metav1.SetMetaDataLabel(&shoot.ObjectMeta, "shoot.gardener.cloud/self-hosted", "true")

				strategy.Canonicalize(shoot)

				Expect(shoot.Labels).To(BeEmpty())
			})
		})
	})

	Context("BindingStrategy", func() {
		BeforeEach(func() {
			strategy = NewBindingStrategy()
		})

		Describe("#PrepareForUpdate", func() {
			var (
				oldShoot *core.Shoot
				newShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = &core.Shoot{}
				newShoot = &core.Shoot{}
			})

			It("should not allow editing the status", func() {
				newShoot.Status.TechnicalID = "foo"

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Status).To(Equal(oldShoot.Status))
			})

			Context("'create-pending' last operation", func() {
				BeforeEach(func() {
					oldShoot.Status.LastOperation = &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStatePending,
					}
					newShoot = oldShoot.DeepCopy()
				})

				It("should remove the last operation when seed was set", func() {
					newShoot.Spec.SeedName = ptr.To("foo")

					strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

					Expect(newShoot.Status.LastOperation).To(BeNil())
				})

				It("should not remove the last operation when seed was not set", func() {
					newShoot.Spec.Region = "foo"

					strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

					Expect(newShoot.Status.LastOperation).To(Equal(oldShoot.Status.LastOperation))
				})
			})

			It("should increase the generation when spec was changed", func() {
				newShoot.Spec.SeedName = ptr.To("foo")

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})
		})

		Context("DNS Provider Credentials", func() {
			// TODO(vpnachev): Remove this context once support for Kubernetes 1.34 is dropped.
			It("should sync Secret credentialsRef to secretName", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(Equal(ptr.To("secret-1")))
			})

			It("should not sync WorkloadIdentity credentialsRef to secretName", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "security.gardener.cloud/v1alpha1",
									Kind:       "WorkloadIdentity",
									Name:       "workload-identity-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(BeNil())
			})
		})
	})

	Context("StatusStrategy", func() {
		BeforeEach(func() {
			strategy = NewStatusStrategy()
		})

		Context("etcd encryption key rotation", func() {
			DescribeTable("etcd encryption key rotation",
				func(oldETCDEncryptionKeyRotation, newETCDEncryptionKeyRotation *core.ETCDEncryptionKeyRotation, shouldIncreaseGeneration bool) {
					oldShoot := &core.Shoot{
						Spec: core.ShootSpec{},
						Status: core.ShootStatus{
							Credentials: &core.ShootCredentials{
								Rotation: &core.ShootCredentialsRotation{
									ETCDEncryptionKey: oldETCDEncryptionKeyRotation,
								},
							},
						},
					}

					newShoot := oldShoot.DeepCopy()
					newShoot.Status.Credentials.Rotation.ETCDEncryptionKey = newETCDEncryptionKeyRotation

					strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

					expectedGeneration := oldShoot.Generation
					if shouldIncreaseGeneration {
						expectedGeneration++
					}
					Expect(newShoot.Generation).To(Equal(expectedGeneration))
				},

				Entry("rotation status is nil", nil, nil, false),
				Entry("rotation phase is empty", nil, &core.ETCDEncryptionKeyRotation{}, false),
				Entry("rotation phase is prepared", nil, &core.ETCDEncryptionKeyRotation{Phase: core.RotationPrepared, AutoCompleteAfterPrepared: ptr.To(true)}, true),
				Entry("rotation phase is prepared and is not single operation", nil, &core.ETCDEncryptionKeyRotation{Phase: core.RotationPrepared, AutoCompleteAfterPrepared: ptr.To(false)}, false),
				Entry("rotation phase has not been updated",
					&core.ETCDEncryptionKeyRotation{Phase: core.RotationPrepared, AutoCompleteAfterPrepared: ptr.To(true)},
					&core.ETCDEncryptionKeyRotation{Phase: core.RotationPrepared, AutoCompleteAfterPrepared: ptr.To(true)}, false),
				Entry("rotation phase is not prepared", nil, &core.ETCDEncryptionKeyRotation{Phase: core.RotationCompleting, AutoCompleteAfterPrepared: ptr.To(true)}, false),
			)
		})

		DescribeTable("should sync encrypted resources from status.encryptedResources to status.credentials.encryptionAtRest.resources",
			func(status core.ShootStatus, expected core.ShootStatus) {
				oldShoot := &core.Shoot{
					Spec:   core.ShootSpec{},
					Status: status,
				}
				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Status).To(Equal(expected))
			},
			Entry("no encrypted resources", core.ShootStatus{}, core.ShootStatus{}),
			Entry("with encrypted resources",
				core.ShootStatus{
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
				core.ShootStatus{
					Credentials: &core.ShootCredentials{
						EncryptionAtRest: &core.EncryptionAtRest{
							Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
						},
					},
					EncryptedResources: []string{"configmaps", "shoots.core.gardener.cloud"},
				},
			),
			Entry("should overwrite", core.ShootStatus{
				Credentials: &core.ShootCredentials{
					EncryptionAtRest: &core.EncryptionAtRest{
						Resources: []string{"configmaps", "shoots.core.gardener.cloud"},
					},
				},
				EncryptedResources: []string{"configmaps"},
			},
				core.ShootStatus{
					Credentials: &core.ShootCredentials{
						EncryptionAtRest: &core.EncryptionAtRest{
							Resources: []string{"configmaps"},
						},
					},
					EncryptedResources: []string{"configmaps"},
				},
			),
		)

		Context("DNS Provider Credentials", func() {
			// TODO(vpnachev): Remove this context once support for Kubernetes 1.34 is dropped.
			It("should sync Secret credentialsRef to secretName", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "secret-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(Equal(ptr.To("secret-1")))
			})

			It("should not sync WorkloadIdentity credentialsRef to secretName", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						DNS: &core.DNS{
							Providers: []core.DNSProvider{{
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "security.gardener.cloud/v1alpha1",
									Kind:       "WorkloadIdentity",
									Name:       "workload-identity-1",
								},
							}},
						},
					},
				}

				newShoot := oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)
				Expect(newShoot.Spec.DNS.Providers[0].SecretName).To(BeNil())
			})
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(createNewShootObject("foo"))

		Expect(result).To(HaveLen(7))
		Expect(result.Has(core.ShootSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootSeedName)).To(Equal("foo"))
		Expect(result.Has(core.ShootCloudProfileName)).To(BeTrue())
		Expect(result.Get(core.ShootCloudProfileName)).To(Equal("baz"))
		Expect(result.Has(core.ShootCloudProfileRefName)).To(BeTrue())
		Expect(result.Get(core.ShootCloudProfileRefName)).To(Equal("baz"))
		Expect(result.Has(core.ShootCloudProfileRefKind)).To(BeTrue())
		Expect(result.Get(core.ShootCloudProfileRefKind)).To(Equal("CloudProfile"))
		Expect(result.Has(core.ShootStatusSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootStatusSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Shoot", func() {
		_, _, err := GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := GetAttrs(createNewShootObject("foo"))

		Expect(err).NotTo(HaveOccurred())
		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.ShootSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := SeedNameTriggerFunc(createNewShootObject("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchShoot", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.ShootSeedName, "foo")

		result := MatchShoot(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.ShootSeedName))
	})
})

var _ = Describe("SyncDNSProviderCredentials", func() {
	// TODO(vpnachev): Remove entire test node once support for Kubernetes 1.34 is dropped.
	const (
		secretName1 string = "secret-1"
		secretName2 string = "secret-2"
	)

	var (
		shoot core.Shoot
	)

	BeforeEach(func() {
		shoot = core.Shoot{
			Spec: core.ShootSpec{
				DNS: nil,
			},
		}
	})

	It("should not modify shoot if DNS is nil", func() {
		originalShoot := shoot.DeepCopy()

		SyncDNSProviderCredentials(&shoot)

		Expect(shoot).To(Equal(*originalShoot))
	})

	It("should not modify shoot if DNS.providers is empty", func() {
		shoot.Spec.DNS = &core.DNS{
			Providers: []core.DNSProvider{},
		}
		originalShoot := shoot.DeepCopy()

		SyncDNSProviderCredentials(&shoot)

		Expect(shoot).To(Equal(*originalShoot))
	})

	DescribeTable("should sync secretName and credentialsRef when possible",
		func(providers, expectedProviders []core.DNSProvider) {
			shoot.Spec.DNS = &core.DNS{
				Providers: providers,
			}

			SyncDNSProviderCredentials(&shoot)

			Expect(shoot.Spec.DNS.Providers).To(Equal(expectedProviders))
		},
		Entry("single provider with secretName without credentialsRef",
			[]core.DNSProvider{
				{SecretName: ptr.To(secretName1)},
			},
			[]core.DNSProvider{
				{
					SecretName: ptr.To(secretName1),
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName1,
					},
				},
			},
		),
		Entry("multiple providers with secretName and without credentialsRef",
			[]core.DNSProvider{
				{SecretName: ptr.To(secretName1)},
				{SecretName: ptr.To(secretName2)},
			},
			[]core.DNSProvider{
				{
					SecretName: ptr.To(secretName1),
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName1,
					},
				},
				{
					SecretName: ptr.To(secretName2),
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName2,
					},
				},
			},
		),
		Entry("multiple providers, some with secretName and all without credentialsRef",
			[]core.DNSProvider{
				{SecretName: ptr.To(secretName1)},
				{
					Domains: &core.DNSIncludeExclude{
						Include: []string{"example.com"},
					},
				},
			},
			[]core.DNSProvider{
				{
					SecretName: ptr.To(secretName1),
					CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       secretName1,
					},
				},
				{
					Domains: &core.DNSIncludeExclude{
						Include: []string{"example.com"},
					},
				},
			},
		),
		Entry("secretName and credentialsRef are already set",
			[]core.DNSProvider{{
				SecretName: ptr.To(secretName1),
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName2,
				},
			}},
			[]core.DNSProvider{{
				SecretName: ptr.To(secretName1),
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName2,
				},
			}},
		),
		Entry("single provider with credentialsRef to secret without secretName",
			[]core.DNSProvider{{
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName2,
				},
			}},
			[]core.DNSProvider{{
				SecretName: ptr.To(secretName2),
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName2,
				},
			}},
		),
		Entry("single provider with credentialsRef to WorkloadIdentity without secretName",
			[]core.DNSProvider{{
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Name:       secretName1,
				},
			}},
			[]core.DNSProvider{{
				CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Name:       secretName1,
				},
			}},
		),
	)
})

func createNewShootObject(seedName string) *core.Shoot {
	return &core.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.ShootSpec{
			CloudProfileName: ptr.To("baz"),
			SeedName:         &seedName,
			CloudProfile: &core.CloudProfileReference{
				Kind: "CloudProfile",
				Name: "baz",
			},
		},
		Status: core.ShootStatus{
			SeedName: &seedName,
		},
	}
}
