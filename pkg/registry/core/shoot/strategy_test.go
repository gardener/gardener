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
			It("should not increase if new=old", func() {
				oldShoot := &core.Shoot{}
				newShoot := oldShoot.DeepCopy()

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			It("should not increase if spec remains the same", func() {
				oldShoot := &core.Shoot{}
				newShoot := oldShoot.DeepCopy()
				newShoot.Labels = map[string]string{"foo": "bar"}
				newShoot.Spec = oldShoot.Spec

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			It("should increase for spec changes only if confineSpecUpdateRollout is false", func() {
				oldShoot := &core.Shoot{}
				newShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Region: "foo",
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(false),
						},
					},
				}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			It("should not increase for spec changes if confineSpecUpdateRollout is true", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(true),
						},
					},
				}
				newShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Region: "foo",
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(true),
						},
					},
				}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			Context("exceptional case: spec.hibernation.enabled changes even if confineSpecUpdateRollout is true", func() {
				var (
					oldShoot *core.Shoot
					newShoot *core.Shoot
				)

				BeforeEach(func() {
					oldShoot = &core.Shoot{
						Spec: core.ShootSpec{
							Maintenance: &core.Maintenance{
								ConfineSpecUpdateRollout: pointer.Bool(true),
							},
						},
					}
					newShoot = oldShoot.DeepCopy()
				})

				It("old hibernation=nil, new hibernation=nil", func() {
					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})

				It("old hibernation=nil, new hibernation.enabled=false", func() {
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})

				It("old hibernation.enabled=nil, new hibernation.enabled=false", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{}
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})

				It("old hibernation=nil, new hibernation.enabled=true", func() {
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=nil, new hibernation.enabled=true", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{}
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=true, new hibernation.enabled=false", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=true, new hibernation.enabled=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}
					newShoot.Spec.Hibernation = &core.Hibernation{}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=true, new hibernation=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}
					newShoot.Spec.Hibernation = nil

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=true, new hibernation.enabled=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}
					newShoot.Spec.Hibernation = &core.Hibernation{}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=false, new hibernation.enabled=true", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}
					newShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(true),
					}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
				})

				It("old hibernation.enabled=false, new hibernation.enabled=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}
					newShoot.Spec.Hibernation = &core.Hibernation{}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})

				It("old hibernation.enabled=false, new hibernation=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{
						Enabled: pointer.Bool(false),
					}
					newShoot.Spec.Hibernation = nil

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})

				It("old hibernation.enabled=nil, new hibernation=nil", func() {
					oldShoot.Spec.Hibernation = &core.Hibernation{}
					newShoot.Spec.Hibernation = nil

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
					Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
				})
			})

			It("should increase for confineSpecUpdateRollout changes from true -> false", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(true),
						},
					},
				}
				newShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(false),
						},
					},
				}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			It("should not increase for confineSpecUpdateRollout changes from false -> true", func() {
				oldShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(false),
						},
					},
				}
				newShoot := &core.Shoot{
					Spec: core.ShootSpec{
						Maintenance: &core.Maintenance{
							ConfineSpecUpdateRollout: pointer.Bool(true),
						},
					},
				}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			It("should increase when the deletion timestamp gets set", func() {
				deletionTimestamp := metav1.Now()

				oldShoot := &core.Shoot{}
				newShoot := &core.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &deletionTimestamp,
					},
				}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			DescribeTable("operation annotations",
				func(operationAnnotation string, mustIncreaseGeneration bool, mutateOldShoot func(*core.Shoot), featureGates map[featuregate.Feature]bool) {
					oldShoot := &core.Shoot{
						Status: core.ShootStatus{
							LastOperation: &core.LastOperation{},
						},
					}

					if mutateOldShoot != nil {
						mutateOldShoot(oldShoot)
					}

					var deferFns []interface{}
					for name, enabled := range featureGates {
						deferFns = append(deferFns, test.WithFeatureGate(utilfeature.DefaultFeatureGate, name, enabled))
					}
					for _, fn := range deferFns {
						DeferCleanup(fn)
					}

					newShoot := oldShoot.DeepCopy()
					newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: operationAnnotation}

					shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)

					if mustIncreaseGeneration {
						Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
					} else {
						Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
					}
				},

				Entry("retry; last operation is failed", v1beta1constants.ShootOperationRetry, true, func(oldShoot *core.Shoot) {
					oldShoot.Status.LastOperation.State = core.LastOperationStateFailed
				}, nil),
				Entry("retry; last operation is not failed", v1beta1constants.ShootOperationRetry, false, func(oldShoot *core.Shoot) {
					oldShoot.Status.LastOperation.State = core.LastOperationStateSucceeded
				}, nil),
				Entry("retry; last operation is not set", v1beta1constants.ShootOperationRetry, false, func(oldShoot *core.Shoot) {
					oldShoot.Status.LastOperation = nil
				}, nil),

				Entry("reconcile", v1beta1constants.GardenerOperationReconcile, true, nil, nil),

				Entry("rotate-kubeconfig-credentials", v1beta1constants.ShootOperationRotateKubeconfigCredentials, true, nil, nil),
				Entry("rotate-ssh-keypair", v1beta1constants.ShootOperationRotateSSHKeypair, true, nil, nil),
				Entry("rotate-observability-credentials", v1beta1constants.ShootOperationRotateObservabilityCredentials, true, nil, nil),
				Entry("rotate-etcd-encryption-key-start", v1beta1constants.ShootOperationRotateETCDEncryptionKeyStart, true, nil, nil),
				Entry("rotate-etcd-encryption-key-complete", v1beta1constants.ShootOperationRotateETCDEncryptionKeyComplete, true, nil, nil),

				Entry("rotate-ca-start; feature gate is enabled", v1beta1constants.ShootOperationRotateCAStart, true, nil, map[featuregate.Feature]bool{features.ShootCARotation: true}),
				Entry("rotate-ca-complete; feature gate is enabled", v1beta1constants.ShootOperationRotateCAComplete, true, nil, map[featuregate.Feature]bool{features.ShootCARotation: true}),
				Entry("rotate-ca-start; feature gate is disabled", v1beta1constants.ShootOperationRotateCAStart, false, nil, map[featuregate.Feature]bool{features.ShootCARotation: false}),
				Entry("rotate-ca-complete; feature gate is disabled", v1beta1constants.ShootOperationRotateCAComplete, false, nil, map[featuregate.Feature]bool{features.ShootCARotation: false}),

				Entry("rotate-serviceaccount-key-start; feature gate is enabled", v1beta1constants.ShootOperationRotateServiceAccountKeyStart, true, nil, map[featuregate.Feature]bool{features.ShootSARotation: true}),
				Entry("rotate-serviceaccount-key-complete; feature gate is enabled", v1beta1constants.ShootOperationRotateServiceAccountKeyComplete, true, nil, map[featuregate.Feature]bool{features.ShootSARotation: true}),
				Entry("rotate-serviceaccount-key-start; feature gate is disabled", v1beta1constants.ShootOperationRotateServiceAccountKeyStart, false, nil, map[featuregate.Feature]bool{features.ShootSARotation: false}),
				Entry("rotate-serviceaccount-key-complete; feature gate is disabled", v1beta1constants.ShootOperationRotateServiceAccountKeyComplete, false, nil, map[featuregate.Feature]bool{features.ShootSARotation: false}),
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
