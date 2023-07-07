// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	shootregistry "github.com/gardener/gardener/pkg/registry/core/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Strategy", func() {
	Describe("#Validate", func() {
		var (
			shoot *core.Shoot
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: core.ShootSpec{
					CloudProfileName: "aws-profile",
					Region:           "eu-west-1",
					Kubernetes: core.Kubernetes{
						Version: "1.25.2",
					},
					Provider: core.Provider{
						Type:    "provider",
						Workers: []core.Worker{},
					},
				},
			}
		})

		It("should forbid an empty worker list if WorkerlessShoots featuregate is disabled", func() {
			errorList := shootregistry.NewStrategy(0).Validate(context.TODO(), shoot)

			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.provider.workers"),
				"Detail": ContainSubstring("must provide at least one worker pool when WorkerlessShoots feature gate is disabled"),
			}))))
		})

		It("should allow an empty worker list if WorkerlessShoots featuregate is enabled", func() {
			DeferCleanup(test.WithFeatureGate(utilfeature.DefaultMutableFeatureGate, features.WorkerlessShoots, true))

			errorList := shootregistry.NewStrategy(0).Validate(context.TODO(), shoot)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#PrepareForUpdate", func() {
		Context("seedName change", func() {
			var (
				oldShoot *core.Shoot
				newShoot *core.Shoot
			)

			BeforeEach(func() {
				oldShoot = &core.Shoot{
					Spec: core.ShootSpec{
						SeedName: pointer.String("seed"),
					},
				}
				newShoot = oldShoot.DeepCopy()
			})

			It("should not allow change of seedName on shoot spec update", func() {
				newShoot.Spec.SeedName = pointer.String("new-seed")
				shootregistry.NewStrategy(0).PrepareForUpdate(context.TODO(), newShoot, oldShoot)

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

					shootregistry.NewStrategy(0).PrepareForUpdate(context.TODO(), newShoot, oldShoot)

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

						shootregistry.NewStrategy(0).PrepareForUpdate(context.TODO(), newShoot, oldShoot)

						expectedGeneration := oldShoot.Generation
						if shouldIncreaseGeneration {
							expectedGeneration++
						}

						Expect(newShoot.Generation).To(Equal(expectedGeneration))
					},

					Entry("confineSpecUpdateRollout true->false",
						pointer.Bool(true), pointer.Bool(false),
						nil, nil,
						true,
					),
					Entry("confineSpecUpdateRollout false->true",
						pointer.Bool(false), pointer.Bool(true),
						nil, nil,
						false,
					),
					Entry("confineSpecUpdateRollout nil->false w/ additional spec change",
						nil, pointer.Bool(false),
						nil, func(s *core.Shoot) { s.Spec.Region = "foo" },
						true,
					),
					Entry("confineSpecUpdateRollout true->true w/ additional spec change",
						pointer.Bool(true), pointer.Bool(true),
						nil, func(s *core.Shoot) { s.Spec.Region = "foo" },
						false,
					),

					// exceptional cases: spec.hibernation.enabled changes even if confineSpecUpdateRollout is true
					Entry("hibernation nil -> nil",
						pointer.Bool(true), pointer.Bool(true),
						nil, nil,
						false,
					),
					Entry("hibernation nil -> false",
						pointer.Bool(true), pointer.Bool(true),
						nil, func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						false,
					),
					Entry("hibernation nil -> true",
						pointer.Bool(true), pointer.Bool(true),
						nil, func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						true,
					),

					Entry("hibernation enabled nil -> false",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						false,
					),
					Entry("hibernation enabled nil -> true",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						true,
					),
					Entry("hibernation enabled nil -> hibernation nil",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						nil,
						false,
					),

					Entry("hibernation enabled true -> true",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						false,
					),
					Entry("hibernation enabled true -> false",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						true,
					),
					Entry("hibernation enabled true -> nil",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						true,
					),
					Entry("hibernation enabled true -> hibernation nil",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						nil,
						true,
					),

					Entry("hibernation enabled false -> true",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(true)} },
						true,
					),
					Entry("hibernation enabled false -> false",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						false,
					),
					Entry("hibernation enabled false -> nil",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{} },
						false,
					),
					Entry("hibernation enabled false -> hibernation nil",
						pointer.Bool(true), pointer.Bool(true),
						func(s *core.Shoot) { s.Spec.Hibernation = &core.Hibernation{Enabled: pointer.Bool(false)} },
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

					shootregistry.NewStrategy(0).PrepareForUpdate(context.TODO(), newShoot, oldShoot)

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
				Entry("rotate-credentials-complete",
					v1beta1constants.OperationRotateCredentialsComplete,
					nil,
					true,
					true,
				),

				Entry("rotate-kubeconfig-credentials",
					v1beta1constants.ShootOperationRotateKubeconfigCredentials,
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
					v1beta1constants.ShootOperationRotateObservabilityCredentials,
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
				Entry("rotate-serviceaccount-key-complete",
					v1beta1constants.OperationRotateServiceAccountKeyComplete,
					nil,
					true,
					true,
				),
			)
		})
	})

	Describe("#Canonicalize", func() {
		var shoot *core.Shoot

		BeforeEach(func() {
			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					Kubernetes: core.Kubernetes{
						Version: "1.24.0",
						KubeAPIServer: &core.KubeAPIServerConfig{
							AdmissionPlugins: []core.AdmissionPlugin{
								{
									Name:   "NodeRestriction",
									Config: &runtime.RawExtension{Raw: []byte("bar")},
								},
								{
									Name:     "PodSecurityPolicy",
									Disabled: pointer.Bool(true),
								},
								{
									Name:   "PodSecurity",
									Config: &runtime.RawExtension{Raw: []byte("foo")},
								},
							},
						},
					},
				},
			}
		})

		Context("PluginsInMigration", func() {

			Context("k8s version >=1.25", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.Version = "1.25.0"
				})

				It("should cleanup PodSecurityPolicy from the admission plugins list", func() {
					shootregistry.NewStrategy(0).Canonicalize(shoot)

					Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(
						core.AdmissionPlugin{
							Name:   "NodeRestriction",
							Config: &runtime.RawExtension{Raw: []byte("bar")},
						},
						core.AdmissionPlugin{
							Name:   "PodSecurity",
							Config: &runtime.RawExtension{Raw: []byte("foo")},
						},
					))
				})
			})

			Context("k8s version < 1.25", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.Version = "1.24.0"
				})

				It("should not cleanup PodSecurityPolicy from the admission plugins list", func() {
					shootregistry.NewStrategy(0).Canonicalize(shoot)

					Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(
						core.AdmissionPlugin{
							Name:   "NodeRestriction",
							Config: &runtime.RawExtension{Raw: []byte("bar")},
						},
						core.AdmissionPlugin{
							Name:   "PodSecurity",
							Config: &runtime.RawExtension{Raw: []byte("foo")},
						},
						core.AdmissionPlugin{
							Name:     "PodSecurityPolicy",
							Disabled: pointer.Bool(true),
						},
					))
				})
			})
		})
	})
})

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := shootregistry.ToSelectableFields(newShoot("foo"))

		Expect(result).To(HaveLen(5))
		Expect(result.Has(core.ShootSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootSeedName)).To(Equal("foo"))
		Expect(result.Has(core.ShootCloudProfileName)).To(BeTrue())
		Expect(result.Get(core.ShootCloudProfileName)).To(Equal("baz"))
		Expect(result.Has(core.ShootStatusSeedName)).To(BeTrue())
		Expect(result.Get(core.ShootStatusSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Shoot", func() {
		_, _, err := shootregistry.GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := shootregistry.GetAttrs(newShoot("foo"))

		Expect(err).NotTo(HaveOccurred())
		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.ShootSeedName)).To(Equal("foo"))
	})
})

var _ = Describe("SeedNameTriggerFunc", func() {
	It("should return spec.seedName", func() {
		actual := shootregistry.SeedNameTriggerFunc(newShoot("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchShoot", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.ShootSeedName, "foo")

		result := shootregistry.MatchShoot(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.ShootSeedName))
	})
})

func newShoot(seedName string) *core.Shoot {
	return &core.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec: core.ShootSpec{
			CloudProfileName: "baz",
			SeedName:         &seedName,
		},
		Status: core.ShootStatus{
			SeedName: &seedName,
		},
	}
}
