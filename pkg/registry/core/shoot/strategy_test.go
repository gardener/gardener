// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	shootregistry "github.com/gardener/gardener/pkg/registry/core/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
)

var _ = Describe("Strategy", func() {
	Describe("#PrepareForCreate", func() {
		Context("max token expiration", func() {
			newShoot := func(duration time.Duration, withDeletionTimestamp bool) *core.Shoot {
				shoot := &core.Shoot{
					Spec: core.ShootSpec{
						Kubernetes: core.Kubernetes{
							KubeAPIServer: &core.KubeAPIServerConfig{
								ServiceAccountConfig: &core.ServiceAccountConfig{
									MaxTokenExpiration: &metav1.Duration{Duration: duration},
								},
							},
						},
					},
				}

				if withDeletionTimestamp {
					shoot.DeletionTimestamp = &metav1.Time{}
				}

				return shoot
			}

			DescribeTable("ShootMaxTokenExpirationOverwrite feature gate",
				func(featureGateEnabled bool, maxTokenExpiration, expectedDuration time.Duration, shootHasDeletionTimestamp bool) {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.ShootMaxTokenExpirationOverwrite, featureGateEnabled)()

					shoot := newShoot(maxTokenExpiration, shootHasDeletionTimestamp)
					shootregistry.Strategy.PrepareForCreate(context.TODO(), shoot)
					Expect(shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.MaxTokenExpiration.Duration).To(Equal(expectedDuration))
				},

				Entry("feature gate enabled, too low value", true, time.Hour, 720*time.Hour, false),
				Entry("feature gate enabled, too high value", true, 3000*time.Hour, 2160*time.Hour, false),
				Entry("feature gate enabled, value within boundaries", true, 1000*time.Hour, 1000*time.Hour, false),
				Entry("feature gate enabled, value out of boundaries, shoot w/ deletionTimestamp", true, 5000*time.Hour, 5000*time.Hour, true),
			)
		})
	})

	Describe("#PrepareForUpdate", func() {
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

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)

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

						shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)

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
				func(operationAnnotation string, mutateOldShoot func(*core.Shoot), featureGates map[featuregate.Feature]bool, shouldIncreaseGeneration bool) {
					oldShoot := &core.Shoot{
						Status: core.ShootStatus{
							LastOperation: &core.LastOperation{},
						},
					}

					if mutateOldShoot != nil {
						mutateOldShoot(oldShoot)
					}

					for name, enabled := range featureGates {
						DeferCleanup(test.WithFeatureGate(utilfeature.DefaultFeatureGate, name, enabled))
					}

					newShoot := oldShoot.DeepCopy()
					newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: operationAnnotation}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)

					expectedGeneration := oldShoot.Generation
					if shouldIncreaseGeneration {
						expectedGeneration++
					}

					Expect(newShoot.Generation).To(Equal(expectedGeneration))
				},

				Entry("retry; last operation is failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateFailed },
					nil,
					true,
				),
				Entry("retry; last operation is not failed",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation.State = core.LastOperationStateSucceeded },
					nil,
					false,
				),
				Entry("retry; last operation is not set",
					v1beta1constants.ShootOperationRetry,
					func(s *core.Shoot) { s.Status.LastOperation = nil },
					nil,
					false,
				),
				Entry("reconcile",
					v1beta1constants.GardenerOperationReconcile,
					nil,
					nil,
					true,
				),

				Entry("rotate-kubeconfig-credentials",
					v1beta1constants.ShootOperationRotateKubeconfigCredentials,
					nil,
					nil,
					true,
				),
				Entry("rotate-ssh-keypair",
					v1beta1constants.ShootOperationRotateSSHKeypair,
					nil,
					nil,
					true,
				),
				Entry("rotate-observability-credentials",
					v1beta1constants.ShootOperationRotateObservabilityCredentials,
					nil,
					nil,
					true,
				),

				Entry("rotate-etcd-encryption-key-start",
					v1beta1constants.ShootOperationRotateETCDEncryptionKeyStart,
					nil,
					nil,
					true,
				),
				Entry("rotate-etcd-encryption-key-complete",
					v1beta1constants.ShootOperationRotateETCDEncryptionKeyComplete,
					nil,
					nil,
					true,
				),

				Entry("rotate-ca-start; feature gate is enabled",
					v1beta1constants.ShootOperationRotateCAStart,
					nil,
					map[featuregate.Feature]bool{features.ShootCARotation: true},
					true,
				),
				Entry("rotate-ca-complete; feature gate is enabled",
					v1beta1constants.ShootOperationRotateCAComplete,
					nil,
					map[featuregate.Feature]bool{features.ShootCARotation: true},
					true,
				),
				Entry("rotate-ca-start; feature gate is disabled",
					v1beta1constants.ShootOperationRotateCAStart,
					nil,
					map[featuregate.Feature]bool{features.ShootCARotation: false},
					false,
				),
				Entry("rotate-ca-complete; feature gate is disabled",
					v1beta1constants.ShootOperationRotateCAComplete,
					nil,
					map[featuregate.Feature]bool{features.ShootCARotation: false},
					false,
				),

				Entry("rotate-serviceaccount-key-start; feature gate is enabled",
					v1beta1constants.ShootOperationRotateServiceAccountKeyStart,
					nil,
					map[featuregate.Feature]bool{features.ShootSARotation: true},
					true,
				),
				Entry("rotate-serviceaccount-key-complete; feature gate is enabled",
					v1beta1constants.ShootOperationRotateServiceAccountKeyComplete,
					nil,
					map[featuregate.Feature]bool{features.ShootSARotation: true},
					true,
				),
				Entry("rotate-serviceaccount-key-start; feature gate is disabled",
					v1beta1constants.ShootOperationRotateServiceAccountKeyStart,
					nil,
					map[featuregate.Feature]bool{features.ShootSARotation: false},
					false,
				),
				Entry("rotate-serviceaccount-key-complete; feature gate is disabled",
					v1beta1constants.ShootOperationRotateServiceAccountKeyComplete,
					nil,
					map[featuregate.Feature]bool{features.ShootSARotation: false},
					false,
				),
			)
		})

		Context("max token expiration", func() {
			newShoot := func(duration time.Duration, withDeletionTimestamp bool) *core.Shoot {
				shoot := &core.Shoot{
					Spec: core.ShootSpec{
						Kubernetes: core.Kubernetes{
							KubeAPIServer: &core.KubeAPIServerConfig{
								ServiceAccountConfig: &core.ServiceAccountConfig{
									MaxTokenExpiration: &metav1.Duration{Duration: duration},
								},
							},
						},
					},
				}

				if withDeletionTimestamp {
					shoot.DeletionTimestamp = &metav1.Time{}
				}

				return shoot
			}

			DescribeTable("ShootMaxTokenExpirationOverwrite feature gate",
				func(featureGateEnabled bool, maxTokenExpiration, expectedDuration time.Duration, shootHasDeletionTimestamp bool) {
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.ShootMaxTokenExpirationOverwrite, featureGateEnabled)()

					shoot := newShoot(maxTokenExpiration, shootHasDeletionTimestamp)
					oldShoot := shoot.DeepCopy()

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), shoot, oldShoot)
					Expect(shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig.MaxTokenExpiration.Duration).To(Equal(expectedDuration))
				},

				Entry("feature gate enabled, too low value", true, time.Hour, 720*time.Hour, false),
				Entry("feature gate enabled, too high value", true, 3000*time.Hour, 2160*time.Hour, false),
				Entry("feature gate enabled, value within boundaries", true, 1000*time.Hour, 1000*time.Hour, false),
				Entry("feature gate enabled, value out of boundaries, shoot w/ deletionTimestamp", true, 5000*time.Hour, 5000*time.Hour, true),
			)
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
