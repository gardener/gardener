// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	seedctrl "github.com/gardener/gardener/pkg/gardenlet/controller/seed"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

var _ = Describe("ExtensionControlReconcile", func() {
	const seedName = "test"

	var (
		fakeClient         *fake.Clientset
		indexer            cache.Indexer
		seed               *gardencorev1beta1.Seed
		lister             gardencorelisters.ControllerInstallationLister
		defaultTime        metav1.Time
		defaultTimeFunc    func() metav1.Time
		controller         seedctrl.ExtensionCheckControlInterface
		updatedSeed        *gardencorev1beta1.Seed
		expectedUpdateCall bool
	)

	BeforeEach(func() {
		// This should not be here!!! Hidden dependency!!!
		logger.Logger = logger.NewNopLogger()

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
		}

		indexer = cache.NewIndexer(
			cache.MetaNamespaceKeyFunc,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

		lister = gardencorelisters.NewControllerInstallationLister(indexer)

		defaultTime = metav1.NewTime(time.Unix(2, 2))
		defaultTimeFunc = func() metav1.Time {
			return defaultTime
		}

		updatedSeed = nil
		fakeClient = fake.NewSimpleClientset()
		fakeClient.PrependReactor("update", "seeds", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() != "status" {
				return true, nil, fmt.Errorf("expected a update on the seeds/status, got %v instead", action)
			}

			updatedSeed = action.(testing.UpdateAction).GetObject().(*gardencorev1beta1.Seed)

			return true, updatedSeed, nil
		})
	})

	JustBeforeEach(func() {
		fakeClientSet := fakeclientset.NewClientSetBuilder().WithGardenCore(fakeClient).Build()
		fakeClientMap := fakeclientmap.NewClientMap().AddClient(keys.ForGarden(), fakeClientSet)
		controller = seedctrl.NewDefaultExtensionCheckControl(fakeClientMap, lister, defaultTimeFunc)
	})

	AfterEach(func() {
		if expectedUpdateCall {
			Expect(fakeClient.Actions()).To(HaveLen(1), "one status update should be executed")
		} else {
			Expect(fakeClient.Actions()).To(BeEmpty(), "no status update should be executed")
		}
	})

	Context("listing of controller installations fails", func() {
		BeforeEach(func() {
			lister = &failingControllerInstallationLister{}
		})

		It("should return error", func() {
			Expect(controller.ReconcileExtensionCheckFor(seed)).To(HaveOccurred())
		})
	})

	Context("update needed", func() {
		var (
			current, expected gardencorev1beta1.Condition
			assertConditions  = func() {
				DescribeTable("condition exist",
					// seed and expected conditions are wrapped in functions, so they are not evaluated
					// at compile time.
					func(mutateFunc func()) {
						mutateFunc()
						seed.Status.Conditions = []gardencorev1beta1.Condition{current}

						result := controller.ReconcileExtensionCheckFor(seed)

						Expect(result).ToNot(HaveOccurred(), "update should succeed")
						Expect(updatedSeed.Status.Conditions).To(ConsistOf(expected))
					},

					Entry("message is different", func() {
						current.Message = "Some message"
						expected.LastUpdateTime = defaultTime
					}),
					Entry("reason is different", func() {
						current.Reason = "Some reason"
						expected.LastUpdateTime = defaultTime
					}),
					Entry("message and reason is different", func() {
						current.Message = "Some message"
						current.Reason = "Some reason"
						expected.LastUpdateTime = defaultTime
					}),
					Entry("status is different", func() {
						current.Status = gardencorev1beta1.ConditionProgressing
						expected.LastTransitionTime = defaultTime
					}),
				)
				It("condition does not exists", func() {
					expected.LastTransitionTime = defaultTime
					expected.LastUpdateTime = defaultTime
					result := controller.ReconcileExtensionCheckFor(seed)

					Expect(result).ToNot(HaveOccurred(), "update should succeed")
					Expect(updatedSeed.Status.Conditions).To(ConsistOf(expected))
				})
			}
		)

		BeforeEach(func() {
			expectedUpdateCall = true
		})

		Context("AllExtensionsReady", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "AllExtensionsReady",
					Message:            "All extensions installed into the seed cluster are ready and healthy.",
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}
				expected = *current.DeepCopy()
			})

			assertConditions()
		})

		Context("NotAllExtensionsInstalled", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionFalse,
					Reason:             "NotAllExtensionsInstalled",
					Message:            `Some extensions are not installed: +map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`,
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}
				expected = *current.DeepCopy()

				c1 := &gardencorev1beta1.ControllerInstallation{}
				c1.SetName("foo-1")
				c1.Spec.SeedRef.Name = seedName

				c2 := c1.DeepCopy()
				c2.SetName("foo-2")
				c2.Spec.SeedRef.Name = "not-seed-2"

				c3 := c1.DeepCopy()
				c3.SetName("foo-3")

				Expect(indexer.Add(c1)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c2)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c3)).NotTo(HaveOccurred(), "adding to the index should succeed")
			})

			assertConditions()
		})

		Context("NotAllExtensionsInstalled", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionFalse,
					Reason:             "NotAllExtensionsInstalled",
					Message:            `Some extensions are not installed: +map[foo-2:]`,
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}
				expected = *current.DeepCopy()

				c1 := &gardencorev1beta1.ControllerInstallation{}
				c1.SetName("foo-1")
				c1.Spec.SeedRef.Name = seedName
				c1.Status.Conditions = []gardencorev1beta1.Condition{
					// string literal to ensure that the tests fails if the constant is changed
					{
						Type:   "Valid",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "Installed",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "Healthy",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "RandomType",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "AnotherRandomType",
						Status: gardencorev1beta1.ConditionFalse,
					},
				}

				c2 := c1.DeepCopy()
				c2.SetName("foo-2")
				c2.Status.Conditions[1].Status = gardencorev1beta1.ConditionFalse

				Expect(indexer.Add(c1)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c2)).NotTo(HaveOccurred(), "adding to the index should succeed")

			})

			assertConditions()
		})
	})

	Context("update not needed", func() {
		var (
			current gardencorev1beta1.Condition
		)

		BeforeEach(func() {
			expectedUpdateCall = false
		})

		Context("AllExtensionsReady", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "AllExtensionsReady",
					Message:            "All extensions installed into the seed cluster are ready and healthy.",
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}
			})

			// assertConditions()

			It("should do nothing", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{current}

				result := controller.ReconcileExtensionCheckFor(seed)

				Expect(result).To(BeNil(), "no error should be returned")
			})
		})

		Context("NotAllExtensionsInstalled", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionFalse,
					Reason:             "NotAllExtensionsInstalled",
					Message:            `Some extensions are not installed: +map[foo-1:extension was not yet installed foo-3:extension was not yet installed]`,
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}

				c1 := &gardencorev1beta1.ControllerInstallation{}
				c1.SetName("foo-1")
				c1.Spec.SeedRef.Name = seedName

				c2 := c1.DeepCopy()
				c2.SetName("foo-2")
				c2.Spec.SeedRef.Name = "not-seed-2"

				c3 := c1.DeepCopy()
				c3.SetName("foo-3")

				Expect(indexer.Add(c1)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c2)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c3)).NotTo(HaveOccurred(), "adding to the index should succeed")
			})

			// assertConditions()

			It("should do nothing", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{current}

				result := controller.ReconcileExtensionCheckFor(seed)

				Expect(result).To(BeNil(), "no error should be returned")
			})
		})

		Context("NotAllExtensionsInstalled", func() {
			BeforeEach(func() {
				current = gardencorev1beta1.Condition{
					Type:               "ExtensionsReady",
					Status:             gardencorev1beta1.ConditionFalse,
					Reason:             "NotAllExtensionsInstalled",
					Message:            `Some extensions are not installed: +map[foo-2:]`,
					LastTransitionTime: metav1.NewTime(time.Unix(1, 1)),
					LastUpdateTime:     metav1.NewTime(time.Unix(1, 1)),
				}

				c1 := &gardencorev1beta1.ControllerInstallation{}
				c1.SetName("foo-1")
				c1.Spec.SeedRef.Name = seedName
				c1.Status.Conditions = []gardencorev1beta1.Condition{
					// string literal to ensure that the tests fails if the constant is changed
					{
						Type:   "Valid",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "Installed",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "Healthy",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "RandomType",
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   "AnotherRandomType",
						Status: gardencorev1beta1.ConditionFalse,
					},
				}

				c2 := c1.DeepCopy()
				c2.SetName("foo-2")
				c2.Status.Conditions[1].Status = gardencorev1beta1.ConditionFalse

				Expect(indexer.Add(c1)).NotTo(HaveOccurred(), "adding to the index should succeed")
				Expect(indexer.Add(c2)).NotTo(HaveOccurred(), "adding to the index should succeed")

			})

			It("should do nothing", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{current}

				result := controller.ReconcileExtensionCheckFor(seed)

				Expect(result).To(BeNil(), "no error should be returned")
			})
		})
	})
})

type failingControllerInstallationLister struct{}

func (f *failingControllerInstallationLister) List(selector labels.Selector) (ret []*gardencorev1beta1.ControllerInstallation, err error) {
	return nil, fmt.Errorf("dummy error")
}

func (f *failingControllerInstallationLister) Get(name string) (*gardencorev1beta1.ControllerInstallation, error) {
	return nil, fmt.Errorf("dummy error")
}
