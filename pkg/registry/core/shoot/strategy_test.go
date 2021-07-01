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

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	shootregistry "github.com/gardener/gardener/pkg/registry/core/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

var _ = Describe("Strategy", func() {
	Context("PrepareForUpdate", func() {
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

			It("should increase when the last operation is failed and the retry annotation gets set", func() {
				oldShoot := &core.Shoot{
					Status: core.ShootStatus{
						LastOperation: &core.LastOperation{
							State: core.LastOperationStateFailed,
						},
					},
				}
				newShoot := oldShoot.DeepCopy()
				newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.ShootOperationRetry}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			It("should not increase when the last operation is not failed and the retry annotation gets set", func() {
				oldShoot := &core.Shoot{
					Status: core.ShootStatus{
						LastOperation: &core.LastOperation{
							State: core.LastOperationStateSucceeded,
						},
					},
				}
				newShoot := oldShoot.DeepCopy()
				newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.ShootOperationRetry}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			It("should increase when the reconcile annotation gets set but no last operation", func() {
				oldShoot := &core.Shoot{}
				newShoot := oldShoot.DeepCopy()
				newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation))
			})

			It("should increase when the reconcile annotation gets set with a last operation", func() {
				oldShoot := &core.Shoot{
					Status: core.ShootStatus{
						LastOperation: &core.LastOperation{},
					},
				}
				newShoot := oldShoot.DeepCopy()
				newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
			})

			It("should increase when the rotate-kubeconfig-credentials annotation gets set with a last operation", func() {
				oldShoot := &core.Shoot{
					Status: core.ShootStatus{
						LastOperation: &core.LastOperation{},
					},
				}
				newShoot := oldShoot.DeepCopy()
				newShoot.Annotations = map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.ShootOperationRotateKubeconfigCredentials}

				shootregistry.Strategy.PrepareForUpdate(context.TODO(), newShoot, oldShoot)
				Expect(newShoot.Generation).To(Equal(oldShoot.Generation + 1))
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
