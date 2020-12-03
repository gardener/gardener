// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	. "github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/pkg/version"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("common", func() {
	Describe("utils", func() {
		Describe("#ComputeOffsetIP", func() {
			Context("IPv4", func() {
				It("should return a cluster IPv4 IP", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/13")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())

					Expect(result).To(HaveLen(net.IPv4len))
					Expect(result).To(Equal(net.ParseIP("100.64.0.10").To4()))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/32")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if ip address is broadcast ip", func() {
					_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
					result, err := ComputeOffsetIP(subnet, 255)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})

			Context("IPv6", func() {
				It("should return a cluster IPv6 IP", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/8")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(net.IPv6len))
					Expect(result).To(Equal(net.ParseIP("fc00::a")))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/128")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})
		})

		Describe("#GenerateAddonConfig", func() {
			Context("values=nil and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values  map[string]interface{}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=nil and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  map[string]interface{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<empty map> and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  = map[string]interface{}{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<non-empty map> and enabled=true", func() {
				It("should return a map with the values and key enabled=true", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					for key := range values {
						_, ok := result[key]
						Expect(ok).To(BeTrue())
					}
					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1+len(values)),
					))
				})
			})

			Context("values=<non-empty map> and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})
		})
	})

	Describe("#MergeOwnerReferences", func() {
		It("should merge the new references into the list of existing references", func() {
			var (
				references = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
				}
				newReferences = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
					{
						UID: types.UID("1235"),
					},
				}
			)

			result := MergeOwnerReferences(references, newReferences...)

			Expect(result).To(ConsistOf(newReferences))
		})
	})

	DescribeTable("#ReplaceCloudProviderConfigKey",
		func(key, oldValue, newValue string) {
			var (
				separator = ": "

				configWithoutQuotes = fmt.Sprintf("%s%s%s", key, separator, oldValue)
				configWithQuotes    = fmt.Sprintf("%s%s\"%s\"", key, separator, strings.Replace(oldValue, `"`, `\"`, -1))
				expected            = fmt.Sprintf("%s%s\"%s\"", key, separator, strings.Replace(newValue, `"`, `\"`, -1))
			)

			Expect(ReplaceCloudProviderConfigKey(configWithoutQuotes, separator, key, newValue)).To(Equal(expected))
			Expect(ReplaceCloudProviderConfigKey(configWithQuotes, separator, key, newValue)).To(Equal(expected))
		},

		Entry("no special characters", "foo", "bar", "baz"),
		Entry("no special characters", "foo", "bar", "baz"),
		Entry("with special characters", "foo", `C*ko4P++$"x`, `"$++*ab*$c4k`),
		Entry("with special characters", "foo", "P+*4", `P*$8uOkv6+4`),
	)

	DescribeTable("#GetDomainInfoFromAnnotations",
		func(annotations map[string]string, expectedProvider, expectedDomain, expectedIncludeZones, expectedExcludeZones, expectedErr gomegatypes.GomegaMatcher) {
			provider, domain, includeZones, excludeZones, err := GetDomainInfoFromAnnotations(annotations)
			Expect(provider).To(expectedProvider)
			Expect(domain).To(expectedDomain)
			Expect(includeZones).To(expectedIncludeZones)
			Expect(excludeZones).To(expectedExcludeZones)
			Expect(err).To(expectedErr)
		},

		Entry("no annotations", nil, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no domain", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no provider", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("all present", map[string]string{
			DNSProvider:     "bar",
			DNSDomain:       "foo",
			DNSIncludeZones: "a,b,c",
			DNSExcludeZones: "d,e,f",
		}, Equal("bar"), Equal("foo"), Equal([]string{"a", "b", "c"}), Equal([]string{"d", "e", "f"}), Not(HaveOccurred())),
	)

	DescribeTable("#RespectSyncPeriodOverwrite",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite",
			true,
			&gardencorev1beta1.Shoot{},
			BeTrue()),
		Entry("don't respect overwrite",
			false,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("don't respect overwrite but garden namespace",
			false,
			&gardencorev1beta1.Shoot{ObjectMeta: kutil.ObjectMeta(v1beta1constants.GardenNamespace, "foo")},
			BeTrue()),
	)

	DescribeTable("#ShouldIgnoreShoot",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite with annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{ShootIgnore: "true"}}},
			BeTrue()),
		Entry("respect overwrite with wrong annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{ShootIgnore: "foo"}}},
			BeFalse()),
		Entry("respect overwrite with no annotation",
			true,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
	)

	DescribeTable("#IsShootFailed",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsShootFailed(shoot)).To(match)
		},

		Entry("no last operation",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("with last operation but not in failed state",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state but not at latest generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation but not latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion + "foo",
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation and latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#IsObservedAtLatestGenerationAndSucceeded",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsObservedAtLatestGenerationAndSucceeded(shoot)).To(match)
		},

		Entry("not at observed generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
			BeFalse()),
		Entry("last operation state not succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateError,
					},
				},
			},
			BeFalse()),
		Entry("observed at latest generation and no last operation state",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("observed at latest generation and last operation state succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#SyncPeriodOfShoot",
		func(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *gardencorev1beta1.Shoot, expected time.Duration) {
			Expect(SyncPeriodOfShoot(respectSyncPeriodOverwrite, defaultMinSyncPeriod, shoot)).To(Equal(expected))
		},

		Entry("don't respect overwrite",
			false,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but no overwrite",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but overwrite invalid",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: "foo"},
				},
			},
			1*time.Second),
		Entry("respect overwrite but overwrite too short",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: (1 * time.Second).String()},
				},
			},
			2*time.Second),
		Entry("respect overwrite with longer overwrite",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: (3 * time.Second).String()},
				},
			},
			3*time.Second),
	)

	Describe("#EffectiveMaintenanceTimeWindow", func() {
		It("should shorten the end of the time window by 15 minutes", func() {
			var (
				begin = utils.NewMaintenanceTime(0, 0, 0)
				end   = utils.NewMaintenanceTime(1, 0, 0)
			)

			Expect(EffectiveMaintenanceTimeWindow(utils.NewMaintenanceTimeWindow(begin, end))).
				To(Equal(utils.NewMaintenanceTimeWindow(begin, utils.NewMaintenanceTime(0, 45, 0))))
		})
	})

	DescribeTable("#EffectiveShootMaintenanceTimeWindow",
		func(shoot *gardencorev1beta1.Shoot, window *utils.MaintenanceTimeWindow) {
			Expect(EffectiveShootMaintenanceTimeWindow(shoot)).To(Equal(window))
		},

		Entry("no maintenance section",
			&gardencorev1beta1.Shoot{},
			utils.AlwaysTimeWindow),
		Entry("no time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{},
				},
			},
			utils.AlwaysTimeWindow),
		Entry("invalid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{},
					},
				},
			},
			utils.AlwaysTimeWindow),
		Entry("valid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "010000+0000",
							End:   "020000+0000",
						},
					},
				},
			},
			utils.NewMaintenanceTimeWindow(
				utils.NewMaintenanceTime(1, 0, 0),
				utils.NewMaintenanceTime(1, 45, 0))),
	)

	Describe("#CheckIfDeletionIsConfirmed", func() {
		It("should prevent the deletion due to missing annotations", func() {
			obj := &corev1.Namespace{}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		Context("deprecated annotation", func() {
			It("should prevent the deletion due annotation value != true", func() {
				obj := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							ConfirmationDeletionDeprecated: "false",
						},
					},
				}

				Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
			})

			It("should allow the deletion due annotation value == true", func() {
				obj := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							ConfirmationDeletionDeprecated: "true",
						},
					},
				}

				Expect(CheckIfDeletionIsConfirmed(obj)).To(Succeed())
			})
		})

		Context("non-deprecated annotation", func() {
			It("should prevent the deletion due annotation value != true", func() {
				obj := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							ConfirmationDeletion: "false",
						},
					},
				}

				Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
			})

			It("should allow the deletion due annotation value == true", func() {
				obj := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							ConfirmationDeletion: "true",
						},
					},
				}

				Expect(CheckIfDeletionIsConfirmed(obj)).To(Succeed())
			})
		})
	})

	Describe("#GetContainerResourcesInStatefulSet", func() {
		var (
			ctrl              *gomock.Controller
			c                 *mockclient.MockClient
			testNamespace     string
			testStatefulset   string
			statefulSet       *appsv1.StatefulSet
			expectedResources *corev1.ResourceRequirements
		)

		BeforeEach(func() {
			expectedResources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("3000Mi"),
				},
			}

			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
			testNamespace = "test-namespace"
			testStatefulset = "test-loki"

			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testStatefulset,
					Namespace: testNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{},
					},
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return container resources when statefulset contains one container", func() {
			var (
				ctx = context.TODO()
			)

			statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:      "contaienr-1",
					Resources: *expectedResources,
				},
			}

			c.EXPECT().Get(ctx, kutil.Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, kutil.Key(testNamespace, testStatefulset))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(len(statefulSet.Spec.Template.Spec.Containers)))
			Expect(rr[0]).To(Equal(expectedResources))
		})

		It("should return all container resources when statefulset contains two containers", func() {
			var (
				ctx = context.TODO()
			)

			statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:      "container-1",
					Resources: *expectedResources,
				},
				{
					Name:      "container-2",
					Resources: *expectedResources,
				},
			}

			c.EXPECT().Get(ctx, kutil.Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, kutil.Key(testNamespace, testStatefulset))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(len(statefulSet.Spec.Template.Spec.Containers)))
			Expect(rr[0]).To(Equal(expectedResources))
			Expect(rr[1]).To(Equal(expectedResources))
		})

		It("should return error if statefulSet is not found", func() {
			var (
				ctx = context.TODO()
			)

			c.EXPECT().Get(ctx, kutil.Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(errors.New("error"))

			_, err := GetContainerResourcesInStatefulSet(ctx, c, kutil.Key(testNamespace, testStatefulset))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ConfirmDeletion", func() {
		var (
			ctrl    *gomock.Controller
			c       *mockclient.MockClient
			now     time.Time
			mockNow *mocktime.MockNow
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockNow = mocktime.NewMockNow(ctrl)
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should add the deletion confirmation annotation for an object without annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{}
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should add the deletion confirmation annotation for an object with annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
				}
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations[ConfirmationDeletion] = "true"
			expectedObj.Annotations[v1beta1constants.GardenerTimestamp] = now.UTC().String()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should ignore non-existing objects", func() {
			var (
				ctx         = context.TODO()
				obj         = &corev1.Namespace{}
				expectedObj = obj.DeepCopy()
			)

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj).Return(apierrors.NewNotFound(corev1.Resource("namespaces"), ""))

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
			Expect(obj).To(Equal(expectedObj))
		})

		It("should retry on conflict and add the deletion confirmation annotation", func() {
			var (
				ctx     = context.TODO()
				baseObj = &corev1.Namespace{}
				obj     = baseObj.DeepCopy()
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj).Return(apierrors.NewConflict(corev1.Resource("namespaces"), "", errors.New("conflict")))
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), expectedObj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
				baseObj.DeepCopyInto(o.(*corev1.Namespace))
				return nil
			})
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})
	})

	Describe("#ExtensionID", func() {
		It("should return the expected identifier", func() {
			Expect(ExtensionID("foo", "bar")).To(Equal("foo/bar"))
		})
	})

	Describe("#DeleteDeploymentsHavingDeprecatedRoleLabelKey", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			ctx     context.Context
			deploy1 *appsv1.Deployment
			deploy2 *appsv1.Deployment
			key1    client.ObjectKey
			key2    client.ObjectKey
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			ctx = context.TODO()
			deploy1 = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: v1beta1constants.GardenNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "foo"},
					},
				},
			}
			deploy2 = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: v1beta1constants.GardenNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "bar"},
					},
				},
			}
			key1 = client.ObjectKey{Name: deploy1.Name, Namespace: deploy1.Namespace}
			key2 = client.ObjectKey{Name: deploy2.Name, Namespace: deploy2.Namespace}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return error if error occurs during get of deployment", func() {
			fakeErr := fmt.Errorf("fake err")

			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).To(MatchError(fakeErr))
		})

		It("should do nothing when the deployments are missing", func() {
			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy1.Name))
			c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy2.Name))

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing when .spec.selector does not have the label key", func() {
			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy1)
			c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy2)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete the deployments when .spec.selector has the label key", func() {
			labelSelector := &metav1.LabelSelector{
				MatchLabels: map[string]string{v1beta1constants.DeprecatedGardenRole: "bar"},
			}
			deploy1.Spec.Selector = labelSelector
			deploy2.Spec.Selector = labelSelector

			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy1)
			c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy2)

			c.EXPECT().Delete(ctx, deploy1)
			c.EXPECT().Delete(ctx, deploy2)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
