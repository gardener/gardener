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

package helper_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("helper", func() {
	Describe("#GetCondition", func() {
		It("should return the found condition", func() {
			var (
				conditionType core.ConditionType = "test-1"
				condition                        = core.Condition{Type: conditionType}
				conditions                       = []core.Condition{condition}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).NotTo(BeNil())
			Expect(*cond).To(Equal(condition))
		})

		It("should return nil because the required condition could not be found", func() {
			var (
				conditionType core.ConditionType = "test-1"
				conditions                       = []core.Condition{}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).To(BeNil())
		})
	})

	DescribeTable("#QuotaScope",
		func(apiVersion, kind, expectedScope string, expectedErr gomegatypes.GomegaMatcher) {
			scope, err := QuotaScope(corev1.ObjectReference{APIVersion: apiVersion, Kind: kind})
			Expect(scope).To(Equal(expectedScope))
			Expect(err).To(expectedErr)
		},

		Entry("project", "core.gardener.cloud/v1beta1", "Project", "project", BeNil()),
		Entry("secret", "v1", "Secret", "secret", BeNil()),
		Entry("unknown", "v2", "Foo", "", HaveOccurred()),
	)

	var (
		trueVar  = true
		falseVar = false
	)

	DescribeTable("#ShootWantsBasicAuthentication",
		func(kubeAPIServerConfig *core.KubeAPIServerConfig, wantsBasicAuth bool) {
			actualWantsBasicAuth := ShootWantsBasicAuthentication(kubeAPIServerConfig)
			Expect(actualWantsBasicAuth).To(Equal(wantsBasicAuth))
		},

		Entry("no kubeapiserver configuration", nil, true),
		Entry("field not set", &core.KubeAPIServerConfig{}, true),
		Entry("explicitly enabled", &core.KubeAPIServerConfig{EnableBasicAuthentication: &trueVar}, true),
		Entry("explicitly disabled", &core.KubeAPIServerConfig{EnableBasicAuthentication: &falseVar}, false),
	)

	DescribeTable("#TaintsHave",
		func(taints []core.SeedTaint, key string, expectation bool) {
			Expect(TaintsHave(taints, key)).To(Equal(expectation))
		},

		Entry("taint exists", []core.SeedTaint{{Key: "foo"}}, "foo", true),
		Entry("taint does not exist", []core.SeedTaint{{Key: "foo"}}, "bar", false),
	)

	DescribeTable("#TaintsAreTolerated",
		func(taints []core.SeedTaint, tolerations []core.Toleration, expectation bool) {
			Expect(TaintsAreTolerated(taints, tolerations)).To(Equal(expectation))
		},

		Entry("no taints",
			nil,
			[]core.Toleration{{Key: "foo"}},
			true,
		),
		Entry("no tolerations",
			[]core.SeedTaint{{Key: "foo"}},
			nil,
			false,
		),
		Entry("taints with keys only, tolerations with keys only (tolerated)",
			[]core.SeedTaint{{Key: "foo"}},
			[]core.Toleration{{Key: "foo"}},
			true,
		),
		Entry("taints with keys only, tolerations with keys only (non-tolerated)",
			[]core.SeedTaint{{Key: "foo"}},
			[]core.Toleration{{Key: "bar"}},
			false,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (tolerated)",
			[]core.SeedTaint{{Key: "foo", Value: pointer.String("bar")}},
			[]core.Toleration{{Key: "foo", Value: pointer.String("bar")}},
			true,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (non-tolerated)",
			[]core.SeedTaint{{Key: "foo", Value: pointer.String("bar")}},
			[]core.Toleration{{Key: "bar", Value: pointer.String("foo")}},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (non-tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "bar"},
				{Key: "foo", Value: pointer.String("baz")},
			},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: pointer.String("bar")},
				{Key: "bar", Value: pointer.String("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (untolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: pointer.String("bar")},
				{Key: "bar", Value: pointer.String("foo")},
			},
			false,
		),
		Entry("taints > tolerations",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "bar", Value: pointer.String("baz")},
			},
			false,
		),
		Entry("tolerations > taints",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: pointer.String("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: pointer.String("bar")},
				{Key: "bar", Value: pointer.String("baz")},
				{Key: "baz", Value: pointer.String("foo")},
			},
			true,
		),
	)

	var (
		unmanagedType = core.DNSUnmanaged
		differentType = "foo"
	)

	DescribeTable("#ShootUsesUnmanagedDNS",
		func(dns *core.DNS, expectation bool) {
			shoot := &core.Shoot{
				Spec: core.ShootSpec{
					DNS: dns,
				},
			}
			Expect(ShootUsesUnmanagedDNS(shoot)).To(Equal(expectation))
		},

		Entry("no dns", nil, false),
		Entry("no dns providers", &core.DNS{}, false),
		Entry("dns providers but no type", &core.DNS{Providers: []core.DNSProvider{{}}}, false),
		Entry("dns providers but different type", &core.DNS{Providers: []core.DNSProvider{{Type: &differentType}}}, false),
		Entry("dns providers and unmanaged type", &core.DNS{Providers: []core.DNSProvider{{Type: &unmanagedType}}}, true),
	)

	DescribeTable("#FindWorkerByName",
		func(workers []core.Worker, name string, expectedWorker *core.Worker) {
			Expect(FindWorkerByName(workers, name)).To(Equal(expectedWorker))
		},

		Entry("no workers", nil, "", nil),
		Entry("worker not found", []core.Worker{{Name: "foo"}}, "bar", nil),
		Entry("worker found", []core.Worker{{Name: "foo"}}, "foo", &core.Worker{Name: "foo"}),
	)

	DescribeTable("#FindPrimaryDNSProvider",
		func(providers []core.DNSProvider, matcher gomegatypes.GomegaMatcher) {
			Expect(FindPrimaryDNSProvider(providers)).To(matcher)
		},

		Entry("no providers", nil, BeNil()),
		Entry("one non primary provider", []core.DNSProvider{{Type: pointer.String("provider")}}, BeNil()),
		Entry("one primary provider", []core.DNSProvider{{Type: pointer.String("provider"),
			Primary: pointer.Bool(true)}}, Equal(&core.DNSProvider{Type: pointer.String("provider"), Primary: pointer.Bool(true)})),
		Entry("multiple w/ one primary provider", []core.DNSProvider{
			{
				Type: pointer.String("provider2"),
			},
			{
				Type:    pointer.String("provider1"),
				Primary: pointer.Bool(true),
			},
			{
				Type: pointer.String("provider3"),
			},
		}, Equal(&core.DNSProvider{Type: pointer.String("provider1"), Primary: pointer.Bool(true)})),
		Entry("multiple w/ multiple primary providers", []core.DNSProvider{
			{
				Type:    pointer.String("provider1"),
				Primary: pointer.Bool(true),
			},
			{
				Type:    pointer.String("provider2"),
				Primary: pointer.Bool(true),
			},
			{
				Type: pointer.String("provider3"),
			},
		}, Equal(&core.DNSProvider{Type: pointer.String("provider1"), Primary: pointer.Bool(true)})),
	)

	Describe("#GetRemovedVersions", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.0.2",
				},
				{
					Version: "1.0.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should detect removed version", func() {
			diff := GetRemovedVersions(versions, versions[0:2])

			Expect(diff).To(HaveLen(1))
			Expect(diff["1.0.0"]).To(Equal(2))
		})

		It("should do nothing", func() {
			diff := GetRemovedVersions(versions, versions)

			Expect(diff).To(HaveLen(0))
		})
	})

	Describe("#GetAddedVersions", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.0.2",
				},
				{
					Version: "1.0.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should detected added versions", func() {
			diff := GetAddedVersions(versions[0:2], versions)

			Expect(diff).To(HaveLen(1))
			Expect(diff["1.0.0"]).To(Equal(2))
		})

		It("should do nothing", func() {
			diff := GetAddedVersions(versions, versions)

			Expect(diff).To(HaveLen(0))
		})
	})

	Describe("#FilterVersionsWithClassification", func() {
		classification := core.ClassificationDeprecated
		var (
			versions = []core.ExpirableVersion{
				{
					Version:        "1.0.2",
					Classification: &classification,
				},
				{
					Version:        "1.0.1",
					Classification: &classification,
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should filter version", func() {
			filteredVersions := FilterVersionsWithClassification(versions, classification)

			Expect(filteredVersions).To(HaveLen(2))
			Expect(filteredVersions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Version":        Equal("1.0.2"),
				"Classification": Equal(&classification),
			}), MatchFields(IgnoreExtras, Fields{
				"Version":        Equal("1.0.1"),
				"Classification": Equal(&classification),
			})))
		})
	})

	Describe("#FindVersionsWithSameMajorMinor", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.1.3",
				},
				{
					Version: "1.1.2",
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should filter version", func() {
			currentSemVer, err := semver.NewVersion("1.1.3")
			Expect(err).ToNot(HaveOccurred())
			filteredVersions, _ := FindVersionsWithSameMajorMinor(versions, *currentSemVer)

			Expect(filteredVersions).To(HaveLen(2))
			Expect(filteredVersions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.1.2"),
			}), MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.1.1"),
			})))
		})
	})

	DescribeTable("#SystemComponentsAllowed",
		func(worker *core.Worker, allowsSystemComponents bool) {
			Expect(SystemComponentsAllowed(worker)).To(Equal(allowsSystemComponents))
		},
		Entry("no systemComponents section", &core.Worker{}, true),
		Entry("systemComponents.allowed = false", &core.Worker{SystemComponents: &core.WorkerSystemComponents{Allow: false}}, false),
		Entry("systemComponents.allowed = true", &core.Worker{SystemComponents: &core.WorkerSystemComponents{Allow: true}}, true),
	)

	DescribeTable("#HibernationIsEnabled",
		func(shoot *core.Shoot, hibernated bool) {
			Expect(HibernationIsEnabled(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section", &core.Shoot{}, false),
		Entry("hibernation.enabled = false", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &falseVar},
			},
		}, false),
		Entry("hibernation.enabled = true", &core.Shoot{
			Spec: core.ShootSpec{
				Hibernation: &core.Hibernation{Enabled: &trueVar},
			},
		}, true),
	)

	DescribeTable("#SeedSettingExcessCapacityReservationEnabled",
		func(settings *core.SeedSettings, expectation bool) {
			Expect(SeedSettingExcessCapacityReservationEnabled(settings)).To(Equal(expectation))
		},

		Entry("setting is nil", nil, true),
		Entry("excess capacity reservation is nil", &core.SeedSettings{}, true),
		Entry("excess capacity reservation 'enabled' is false", &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: false}}, false),
		Entry("excess capacity reservation 'enabled' is true", &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: true}}, true),
	)

	DescribeTable("#SeedSettingSchedulingVisible",
		func(settings *core.SeedSettings, expectation bool) {
			Expect(SeedSettingSchedulingVisible(settings)).To(Equal(expectation))
		},

		Entry("setting is nil", nil, true),
		Entry("scheduling is nil", &core.SeedSettings{}, true),
		Entry("scheduling 'visible' is false", &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: false}}, false),
		Entry("scheduling 'visible' is true", &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: true}}, true),
	)

	DescribeTable("#SeedSettingShootDNSEnabled",
		func(settings *core.SeedSettings, expectation bool) {
			Expect(SeedSettingShootDNSEnabled(settings)).To(Equal(expectation))
		},

		Entry("setting is nil", nil, true),
		Entry("shoot dns is nil", &core.SeedSettings{}, true),
		Entry("shoot dns 'enabled' is false", &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: false}}, false),
		Entry("shoot dns 'enabled' is true", &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: true}}, true),
	)

	classificationPreview := core.ClassificationPreview
	classificationDeprecated := core.ClassificationDeprecated
	classificationSupported := core.ClassificationSupported
	previewVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.2",
			Classification: &classificationPreview,
		},
	}
	deprecatedVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.1",
			Classification: &classificationDeprecated,
		},
	}
	supportedVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.0",
			Classification: &classificationSupported,
		},
	}

	var versions = []core.MachineImageVersion{
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.0",
				Classification: &classificationDeprecated,
			},
		},
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.1",
				Classification: &classificationDeprecated,
			},
		},
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.2",
				Classification: &classificationDeprecated,
			},
		},
		supportedVersion,
		deprecatedVersion,
		previewVersion,
	}

	DescribeTable("#DetermineLatestMachineImageVersion",
		func(versions []core.MachineImageVersion, filterPreviewVersions bool, expectation core.MachineImageVersion, expectError bool) {
			result, err := DetermineLatestMachineImageVersion(versions, filterPreviewVersions)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(result).To(Equal(expectation))
		},

		Entry("should determine latest expirable version - do not ignore preview version", versions, false, previewVersion, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (full list of versions)", versions, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (latest non-deprecated version is earlier in the list)", []core.MachineImageVersion{supportedVersion, deprecatedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (latest deprecated version is earlier in the list)", []core.MachineImageVersion{deprecatedVersion, supportedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - select deprecated version when there is no supported one", []core.MachineImageVersion{previewVersion, deprecatedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.1", Classification: &classificationDeprecated}}, false),
		Entry("should return an error - only preview versions", []core.MachineImageVersion{previewVersion}, true, nil, true),
		Entry("should return an error - empty version slice", []core.MachineImageVersion{}, true, nil, true),
	)

	DescribeTable("#KubernetesDashboardEnabled",
		func(addons *core.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(KubernetesDashboardEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("kubernetesDashboard nil", &core.Addons{}, BeFalse()),
		Entry("kubernetesDashboard disabled", &core.Addons{KubernetesDashboard: &core.KubernetesDashboard{Addon: core.Addon{Enabled: false}}}, BeFalse()),
		Entry("kubernetesDashboard enabled", &core.Addons{KubernetesDashboard: &core.KubernetesDashboard{Addon: core.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#NginxIngressEnabled",
		func(addons *core.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(NginxIngressEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("nginxIngress nil", &core.Addons{}, BeFalse()),
		Entry("nginxIngress disabled", &core.Addons{NginxIngress: &core.NginxIngress{Addon: core.Addon{Enabled: false}}}, BeFalse()),
		Entry("nginxIngress enabled", &core.Addons{NginxIngress: &core.NginxIngress{Addon: core.Addon{Enabled: true}}}, BeTrue()),
	)

	Describe("#ConvertSeed", func() {
		It("should convert the external Seed version to an internal one", func() {
			result, err := ConvertSeed(&gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&core.Seed{}))
		})
	})

	Describe("#ConvertSeedExternal", func() {
		It("should convert the internal Seed version to an external one", func() {
			result, err := ConvertSeedExternal(&core.Seed{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
				},
			}))
		})
	})

	Describe("#CalculateSeedUsage", func() {
		type shootCase struct {
			specSeedName, statusSeedName string
		}

		test := func(shoots []shootCase, expectedUsage map[string]int) {
			var shootList []*core.Shoot

			for i, shoot := range shoots {
				s := &core.Shoot{}
				s.Name = fmt.Sprintf("shoot-%d", i)
				if shoot.specSeedName != "" {
					s.Spec.SeedName = pointer.String(shoot.specSeedName)
				}
				if shoot.statusSeedName != "" {
					s.Status.SeedName = pointer.String(shoot.statusSeedName)
				}
				shootList = append(shootList, s)
			}

			ExpectWithOffset(1, CalculateSeedUsage(shootList)).To(Equal(expectedUsage))
		}

		It("no shoots", func() {
			test([]shootCase{}, map[string]int{})
		})
		It("shoot with both fields unset", func() {
			test([]shootCase{{}}, map[string]int{})
		})
		It("shoot with only spec set", func() {
			test([]shootCase{{specSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with only status set", func() {
			test([]shootCase{{statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with both fields set to same seed", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with fields set to different seeds", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed2"}}, map[string]int{"seed": 1, "seed2": 1})
		})
		It("multiple shoots", func() {
			test([]shootCase{
				{},
				{specSeedName: "seed", statusSeedName: "seed2"},
				{specSeedName: "seed2", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed4"},
			}, map[string]int{"seed": 1, "seed2": 3, "seed3": 2, "seed4": 1})
		})
	})
})
