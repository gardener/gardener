// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apiserver/registry/core/shoot"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
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

			It("should unset cloudProfileName field if NamespacedCloudProfile is referenced and feature gate is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
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

			It("should keep cloudProfileName field and overwrite the cloudprofile reference if NamespacedCloudProfile is referenced and feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				shoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForCreate(ctx, shoot)

				Expect(*shoot.Spec.CloudProfileName).To(Equal("foo"))
				Expect(shoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "foo",
				}))
			})

			It("should remove CredentialsBindingName field if ShootCredentialsBinding feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, false))

				shoot.Spec.CredentialsBindingName = ptr.To("binding")
				strategy.PrepareForCreate(ctx, shoot)

				Expect(shoot.Spec.CredentialsBindingName).To(BeNil())
			})

			It("should not remove CredentialsBindingName field if ShootCredentialsBinding feature gate is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, true))

				shoot.Spec.CredentialsBindingName = ptr.To("binding")
				strategy.PrepareForCreate(ctx, shoot)

				Expect(shoot.Spec.CredentialsBindingName).To(Equal(ptr.To("binding")))
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

			It("should unset cloudProfileName field if NamespacedCloudProfile is referenced and feature gate is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, true))
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

			It("should keep cloudProfileName field and overwrite the cloudprofile reference if NamespacedCloudProfile is referenced and feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				newShoot.Spec.CloudProfileName = ptr.To("foo")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(*newShoot.Spec.CloudProfileName).To(Equal("foo"))
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "foo",
				}))
			})

			It("should keep the NamespacedCloudProfile if it has been enabled before and now the feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.UseNamespacedCloudProfile, false))
				oldShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				newShoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.CloudProfileName).To(BeNil())
				Expect(newShoot.Spec.CloudProfile).To(Equal(&core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "bar",
				}))
			})

			It("should remove CredentialsBindingName field if ShootCredentialsBinding feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, false))

				newShoot.Spec.CredentialsBindingName = ptr.To("binding")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.CredentialsBindingName).To(BeNil())
			})

			It("should not remove CredentialsBindingName field if ShootCredentialsBinding feature gate is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, true))

				newShoot.Spec.CredentialsBindingName = ptr.To("binding")
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.CredentialsBindingName).To(Equal(ptr.To("binding")))
			})

			It("should not remove CredentialsBindingName field if ShootCredentialsBinding feature gate is disabled but the CredentialsBindingName field is present in the old Shoot", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootCredentialsBinding, false))

				bindingName := ptr.To("binding")
				oldShoot.Spec.CredentialsBindingName = bindingName
				newShoot.Spec.CredentialsBindingName = bindingName
				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec.CredentialsBindingName).To(Equal(ptr.To("binding")))
			})

			It("should not mutate shoots being deleted (cloud profile sync)", func() {
				oldShoot.Spec.CloudProfileName = ptr.To("profile")
				oldShoot.DeletionTimestamp = ptr.To(metav1.Now())
				newShoot = oldShoot.DeepCopy()

				strategy.PrepareForUpdate(ctx, newShoot, oldShoot)

				Expect(newShoot.Spec).To(Equal(oldShoot.Spec))
			})
		})

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
				func(operationAnnotation string, mutateOldShoot func(*core.Shoot), shouldIncreaseGeneration, shouldKeepAnnotation bool) {
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

					if shouldKeepAnnotation {
						Expect(newShoot.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, operationAnnotation))
					} else {
						Expect(newShoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
					}
				},

				Entry("retry; last operation is failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateFailed },
					true,
					false,
				),
				Entry("retry; last operation is not failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateSucceeded },
					false,
					true,
				),
				Entry("retry; last operation is not set",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation = nil },
					false,
					true,
				),
				Entry("reconcile",
					v1beta1constants.GardenerOperationReconcile,
					nil,
					true,
					false,
				),

				Entry("rotate-credentials-start",
					v1beta1constants.OperationRotateCredentialsStart,
					nil,
					true,
					true,
				),
				Entry("rotate-credentials-start-without-workers-rollout",
					v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
					nil,
					true,
					true,
				),
				Entry("rotate-credentials-complete",
					v1beta1constants.OperationRotateCredentialsComplete,
					nil,
					true,
					true,
				),

				Entry("rotate-ssh-keypair (ssh enabled)",
					v1beta1constants.ShootOperationRotateSSHKeypair,
					nil,
					true,
					true,
				),
				Entry("rotate-ssh-keypair (ssh is not enabled)",
					v1beta1constants.ShootOperationRotateSSHKeypair,
					func(s *core.Shoot) { s.Spec.Provider.Workers = nil },
					false,
					false,
				),
				Entry("rotate-observability-credentials",
					v1beta1constants.OperationRotateObservabilityCredentials,
					nil,
					true,
					true,
				),

				Entry("rotate-etcd-encryption-key-start",
					v1beta1constants.OperationRotateETCDEncryptionKeyStart,
					nil,
					true,
					true,
				),
				Entry("rotate-etcd-encryption-key-complete",
					v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
					nil,
					true,
					true,
				),

				Entry("rotate-ca-start",
					v1beta1constants.OperationRotateCAStart,
					nil,
					true,
					true,
				),
				Entry("rotate-ca-start-without-workers-rollout",
					v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
					nil,
					true,
					true,
				),
				Entry("rotate-ca-complete",
					v1beta1constants.OperationRotateCAComplete,
					nil,
					true,
					true,
				),

				Entry("rotate-serviceaccount-key-start",
					v1beta1constants.OperationRotateServiceAccountKeyStart,
					nil,
					true,
					true,
				),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout",
					v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout,
					nil,
					true,
					true,
				),
				Entry("rotate-serviceaccount-key-complete",
					v1beta1constants.OperationRotateServiceAccountKeyComplete,
					nil,
					true,
					true,
				),

				Entry("rotate-rollout-workers",
					v1beta1constants.OperationRotateRolloutWorkers+"=foo",
					nil,
					true,
					true,
				),

				Entry("force-in-place-update",
					v1beta1constants.ShootOperationForceInPlaceUpdate,
					nil,
					false,
					true,
				),
			)
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

		Context("enableStaticTokenKubeconfig", func() {
			It("should set spec.kubernetes.enableStaticTokenKubeconfig to nil", func() {
				shoot := &core.Shoot{}
				shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)

				strategy.Canonicalize(shoot)
				Expect(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(BeNil())

				shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(false)

				strategy.Canonicalize(shoot)
				Expect(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig).To(BeNil())
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
