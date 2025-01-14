// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	var (
		trueVar                 = true
		falseVar                = false
		expirationDateInThePast = metav1.Time{Time: time.Now().AddDate(0, 0, -1)}
	)

	DescribeTable("#IsResourceSupported",
		func(resources []gardencorev1beta1.ControllerResource, resourceKind, resourceType string, expectation bool) {
			Expect(IsResourceSupported(resources, resourceKind, resourceType)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"bar",
			true,
		),
		Entry("expect true",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"BAR",
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.ControllerResource{
				{
					Kind: "foo",
					Type: "bar",
				},
			},
			"foo",
			"baz",
			false,
		),
	)

	DescribeTable("#IsControllerInstallationSuccessful",
		func(conditions []gardencorev1beta1.Condition, expectation bool) {
			controllerInstallation := gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: conditions,
				},
			}
			Expect(IsControllerInstallationSuccessful(controllerInstallation)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationInstalled,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   gardencorev1beta1.ControllerInstallationProgressing,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{},
			false,
		),
	)

	DescribeTable("#IsControllerInstallationRequired",
		func(conditions []gardencorev1beta1.Condition, expectation bool) {
			controllerInstallation := gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: conditions,
				},
			}
			Expect(IsControllerInstallationRequired(controllerInstallation)).To(Equal(expectation))
		},
		Entry("expect true",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationRequired,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{
				{
					Type:   gardencorev1beta1.ControllerInstallationRequired,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			false,
		),
		Entry("expect false",
			[]gardencorev1beta1.Condition{},
			false,
		),
	)

	DescribeTable("#HasOperationAnnotation",
		func(objectMeta metav1.ObjectMeta, expected bool) {
			Expect(HasOperationAnnotation(objectMeta.Annotations)).To(Equal(expected))
		},
		Entry("reconcile", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}}, true),
		Entry("restore", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore}}, true),
		Entry("migrate", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate}}, true),
		Entry("unknown", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: "unknown"}}, false),
		Entry("not present", metav1.ObjectMeta{}, false),
	)

	DescribeTable("#FindMachineTypeByName",
		func(machines []gardencorev1beta1.MachineType, name string, expectedMachine *gardencorev1beta1.MachineType) {
			Expect(FindMachineTypeByName(machines, name)).To(Equal(expectedMachine))
		},

		Entry("no workers", nil, "", nil),
		Entry("worker not found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "bar", nil),
		Entry("worker found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "foo", &gardencorev1beta1.MachineType{Name: "foo"}),
	)

	DescribeTable("#TaintsHave",
		func(taints []gardencorev1beta1.SeedTaint, key string, expectation bool) {
			Expect(TaintsHave(taints, key)).To(Equal(expectation))
		},
		Entry("taint exists", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "foo", true),
		Entry("taint does not exist", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "bar", false),
	)

	DescribeTable("#TaintsAreTolerated",
		func(taints []gardencorev1beta1.SeedTaint, tolerations []gardencorev1beta1.Toleration, expectation bool) {
			Expect(TaintsAreTolerated(taints, tolerations)).To(Equal(expectation))
		},

		Entry("no taints",
			nil,
			[]gardencorev1beta1.Toleration{{Key: "foo"}},
			true,
		),
		Entry("no tolerations",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			nil,
			false,
		),
		Entry("taints with keys only, tolerations with keys only (tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			[]gardencorev1beta1.Toleration{{Key: "foo"}},
			true,
		),
		Entry("taints with keys only, tolerations with keys only (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			[]gardencorev1beta1.Toleration{{Key: "bar"}},
			false,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]gardencorev1beta1.Toleration{{Key: "foo", Value: ptr.To("bar")}},
			true,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]gardencorev1beta1.Toleration{{Key: "bar", Value: ptr.To("foo")}},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "bar"},
				{Key: "foo", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (untolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("foo")},
			},
			false,
		),
		Entry("taints > tolerations",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "bar", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("tolerations > taints",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "baz", Value: ptr.To("foo")},
			},
			true,
		),
	)

	DescribeTable("#AccessRestrictionsAreSupported",
		func(seedAccessRestrictions []gardencorev1beta1.AccessRestriction, shootAccessRestrictions []gardencorev1beta1.AccessRestrictionWithOptions, expectation bool) {
			Expect(AccessRestrictionsAreSupported(seedAccessRestrictions, shootAccessRestrictions)).To(Equal(expectation))
		},

		Entry("both have no access restrictions",
			nil,
			nil,
			true,
		),
		Entry("shoot has no access restrictions",
			[]gardencorev1beta1.AccessRestriction{{Name: "foo"}},
			nil,
			true,
		),
		Entry("seed has no access restrictions",
			nil,
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			false,
		),
		Entry("both have access restrictions and they match",
			[]gardencorev1beta1.AccessRestriction{{Name: "foo"}},
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			true,
		),
		Entry("both have access restrictions and they don't match",
			[]gardencorev1beta1.AccessRestriction{{Name: "bar"}},
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			false,
		),
	)

	Describe("#ReadManagedSeedAPIServer", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   v1beta1constants.GardenNamespace,
					Annotations: nil,
				},
			}
		})

		It("should return nil,nil when the Shoot is not in the garden namespace", func() {
			shoot.Namespace = "garden-dev"

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return nil,nil when the annotations are nil", func() {
			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return nil,nil when the annotation is not set", func() {
			shoot.Annotations = map[string]string{
				"foo": "bar",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas is specified but maxReplicas is not", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(MatchError("apiSrvMaxReplicas has to be specified for ManagedSeed API server autoscaler"))
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=foo,,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when maxReplicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=foo",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when replicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=foo,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when replicas is invalid", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=-1,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas is greater than maxReplicas", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=2",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return the default the minReplicas and maxReplicas settings when they are not provided", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(Equal(&ManagedSeedAPIServer{
				Replicas: ptr.To[int32](3),
				Autoscaler: &ManagedSeedAPIServerAutoscaler{
					MinReplicas: ptr.To[int32](3),
					MaxReplicas: 3,
				},
			}))
		})

		It("should return the configured settings", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=6",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(Equal(&ManagedSeedAPIServer{
				Replicas: ptr.To[int32](3),
				Autoscaler: &ManagedSeedAPIServerAutoscaler{
					MinReplicas: ptr.To[int32](3),
					MaxReplicas: 6,
				},
			}))
		})
	})

	DescribeTable("#SystemComponentsAllowed",
		func(worker *gardencorev1beta1.Worker, allowsSystemComponents bool) {
			Expect(SystemComponentsAllowed(worker)).To(Equal(allowsSystemComponents))
		},
		Entry("no systemComponents section", &gardencorev1beta1.Worker{}, true),
		Entry("systemComponents.allowed = false", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: false}}, false),
		Entry("systemComponents.allowed = true", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true}}, true),
	)

	DescribeTable("#HibernationIsEnabled",
		func(shoot *gardencorev1beta1.Shoot, hibernated bool) {
			Expect(HibernationIsEnabled(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section", &gardencorev1beta1.Shoot{}, false),
		Entry("hibernation.enabled = false", &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Hibernation: &gardencorev1beta1.Hibernation{Enabled: &falseVar},
			},
		}, false),
		Entry("hibernation.enabled = true", &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Hibernation: &gardencorev1beta1.Hibernation{Enabled: &trueVar},
			},
		}, true),
	)

	DescribeTable("#ShootWantsClusterAutoscaler",
		func(shoot *gardencorev1beta1.Shoot, wantsAutoscaler bool) {
			actualWantsAutoscaler, err := ShootWantsClusterAutoscaler(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualWantsAutoscaler).To(Equal(wantsAutoscaler))
		},
		Entry("no workers",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			},
			false),
		Entry("one worker no difference in auto scaler max and min",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "foo"}},
					},
				},
			},
			false),
		Entry("one worker with difference in auto scaler max and min",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "foo", Minimum: 1, Maximum: 2}},
					},
				},
			},
			true),
	)

	Describe("#ShootWantsVerticalPodAutoscaler", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return false", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = nil
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeFalse())
		})

		It("should return false", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = &gardencorev1beta1.VerticalPodAutoscaler{Enabled: false}
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeFalse())
		})

		It("should return true", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = &gardencorev1beta1.VerticalPodAutoscaler{Enabled: true}
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeTrue())
		})
	})

	var (
		unmanagedType = "unmanaged"
		differentType = "foo"
	)

	DescribeTable("#ShootUsesUnmanagedDNS",
		func(dns *gardencorev1beta1.DNS, expectation bool) {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					DNS: dns,
				},
			}
			Expect(ShootUsesUnmanagedDNS(shoot)).To(Equal(expectation))
		},

		Entry("no dns", nil, false),
		Entry("no dns providers", &gardencorev1beta1.DNS{}, false),
		Entry("dns providers but no type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{}}}, false),
		Entry("dns providers but different type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{Type: &differentType}}}, false),
		Entry("dns providers and unmanaged type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{Type: &unmanagedType}}}, true),
	)

	DescribeTable("#ShootNeedsForceDeletion",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(ShootNeedsForceDeletion(shoot)).To(match)
		},

		Entry("shoot is nil",
			nil,
			BeFalse()),
		Entry("no force-delete annotation present",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("force-delete annotation present but value is false",
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationConfirmationForceDeletion: "0"}}},
			BeFalse()),
		Entry("force-delete annotation present and value is true",
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.AnnotationConfirmationForceDeletion: "t"}}},
			BeTrue()),
	)

	var profile = gardencorev1beta1.SchedulingProfileBinPacking

	DescribeTable("#ShootSchedulingProfile",
		func(shoot *gardencorev1beta1.Shoot, expected *gardencorev1beta1.SchedulingProfile) {
			Expect(ShootSchedulingProfile(shoot)).To(Equal(expected))
		},
		Entry("no kube-scheduler config",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.27.0",
					},
				},
			},
			nil,
		),
		Entry("kube-scheduler profile is set",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.27.0",
						KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{
							Profile: &profile,
						},
					},
				},
			},
			&profile,
		),
	)

	DescribeTable("#SeedSettingExcessCapacityReservationEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expectation bool) {
			Expect(SeedSettingExcessCapacityReservationEnabled(settings)).To(Equal(expectation))
		},

		Entry("setting is nil", nil, true),
		Entry("excess capacity reservation is nil", &gardencorev1beta1.SeedSettings{}, true),
		Entry("excess capacity reservation 'enabled' is nil", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: nil}}, true),
		Entry("excess capacity reservation 'enabled' is false", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: ptr.To(false)}}, false),
		Entry("excess capacity reservation 'enabled' is true", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: ptr.To(true)}}, true),
	)

	DescribeTable("#SeedSettingDependencyWatchdogWeederEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogWeederEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no dwd setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("no dwd weeder setting", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{}}, true),
		Entry("dwd weeder enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: true}}}, true),
		Entry("dwd weeder disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingDependencyWatchdogProberEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogProberEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no dwd setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("no dwd prober setting", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{}}, true),
		Entry("dwd prober enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: true}}}, true),
		Entry("dwd prober disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingTopologyAwareRoutingEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingTopologyAwareRoutingEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, false),
		Entry("no topology-aware routing setting", &gardencorev1beta1.SeedSettings{}, false),
		Entry("topology-aware routing enabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}, true),
		Entry("topology-aware routing disabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}, false),
	)

	Describe("#FindMachineImageVersion", func() {
		var machineImages []gardencorev1beta1.MachineImage

		BeforeEach(func() {
			machineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "coreos",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.2",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.3",
							},
						},
					},
				},
			}
		})

		It("should find the machine image version when it exists", func() {
			expected := gardencorev1beta1.MachineImageVersion{
				ExpirableVersion: gardencorev1beta1.ExpirableVersion{
					Version: "0.0.3",
				},
			}

			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.3")
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))
		})

		It("should return false when machine image with the given name does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "foo", "0.0.3")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})

		It("should return false when machine image version with the given version does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.4")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})
	})

	DescribeTable("#FindPrimaryDNSProvider",
		func(providers []gardencorev1beta1.DNSProvider, matcher gomegatypes.GomegaMatcher) {
			Expect(FindPrimaryDNSProvider(providers)).To(matcher)
		},

		Entry("no providers", nil, BeNil()),
		Entry("one non primary provider", []gardencorev1beta1.DNSProvider{
			{Type: ptr.To("provider")},
		}, BeNil()),
		Entry("one primary provider", []gardencorev1beta1.DNSProvider{{Type: ptr.To("provider"),
			Primary: ptr.To(true)}}, Equal(&gardencorev1beta1.DNSProvider{Type: ptr.To("provider"), Primary: ptr.To(true)})),
		Entry("multiple w/ one primary provider", []gardencorev1beta1.DNSProvider{
			{
				Type: ptr.To("provider2"),
			},
			{
				Type:    ptr.To("provider1"),
				Primary: ptr.To(true),
			},
			{
				Type: ptr.To("provider3"),
			},
		}, Equal(&gardencorev1beta1.DNSProvider{Type: ptr.To("provider1"), Primary: ptr.To(true)})),
		Entry("multiple w/ multiple primary providers", []gardencorev1beta1.DNSProvider{
			{
				Type:    ptr.To("provider1"),
				Primary: ptr.To(true),
			},
			{
				Type:    ptr.To("provider2"),
				Primary: ptr.To(true),
			},
			{
				Type: ptr.To("provider3"),
			},
		}, Equal(&gardencorev1beta1.DNSProvider{Type: ptr.To("provider1"), Primary: ptr.To(true)})),
	)

	Describe("#ShootMachineImageVersionExists", func() {
		var (
			constraint        gardencorev1beta1.MachineImage
			shootMachineImage gardencorev1beta1.ShootMachineImage
		)

		BeforeEach(func() {
			constraint = gardencorev1beta1.MachineImage{
				Name: "coreos",
				Versions: []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.2",
						},
					},
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.3",
						},
					},
				},
			}

			shootMachineImage = gardencorev1beta1.ShootMachineImage{
				Name:    "coreos",
				Version: ptr.To("0.0.2"),
			}
		})

		It("should determine that the version exists", func() {
			exists, index := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(Equal(trueVar))
			Expect(index).To(Equal(0))
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Name = "xy"
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(BeFalse())
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Version = ptr.To("0.0.4")
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(BeFalse())
		})
	})

	Describe("Version helper", func() {
		var previewClassification = gardencorev1beta1.ClassificationPreview
		var deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		var supportedClassification = gardencorev1beta1.ClassificationSupported

		DescribeTable("#GetOverallLatestVersionForAutoUpdate",
			func(versions []gardencorev1beta1.ExpirableVersion, currentVersion string, foundVersion bool, expectedVersion string, expectError bool) {
				qualifyingVersionFound, latestVersion, err := GetOverallLatestVersionForAutoUpdate(versions, currentVersion)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(qualifyingVersionFound).To(Equal(foundVersion))
				Expect(latestVersion).To(Equal(expectedVersion))
			},
			Entry("Get latest version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.17.1",
					},
					{
						Version: "1.15.0",
					},
					{
						Version: "1.14.3",
					},
					{
						Version: "1.13.1",
					},
				},
				"1.14.3",
				true,
				"1.17.1",
				false,
			),
			Entry("Get latest version across major versions",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "3.0.1",
					},
					{
						Version:        "2.1.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "2.0.0",
						Classification: &supportedClassification,
					},
					{
						Version: "0.4.1",
					},
				},
				"0.4.1",
				true,
				"3.0.1",
				false,
			),
			Entry("Get latest version across major versions, preferring lower supported version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "3.0.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "2.1.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "2.0.0",
						Classification: &supportedClassification,
					},
					{
						Version: "0.4.1",
					},
				},
				"0.4.1",
				true,
				"2.0.0",
				false,
			),
			Entry("Expect no higher version than the current version to be found, as already on the latest version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.17.1",
					},
					{
						Version: "1.15.0",
					},
					{
						Version: "1.14.3",
					},
					{
						Version: "1.13.1",
					},
				},
				"1.17.1",
				false,
				"",
				false,
			),
			Entry("Expect to first update to the latest patch version of the same minor before updating to the overall latest version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.17.1",
					},
					{
						Version: "1.15.3",
					},
					{
						Version: "1.15.0",
					},
				},
				"1.15.0",
				true,
				"1.15.3",
				false,
			),
			Entry("Expect no qualifying version to be found - machine image has only versions in preview and expired versions",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.17.1",
						Classification: &previewClassification,
					},
					{
						Version:        "1.15.0",
						Classification: &previewClassification,
					},
					{
						Version:        "1.14.3",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.13.1",
						ExpirationDate: &expirationDateInThePast,
					},
				},
				"1.13.1",
				false,
				"",
				false,
			),
			Entry("Expect older but supported version to be preferred over newer but deprecated one",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.17.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "1.16.1",
						Classification: &supportedClassification,
					},
					{
						Version:        "1.15.0",
						Classification: &previewClassification,
					},
					{
						Version:        "1.14.3",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.13.1",
						ExpirationDate: &expirationDateInThePast,
					},
				},
				"1.13.1",
				true,
				"1.16.1",
				false,
			),
			Entry("Expect latest deprecated version to be selected when there is no supported version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.17.3",
						Classification: &previewClassification,
					},
					{
						Version:        "1.17.2",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.17.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "1.16.1",
						Classification: &deprecatedClassification,
					},
					{
						Version:        "1.15.0",
						Classification: &previewClassification,
					},
					{
						Version:        "1.14.3",
						ExpirationDate: &expirationDateInThePast,
					},
				},
				"1.14.3",
				true,
				"1.17.1",
				false,
			),
		)

		DescribeTable("#GetLatestQualifyingVersion",
			func(original []gardencorev1beta1.ExpirableVersion, expectVersionToBeFound bool, expected *gardencorev1beta1.ExpirableVersion, expectError bool) {
				qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(original, nil)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(qualifyingVersionFound).To(Equal(expectVersionToBeFound))
				Expect(latestVersion).To(Equal(expected))
			},
			Entry("Get latest non-preview version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.17.2",
						Classification: &previewClassification,
					},
					{
						Version: "1.17.1",
					},
					{
						Version: "1.15.0",
					},
					{
						Version: "1.14.3",
					},
					{
						Version: "1.13.1",
					},
				},
				true,
				&gardencorev1beta1.ExpirableVersion{
					Version: "1.17.1",
				},
				false,
			),
			Entry("Expect no qualifying version to be found - no latest version could be found",
				[]gardencorev1beta1.ExpirableVersion{},
				false,
				nil,
				false,
			),
			Entry("Expect error, because contains invalid semVer",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.213123xx",
					},
				},
				false,
				nil,
				true,
			),
		)

		DescribeTable("#GetQualifyingVersionForNextHigher",
			func(original []gardencorev1beta1.ExpirableVersion, currentVersion string, getNextHigherMinor bool, expectVersionToBeFound bool, expected *string, expectedNextMinorOrMajorVersion uint64, expectError bool) {
				var (
					majorMinor    GetMajorOrMinor
					filterSmaller VersionPredicate
				)

				currentSemVerVersion := semver.MustParse(currentVersion)

				// setup filter for smaller minor or smaller major
				if getNextHigherMinor {
					majorMinor = func(v semver.Version) uint64 { return v.Minor() }
					filterSmaller = FilterEqualAndSmallerMinorVersion(*currentSemVerVersion)
				} else {
					majorMinor = func(v semver.Version) uint64 { return v.Major() }
					filterSmaller = FilterEqualAndSmallerMajorVersion(*currentSemVerVersion)
				}

				foundVersion, qualifyingVersion, nextMinorOrMajorVersion, err := GetQualifyingVersionForNextHigher(original, majorMinor, currentSemVerVersion, filterSmaller)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(nextMinorOrMajorVersion).To(Equal(expectedNextMinorOrMajorVersion))
				Expect(err).ToNot(HaveOccurred())
				Expect(foundVersion).To(Equal(expectVersionToBeFound))
				if foundVersion {
					Expect(qualifyingVersion.Version).To(Equal(*expected))
				}
			},
			Entry("Get latest non-preview version for next higher minor version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.3.2",
						Classification: &previewClassification,
					},
					{
						Version:        "1.3.2",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.3.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version: "1.1.1",
					},
					{
						Version: "1.0.0",
					},
				},
				"1.1.0",
				true, // target minor
				true,
				ptr.To("1.3.2"),
				uint64(3), // next minor version to be found
				false,
			),
			Entry("Get latest non-preview version for next higher major version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "4.4.2",
						Classification: &previewClassification,
					},
					{
						Version:        "4.3.2",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "4.3.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version: "1.1.0",
					},
					{
						Version: "1.0.0",
					},
				},
				"1.1.0",
				false, // target major
				true,
				ptr.To("4.3.2"),
				uint64(4), // next major version to be found
				false,
			),
			Entry("Skip next higher minor version if contains no qualifying version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.4.2",
					},
					{
						Version:        "1.3.2",
						Classification: &previewClassification,
					},
					{
						Version: "1.1.1",
					},
					{
						Version: "1.0.0",
					},
				},
				"1.1.0",
				true, // target minor
				true,
				ptr.To("1.4.2"),
				uint64(3), // next minor version to be found
				false,
			),
			Entry("Skip next higher major version if contains no qualifying version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "4.4.2",
					},
					{
						Version:        "3.3.2",
						Classification: &previewClassification,
					},
					{
						Version: "1.1.1",
					},
					{
						Version: "1.0.0",
					},
				},
				"1.1.0",
				false, // target major
				true,
				ptr.To("4.4.2"),
				uint64(3), // next major version to be found
				false,
			),
			Entry("Expect no version to be found: already on highest version in major",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "2.0.0",
					},
					{
						Version:        "1.3.2",
						Classification: &previewClassification,
					},
					{
						Version: "1.1.1",
					},
					{
						Version: "1.0.0",
					},
				},
				"1.1.0",
				true, // target minor
				false,
				nil,
				uint64(3), // next minor version to be found
				false,
			),
			Entry("Expect no version to be found: already on overall highest version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "2.0.0",
					},
				},
				"2.0.0",
				false, // target major
				false,
				nil,
				uint64(0), // next minor version to be found
				false,
			),
			Entry("Expect no qualifying version to be found - no latest version could be found",
				[]gardencorev1beta1.ExpirableVersion{},
				"1.1.0",
				true, // target minor
				false,
				nil,
				uint64(0),
				false,
			),
			Entry("Expect error, because contains invalid semVer",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.213123xx",
					},
				},
				"1.1.0",
				false,
				false,
				nil,
				uint64(1),
				true,
			),
		)

		Describe("#Expirable Version Helper", func() {
			classificationPreview := gardencorev1beta1.ClassificationPreview

			DescribeTable("#GetLatestVersionForPatchAutoUpdate",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					ok, newVersion, err := GetLatestVersionForPatchAutoUpdate(cloudProfileVersions, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Do not consider preview versions for patch update.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						// latest qualifying version for updating version 1.12.2
						{Version: "1.12.3"},
						{Version: "1.12.2"},
					},
					"1.12.3",
					true,
				),
				Entry("Do not consider expired versions for patch update.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.12.2
						{Version: "1.12.3"},
						{Version: "1.12.2"},
					},
					"1.12.3",
					true,
				),
				Entry("Should not find qualifying version - no higher version available that is not expired or in preview.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already highest version of minor.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
						{Version: "1.12.1"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already on latest version of latest minor.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)

			DescribeTable("#GetLatestVersionForMinorAutoUpdate",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					foundVersion, newVersion, err := GetLatestVersionForMinorAutoUpdate(cloudProfileVersions, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(foundVersion).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Should find qualifying version - the latest version for the major.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"1.15.1",
					true,
				),
				Entry("Should find qualifying version - the latest version for the major.",
					"0.2.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.15.1"},
						{Version: "0.4.1"},
						{Version: "0.4.0"},
						{Version: "0.2.3"},
					},
					"0.4.1",
					true,
				),
				Entry("Should find qualifying version - do not consider preview versions for auto updates.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{
							Version:        "1.15.2",
							Classification: &previewClassification,
						},
						// latest qualifying version for updating version 1.12.2 to the latest version for the major
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
					},
					"1.15.1",
					true,
				),
				Entry("Should find qualifying version - always first update to latest patch of minor.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{
							Version:        "1.15.2",
							Classification: &previewClassification,
						},
						// latest qualifying version for updating version 1.12.2 to the latest version for the major
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.3"},
						{Version: "1.12.2"},
					},
					"1.12.3",
					true,
				),
				Entry("Should find qualifying version - do not consider expired versions for auto updates.",
					"1.1.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.1.2
						{Version: "1.10.3"},
						{Version: "1.10.2"},
						{Version: "1.1.2"},
						{Version: "1.0.2"},
					},
					"1.10.3",
					true,
				),
				Entry("Should not find qualifying version - no higher version available that is not expired or in preview.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{
							Version:        "1.15.1",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.15.0",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.14.4",
							Classification: &previewClassification,
						},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already highest version of major.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
						{Version: "1.12.1"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - current version is higher than any given version in major.",
					"1.17.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
						{Version: "1.12.1"},
					},
					"",
					false,
				),
			)

			DescribeTable("#GetVersionForForcefulUpdateToConsecutiveMinor",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					ok, newVersion, err := GetVersionForForcefulUpdateToConsecutiveMinor(cloudProfileVersions, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Do not consider preview versions of the consecutive minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						// latest qualifying version for minor version update for version 1.11.3
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.11.3"},
					},
					"1.12.3",
					true,
				),
				Entry("Should find qualifying version - latest non-expired version of the consecutive minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.11.3
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.11.3"},
						{Version: "1.10.1"},
						{Version: "1.9.0"},
					},
					"1.12.3",
					true,
				),
				// check that multiple consecutive minor versions are possible
				Entry("Should find qualifying version if there are only expired versions available in the consecutive minor version - pick latest expired version of that minor.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.11.3"},
					},
					"1.12.9",
					true,
				),
				Entry("Should not find qualifying version - there is no consecutive minor version available.",
					"1.10.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.10.3"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - already on latest minor version.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already on latest version of latest minor version.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)

			DescribeTable("#GetVersionForForcefulUpdateToNextHigherMinor",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					ok, newVersion, err := GetVersionForForcefulUpdateToNextHigherMinor(cloudProfileVersions, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Should find qualifying version - but do not consider preview versions of the next minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.13.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1.13.4",
							Classification: &previewClassification,
						},
						// latest qualifying version for minor version update for version 1.11.3
						{Version: "1.13.3"},
						{Version: "1.13.2"},
						{Version: "1.11.3"},
					},
					"1.13.3",
					true,
				),
				Entry("Should find qualifying version - latest non-expired version of the next minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.11.3
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.11.3"},
						{Version: "1.10.1"},
						{Version: "1.9.0"},
					},
					"1.12.3",
					true,
				),
				Entry("Should find qualifying version if the latest version in next minor is expired - pick latest non-expired version of that minor.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1.13.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version: "1.13.4",
						},
						{Version: "1.11.3"},
					},
					"1.13.4",
					true,
				),
				// check that multiple consecutive minor versions are possible
				Entry("Should find qualifying version if there are only expired versions available in the next minor version - pick latest expired version of that minor.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1.13.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.13.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.11.3"},
					},
					"1.13.9",
					true,
				),
				Entry("Should find qualifying version - there is a next higher minor version available.",
					"1.10.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.10.3"},
					},
					"1.12.3",
					true,
				),
				Entry("Should find qualifying version - but skip over next higher minor as it does not contain qualifying versions.",
					"1.10.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							Classification: &classificationPreview,
						},
						{
							Version:        "1.12.4",
							Classification: &classificationPreview,
						},
						{Version: "1.10.3"},
					},
					"1.15.1",
					true,
				),
				Entry("Should not find qualifying version - already on latest minor version.",
					"1.17.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.0.0"},
						{Version: "1.17.1"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already on latest version of latest minor version.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)

			DescribeTable("#GetVersionForForcefulUpdateToNextHigherMajor",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					ok, newVersion, err := GetVersionForForcefulUpdateToNextHigherMajor(cloudProfileVersions, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Should find qualifying version - but do not consider preview versions of the next major version.",
					"534.6.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1096.0.0"},
						// latest qualifying version for minor version update for version 1.11.3
						{Version: "1034.1.1"},
						{Version: "1034.0.0"},
						{
							Version:        "1034.0.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1034.1.4",
							Classification: &previewClassification,
						},
						{Version: "534.6.3"},
						{Version: "534.5.0"},
						{Version: "1.11.3"},
					},
					"1034.1.1",
					true,
				),
				Entry("Should find qualifying version - latest non-expired version of the next major version.",
					"534.0.0",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1096.0.0"},
						{Version: "1034.5.1"},
						{Version: "1034.5.0"},
						{Version: "1034.2.0"},
						{
							Version:        "1034.1.0",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1034.0.0",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "534.0.0"},
					},
					"1034.5.1",
					true,
				),
				Entry("Should find qualifying version if the latest version in next major is expired - pick latest non-expired version of that major.",
					"534.0.0",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1096.0.1"},
						{Version: "1096.0.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1034.1.1",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version: "1034.1.0",
						},
						{
							Version:        "1034.0.1",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "534.0.0"},
					},
					"1034.1.0",
					true,
				),
				Entry("Should find qualifying version if there are only expired versions available in the next major version - pick latest expired version of that major.",
					"534.0.0",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1096.0.1"},
						{Version: "1096.0.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1034.1.1",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1034.1.0",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1034.0.1",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "534.0.0"},
					},
					"1034.1.1",
					true,
				),
				Entry("Should find qualifying version - skip over next higher major as it contains no qualifying version.",
					"534.0.0",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1096.1.0"},
						{Version: "1096.0.0"},
						{
							Version:        "1034.1.0",
							Classification: &previewClassification,
						},
						{
							Version:        "1034.0.0",
							Classification: &previewClassification,
						},
						{Version: "534.0.0"},
					},
					"1096.1.0",
					true,
				),
				Entry("Should not find qualifying version - already on latest overall version.",
					"2.1.0",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "2.1.0"},
						{Version: "2.0.0"},
						{Version: "1.17.1"},
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)

			DescribeTable("Test version filter predicates",
				func(predicate VersionPredicate, version *semver.Version, expirableVersion gardencorev1beta1.ExpirableVersion, expectFilterVersion, expectError bool) {
					shouldFilter, err := predicate(expirableVersion, version)
					if expectError {
						Expect(err).To(HaveOccurred())
						return
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(shouldFilter).To(Equal(expectFilterVersion))
				},

				// #FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor
				Entry("Should filter version - has not the same major.minor.",
					FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.2.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - version has same major.minor but is lower",
					FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.1.2")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - has the same major.minor.",
					FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.1.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterNonConsecutiveMinorVersion
				Entry("Should filter version - has not the consecutive minor version.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.3.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - has the same minor version.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - has consecutive minor.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
					semver.MustParse("1.2.0"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterSameVersion
				Entry("Should filter version.",
					FilterSameVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version.",
					FilterSameVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.2"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterExpiredVersion
				Entry("Should filter expired version.",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{
						ExpirationDate: &expirationDateInThePast,
					},
					true,
					false,
				),
				Entry("Should not filter version - expiration date is not expired",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{
						ExpirationDate: &metav1.Time{Time: time.Now().Add(time.Hour)},
					},
					false,
					false,
				),
				Entry("Should not filter version.",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterDeprecatedVersion
				Entry("Should filter version - version is deprecated",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &deprecatedClassification},
					true,
					false,
				),
				Entry("Should not filter version - version has preview classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &previewClassification},
					false,
					false,
				),
				Entry("Should not filter version - version has supported classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &supportedClassification},
					false,
					false,
				),
				Entry("Should not filter version - version has no classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterLowerVersion
				Entry("Should filter version - version is lower",
					FilterLowerVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.0"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - version is higher / equal",
					FilterLowerVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.2"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterEqualAndSmallerMinorVersion
				Entry("Should filter version - version has the same minor version",
					FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
					semver.MustParse("1.1.6"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - version has smaller minor version",
					FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
					semver.MustParse("1.0.0"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - version has higher minor version",
					FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
					semver.MustParse("1.2.0"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterEqualAndSmallerMajorVersion
				Entry("Should filter version - version has the same major version",
					FilterEqualAndSmallerMajorVersion(*semver.MustParse("2.1.5")),
					semver.MustParse("2.3.6"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - version has smaller major version",
					FilterEqualAndSmallerMajorVersion(*semver.MustParse("2.1.5")),
					semver.MustParse("1.0.0"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - version has higher major version",
					FilterEqualAndSmallerMajorVersion(*semver.MustParse("1.1.5")),
					semver.MustParse("2.2.0"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterDifferentMajorVersion
				Entry("Should filter version - version has the higher major version",
					FilterDifferentMajorVersion(*semver.MustParse("1.1.5")),
					semver.MustParse("2.3.6"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - version has smaller major version",
					FilterDifferentMajorVersion(*semver.MustParse("2.1.5")),
					semver.MustParse("1.0.0"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - version has the same major version",
					FilterDifferentMajorVersion(*semver.MustParse("2.1.5")),
					semver.MustParse("2.2.0"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
			)
		})

		DescribeTable("#GetResourceByName",
			func(resources []gardencorev1beta1.NamedResourceReference, name string, expected *gardencorev1beta1.NamedResourceReference) {
				actual := GetResourceByName(resources, name)
				Expect(actual).To(Equal(expected))
			},

			Entry("resources is nil", nil, "foo", nil),
			Entry("resources doesn't contain a resource with the given name",
				[]gardencorev1beta1.NamedResourceReference{
					{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "bar"}},
					{Name: "baz", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", Name: "baz"}},
				},
				"foo",
				nil,
			),
			Entry("resources contains a resource with the given name",
				[]gardencorev1beta1.NamedResourceReference{
					{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "bar"}},
					{Name: "baz", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "ConfigMap", Name: "baz"}},
					{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "foo"}},
				},
				"foo",
				&gardencorev1beta1.NamedResourceReference{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{Kind: "Secret", Name: "foo"}},
			),
		)

		DescribeTable("#UpsertLastError",
			func(lastErrors []gardencorev1beta1.LastError, lastError gardencorev1beta1.LastError, expected []gardencorev1beta1.LastError) {
				Expect(UpsertLastError(lastErrors, lastError)).To(Equal(expected))
			},

			Entry(
				"insert",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
				gardencorev1beta1.LastError{TaskID: ptr.To("foo"), Description: "error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
					{TaskID: ptr.To("foo"), Description: "error"},
				},
			),
			Entry(
				"update",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo"), Description: "error"},
					{TaskID: ptr.To("bar")},
				},
				gardencorev1beta1.LastError{TaskID: ptr.To("foo"), Description: "new-error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo"), Description: "new-error"},
					{TaskID: ptr.To("bar")},
				},
			),
		)

		DescribeTable("#DeleteLastErrorByTaskID",
			func(lastErrors []gardencorev1beta1.LastError, taskID string, expected []gardencorev1beta1.LastError) {
				Expect(DeleteLastErrorByTaskID(lastErrors, taskID)).To(Equal(expected))
			},

			Entry(
				"task id not found",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
			),
			Entry(
				"task id found",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo")},
					{TaskID: ptr.To("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
			),
		)
	})

	Describe("ShootItems", func() {
		Describe("#Union", func() {
			It("tests if provided two sets of shoot slices will return ", func() {
				shootList1 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace2",
							},
						},
					},
				}

				shootList2 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace2",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace3",
							},
						},
					},
				}

				s := ShootItems(shootList1)
				s2 := ShootItems(shootList2)
				shootSet := s.Union(&s2)

				Expect(shootSet).To(HaveLen(5))
			})

			It("should not fail if one of the lists is empty", func() {
				shootList1 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{},
				}

				shootList2 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace2",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace3",
							},
						},
					},
				}

				s := ShootItems(shootList1)
				s2 := ShootItems(shootList2)
				shootSet := s.Union(&s2)
				Expect(shootSet).To(HaveLen(3))

				shootSet2 := s2.Union(&s)
				Expect(shootSet).To(HaveLen(3))
				Expect(shootSet).To(ConsistOf(shootSet2))

			})
		})

		It("should not fail if no items", func() {
			shootList1 := gardencorev1beta1.ShootList{}

			shootList2 := gardencorev1beta1.ShootList{}

			s := ShootItems(shootList1)
			s2 := ShootItems(shootList2)
			shootSet := s.Union(&s2)
			Expect(shootSet).To(BeEmpty())
		})
	})

	Describe("#GetPurpose", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			}
		})

		It("should get default purpose if not defined", func() {
			purpose := GetPurpose(shoot)
			Expect(purpose).To(Equal(gardencorev1beta1.ShootPurposeEvaluation))
		})

		It("should get purpose", func() {
			shootPurpose := gardencorev1beta1.ShootPurposeProduction
			shoot.Spec.Purpose = &shootPurpose
			purpose := GetPurpose(shoot)
			Expect(purpose).To(Equal(shootPurpose))
		})
	})

	Context("Shoot Alerts", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		Describe("#ShootIgnoresAlerts", func() {
			It("should not ignore alerts because no annotations given", func() {
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should not ignore alerts because annotation is not given", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "foo", "bar")
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should not ignore alerts because annotation value is false", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "false")
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should ignore alerts because annotation value is true", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")
				Expect(ShootIgnoresAlerts(shoot)).To(BeTrue())
			})
		})

		Describe("#ShootWantsAlertManager", func() {
			It("should not want alert manager because alerts are ignored", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing monitoring configuration", func() {
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing alerting configuration", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing email configuration", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{
						Alerting: &gardencorev1beta1.Alerting{},
					},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should want alert manager", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{
						Alerting: &gardencorev1beta1.Alerting{
							EmailReceivers: []string{"operators@gardener.clou"},
						},
					},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeTrue())
			})
		})
	})

	DescribeTable("#KubernetesDashboardEnabled",
		func(addons *gardencorev1beta1.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(KubernetesDashboardEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("kubernetesDashboard nil", &gardencorev1beta1.Addons{}, BeFalse()),
		Entry("kubernetesDashboard disabled", &gardencorev1beta1.Addons{KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{Addon: gardencorev1beta1.Addon{Enabled: false}}}, BeFalse()),
		Entry("kubernetesDashboard enabled", &gardencorev1beta1.Addons{KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{Addon: gardencorev1beta1.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#NginxIngressEnabled",
		func(addons *gardencorev1beta1.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(NginxIngressEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("nginxIngress nil", &gardencorev1beta1.Addons{}, BeFalse()),
		Entry("nginxIngress disabled", &gardencorev1beta1.Addons{NginxIngress: &gardencorev1beta1.NginxIngress{Addon: gardencorev1beta1.Addon{Enabled: false}}}, BeFalse()),
		Entry("nginxIngress enabled", &gardencorev1beta1.Addons{NginxIngress: &gardencorev1beta1.NginxIngress{Addon: gardencorev1beta1.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#KubeProxyEnabled",
		func(kubeProxy *gardencorev1beta1.KubeProxyConfig, matcher gomegatypes.GomegaMatcher) {
			Expect(KubeProxyEnabled(kubeProxy)).To(matcher)
		},

		Entry("kubeProxy nil", nil, BeFalse()),
		Entry("kubeProxy empty", &gardencorev1beta1.KubeProxyConfig{}, BeFalse()),
		Entry("kubeProxy disabled", &gardencorev1beta1.KubeProxyConfig{Enabled: ptr.To(false)}, BeFalse()),
		Entry("kubeProxy enabled", &gardencorev1beta1.KubeProxyConfig{Enabled: ptr.To(true)}, BeTrue()),
	)

	DescribeTable("#BackupBucketIsErroneous",
		func(bb *gardencorev1beta1.BackupBucket, matcher1, matcher2 gomegatypes.GomegaMatcher) {
			erroneous, msg := BackupBucketIsErroneous(bb)
			Expect(erroneous).To(matcher1)
			Expect(msg).To(matcher2)
		},

		Entry("W/o BackupBucket", nil, BeFalse(), BeEmpty()),
		Entry("W/o last error", &gardencorev1beta1.BackupBucket{}, BeFalse(), BeEmpty()),
		Entry("W/ last error",
			&gardencorev1beta1.BackupBucket{Status: gardencorev1beta1.BackupBucketStatus{LastError: &gardencorev1beta1.LastError{Description: "foo"}}},
			BeTrue(),
			Equal("foo"),
		),
	)

	DescribeTable("#SeedBackupSecretRefEqual",
		func(oldBackup, newBackup *gardencorev1beta1.SeedBackup, matcher gomegatypes.GomegaMatcher) {
			Expect(SeedBackupSecretRefEqual(oldBackup, newBackup)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old nil, new empty", nil, &gardencorev1beta1.SeedBackup{}, BeTrue()),
		Entry("old empty, new nil", &gardencorev1beta1.SeedBackup{}, nil, BeTrue()),
		Entry("both empty", &gardencorev1beta1.SeedBackup{}, &gardencorev1beta1.SeedBackup{}, BeTrue()),
		Entry("difference", &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "bar", Namespace: "foo"}}, BeFalse()),
		Entry("equality", &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, BeTrue()),
	)

	DescribeTable("#ShootDNSProviderSecretNamesEqual",
		func(oldDNS, newDNS *gardencorev1beta1.DNS, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootDNSProviderSecretNamesEqual(oldDNS, newDNS)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old nil, new w/o secret names", nil, &gardencorev1beta1.DNS{}, BeTrue()),
		Entry("old w/o secret names, new nil", &gardencorev1beta1.DNS{}, nil, BeTrue()),
		Entry("difference due to old", &gardencorev1beta1.DNS{}, &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: ptr.To("foo")}}}, BeFalse()),
		Entry("difference due to new", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: ptr.To("foo")}}}, &gardencorev1beta1.DNS{}, BeFalse()),
		Entry("equality", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: ptr.To("foo")}}}, &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: ptr.To("foo")}}}, BeTrue()),
	)

	DescribeTable("#ShootResourceReferencesEqual",
		func(oldResources, newResources []gardencorev1beta1.NamedResourceReference, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootResourceReferencesEqual(oldResources, newResources)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old empty, new w/o secrets", []gardencorev1beta1.NamedResourceReference{}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, BeTrue()),
		Entry("old empty, new w/ secrets", []gardencorev1beta1.NamedResourceReference{}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeFalse()),
		Entry("old empty, new w/ configMap", []gardencorev1beta1.NamedResourceReference{}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo"}}}, BeFalse()),
		Entry("old w/o secrets, new empty", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{}, BeTrue()),
		Entry("old w/ secrets, new empty", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{}, BeFalse()),
		Entry("difference", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "bar"}}}, BeFalse()),
		Entry("difference because no secret", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "bar"}}}, BeFalse()),
		Entry("difference because new is configMap with same name", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo"}}}, BeFalse()),
		Entry("equality", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeTrue()),
	)

	DescribeTable("#AnonymousAuthenticationEnabled",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, wantsAnonymousAuth bool) {
			actualWantsAnonymousAuth := AnonymousAuthenticationEnabled(kubeAPIServerConfig)
			Expect(actualWantsAnonymousAuth).To(Equal(wantsAnonymousAuth))
		},

		Entry("no kubeapiserver configuration", nil, false),
		Entry("field not set", &gardencorev1beta1.KubeAPIServerConfig{}, false),
		Entry("explicitly enabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &trueVar}, true),
		Entry("explicitly disabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &falseVar}, false),
	)

	Describe("GetShootAuditPolicyConfigMapName", func() {
		test := func(description string, config *gardencorev1beta1.KubeAPIServerConfig, expectedName string) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapName(config)).To(Equal(expectedName))
			})
		}

		test("KubeAPIServerConfig = nil", nil, "")
		test("AuditConfig = nil", &gardencorev1beta1.KubeAPIServerConfig{}, "")
		test("AuditPolicy = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{},
		}, "")
		test("ConfigMapRef = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{},
			},
		}, "")
		test("ConfigMapRef set", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, "foo")
	})

	Describe("GetShootAuditPolicyConfigMapRef", func() {
		test := func(description string, config *gardencorev1beta1.KubeAPIServerConfig, expectedRef *corev1.ObjectReference) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapRef(config)).To(Equal(expectedRef))
			})
		}

		test("KubeAPIServerConfig = nil", nil, nil)
		test("AuditConfig = nil", &gardencorev1beta1.KubeAPIServerConfig{}, nil)
		test("AuditPolicy = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{},
		}, nil)
		test("ConfigMapRef = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{},
			},
		}, nil)
		test("ConfigMapRef set", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, &corev1.ObjectReference{Name: "foo"})
	})

	DescribeTable("#GetShootAuthenticationConfigurationConfigMapName",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, expectedName string) {
			authConfigName := GetShootAuthenticationConfigurationConfigMapName(kubeAPIServerConfig)
			Expect(authConfigName).To(Equal(expectedName))
		},

		Entry("KubeAPIServerConfig = nil", nil, ""),
		Entry("StructuredAuthentication = nil", &gardencorev1beta1.KubeAPIServerConfig{}, ""),
		Entry("ConfigMapName not set", &gardencorev1beta1.KubeAPIServerConfig{
			StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{},
		}, ""),
		Entry("ConfigMapName set", &gardencorev1beta1.KubeAPIServerConfig{
			StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
				ConfigMapName: "foo",
			},
		}, "foo"),
	)

	DescribeTable("#GetShootAuthorizationConfigurationConfigMapName",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, expectedName string) {
			authConfigName := GetShootAuthorizationConfigurationConfigMapName(kubeAPIServerConfig)
			Expect(authConfigName).To(Equal(expectedName))
		},

		Entry("KubeAPIServerConfig = nil", nil, ""),
		Entry("StructuredAuthorization = nil", &gardencorev1beta1.KubeAPIServerConfig{}, ""),
		Entry("ConfigMapName not set", &gardencorev1beta1.KubeAPIServerConfig{
			StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{},
		}, ""),
		Entry("ConfigMapName set", &gardencorev1beta1.KubeAPIServerConfig{
			StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
				ConfigMapName: "foo",
			},
		}, "foo"),
	)

	DescribeTable("#GetShootAuthorizationConfiguration",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, expectedResult *gardencorev1beta1.StructuredAuthorization) {
			Expect(GetShootAuthorizationConfiguration(kubeAPIServerConfig)).To(Equal(expectedResult))
		},

		Entry("KubeAPIServerConfig = nil", nil, nil),
		Entry("StructuredAuthorization not set", &gardencorev1beta1.KubeAPIServerConfig{}, nil),
		Entry("StructuredAuthorization set", &gardencorev1beta1.KubeAPIServerConfig{StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{}}, &gardencorev1beta1.StructuredAuthorization{}),
	)

	Describe("#CalculateSeedUsage", func() {
		type shootCase struct {
			specSeedName, statusSeedName string
		}

		test := func(shoots []shootCase, expectedUsage map[string]int) {
			var shootList []*gardencorev1beta1.Shoot

			for i, shoot := range shoots {
				s := &gardencorev1beta1.Shoot{}
				s.Name = fmt.Sprintf("shoot-%d", i)
				if shoot.specSeedName != "" {
					s.Spec.SeedName = ptr.To(shoot.specSeedName)
				}
				if shoot.statusSeedName != "" {
					s.Status.SeedName = ptr.To(shoot.statusSeedName)
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

	DescribeTable("#CalculateEffectiveKubernetesVersion",
		func(controlPlaneVersion *semver.Version, workerKubernetes *gardencorev1beta1.WorkerKubernetes, expectedRes *semver.Version) {
			res, err := CalculateEffectiveKubernetesVersion(controlPlaneVersion, workerKubernetes)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expectedRes))
		},

		Entry("workerKubernetes = nil", semver.MustParse("1.2.3"), nil, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version = nil", semver.MustParse("1.2.3"), &gardencorev1beta1.WorkerKubernetes{}, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version != nil", semver.MustParse("1.2.3"), &gardencorev1beta1.WorkerKubernetes{Version: ptr.To("4.5.6")}, semver.MustParse("4.5.6")),
	)

	var (
		sampleShootKubelet = &gardencorev1beta1.KubeletConfig{
			MaxPods: ptr.To(int32(50)),
		}
		sampleWorkerKubelet = &gardencorev1beta1.KubeletConfig{
			MaxPods: ptr.To(int32(100)),
		}
	)
	DescribeTable("#CalculateEffectiveKubeletConfiguration",
		func(shootKubelet *gardencorev1beta1.KubeletConfig, workerKubernetes *gardencorev1beta1.WorkerKubernetes, expectedRes *gardencorev1beta1.KubeletConfig) {
			res := CalculateEffectiveKubeletConfiguration(shootKubelet, workerKubernetes)
			Expect(res).To(Equal(expectedRes))
		},

		Entry("all nil", nil, nil, nil),
		Entry("workerKubernetes = nil", sampleShootKubelet, nil, sampleShootKubelet),
		Entry("workerKubernetes.kubelet = nil", sampleShootKubelet, &gardencorev1beta1.WorkerKubernetes{}, sampleShootKubelet),
		Entry("workerKubernetes.kubelet != nil", sampleShootKubelet, &gardencorev1beta1.WorkerKubernetes{Kubelet: sampleWorkerKubelet}, sampleWorkerKubelet),
	)

	DescribeTable("#GetSecretBindingTypes",
		func(secretBinding *gardencorev1beta1.SecretBinding, expected []string) {
			actual := GetSecretBindingTypes(secretBinding)
			Expect(actual).To(Equal(expected))
		},

		Entry("with single-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, []string{"foo"}),
		Entry("with multi-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar,baz"}}, []string{"foo", "bar", "baz"}),
	)

	DescribeTable("#SecretBindingHasType",
		func(secretBinding *gardencorev1beta1.SecretBinding, toFind string, expected bool) {
			actual := SecretBindingHasType(secretBinding, toFind)
			Expect(actual).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", false),
		Entry("when single-value provider type equals to the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", true),
		Entry("when single-value provider type does not match the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", false),
		Entry("when multi-value provider type contains the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "bar", true),
		Entry("when multi-value provider type does not contain the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", false),
	)

	DescribeTable("#AddTypeToSecretBinding",
		func(secretBinding *gardencorev1beta1.SecretBinding, toAdd, expected string) {
			AddTypeToSecretBinding(secretBinding, toAdd)
			Expect(secretBinding.Provider.Type).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", "foo"),
		Entry("when single-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", "foo"),
		Entry("when single-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", "foo,bar"),
		Entry("when multi-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "foo", "foo,bar"),
		Entry("when multi-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", "foo,bar,baz"),
	)

	DescribeTable("#IsCoreDNSAutoscalingModeUsed",
		func(systemComponents *gardencorev1beta1.SystemComponents, autoscalingMode gardencorev1beta1.CoreDNSAutoscalingMode, expected bool) {
			Expect(IsCoreDNSAutoscalingModeUsed(systemComponents, autoscalingMode)).To(Equal(expected))
		},

		Entry("with nil (cluster-proportional)", nil, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with nil (horizontal)", nil, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty system components (cluster-proportional)", &gardencorev1beta1.SystemComponents{}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty system components (horizontal)", &gardencorev1beta1.SystemComponents{}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty core dns (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty core dns (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty core dns autoscaling (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty core dns autoscaling (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
		Entry("with incorrect autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "test"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with incorrect autoscaling mode (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "test"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
		Entry("with horizontal autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "horizontal"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with horizontal autoscaling mode (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "horizontal"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with cluster-proportional autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "cluster-proportional"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, true),
		Entry("with cluster-proportional autoscaling mode (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "cluster-proportional"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
	)

	DescribeTable("#IsNodeLocalDNSEnabled",
		func(systemComponents *gardencorev1beta1.SystemComponents, expected bool) {
			Expect(IsNodeLocalDNSEnabled(systemComponents)).To(Equal(expected))
		},

		Entry("with nil (disabled)", nil, false),
		Entry("with empty system components", &gardencorev1beta1.SystemComponents{}, false),
		Entry("with system components and node-local-dns is enabled", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true}}, true),
		Entry("with system components and node-local-dns is disabled", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: false}}, false),
	)

	DescribeTable("#GetNodeLocalDNS",
		func(systemComponents *gardencorev1beta1.SystemComponents, expected *gardencorev1beta1.NodeLocalDNS) {
			Expect(GetNodeLocalDNS(systemComponents)).To(Equal(expected))
		},
		Entry("with nil", nil, nil),
		Entry("with system components and nil", nil, nil),
		Entry("with system components and node local DNS spec", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true, ForceTCPToClusterDNS: ptr.To(true), ForceTCPToUpstreamDNS: ptr.To(true), DisableForwardToUpstreamDNS: ptr.To(true)}}, &gardencorev1beta1.NodeLocalDNS{Enabled: true, ForceTCPToClusterDNS: ptr.To(true), ForceTCPToUpstreamDNS: ptr.To(true), DisableForwardToUpstreamDNS: ptr.To(true)}),
	)

	DescribeTable("#GetShootCARotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootCARotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("ca nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootCARotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootCARotation(shoot, nil)
			Expect(GetShootCARotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	Describe("#MutateShootKubeconfigRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootKubeconfigRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateShootKubeconfigRotation(shoot, func(rotation *gardencorev1beta1.ShootKubeconfigRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.Kubeconfig.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("kubeconfig nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("kubeconfig non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("kubeconfig nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(-time.Minute)}}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(time.Minute)}}}}, BeFalse()),
	)

	Describe("#MutateShootSSHKeypairRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootSSHKeypairRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.SSHKeypair.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("sshKeypair nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("sshKeypair non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("sshKeypair nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(-time.Minute)}}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(time.Minute)}}}}, BeFalse()),
	)

	Describe("#MutateObservabilityRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateObservabilityRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ObservabilityRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.Observability.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("observability nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("observability non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("observability nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(-time.Minute)}}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(time.Minute)}}}}, BeFalse()),
	)

	DescribeTable("#GetShootServiceAccountKeyRotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootServiceAccountKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("serviceAccountKey nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootServiceAccountKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootServiceAccountKeyRotation(shoot, nil)
			Expect(GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("serviceAccountKey nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("serviceAccountKey non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#GetShootETCDEncryptionKeyRotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootETCDEncryptionKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("etcdEncryptionKey nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootETCDEncryptionKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootETCDEncryptionKeyRotation(shoot, nil)
			Expect(GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.ETCDEncryptionKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("etcdEncryptionKey nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("etcdEncryptionKey non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	Describe("#GetAllZonesFromShoot", func() {
		It("should return an empty list because there are no zones", func() {
			Expect(sets.List(GetAllZonesFromShoot(&gardencorev1beta1.Shoot{}))).To(BeEmpty())
		})

		It("should return the expected list when there is only one pool", func() {
			Expect(sets.List(GetAllZonesFromShoot(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Zones: []string{"a", "b"}},
						},
					},
				},
			}))).To(ConsistOf("a", "b"))
		})

		It("should return the expected list when there are more than one pools", func() {
			Expect(sets.List(GetAllZonesFromShoot(&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{Zones: []string{"a", "c"}},
							{Zones: []string{"b", "d"}},
						},
					},
				},
			}))).To(ConsistOf("a", "b", "c", "d"))
		})
	})

	DescribeTable("#IsFailureToleranceTypeZone",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeZone(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", ptr.To(gardencorev1beta1.FailureToleranceTypeZone), true),
		Entry("failureToleranceType is node", ptr.To(gardencorev1beta1.FailureToleranceTypeNode), false),
		Entry("failureToleranceType is nil", nil, false),
	)

	DescribeTable("#IsFailureToleranceTypeNode",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeNode(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", ptr.To(gardencorev1beta1.FailureToleranceTypeZone), false),
		Entry("failureToleranceType is node", ptr.To(gardencorev1beta1.FailureToleranceTypeNode), true),
		Entry("failureToleranceType is nil", nil, false),
	)

	Describe("#IsHAControlPlaneConfigured", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("return false when HighAvailability is not set", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("return false when ControlPlane is not set", func() {
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("should return true when HighAvailability is set", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{},
			}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeTrue())
		})
	})

	Describe("#IsMultiZonalShootControlPlane", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return false when shoot has no ControlPlane Spec", func() {
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot has no HighAvailability Spec", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot defines failure tolerance type 'node'", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return true when shoot defines failure tolerance type 'zone'", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeTrue())
		})
	})

	Describe("#IsWorkerless", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "worker",
							},
						},
					},
				},
			}
		})

		It("should return false when shoot has workers", func() {
			Expect(IsWorkerless(shoot)).To(BeFalse())
		})

		It("should return true when shoot has zero workers", func() {
			shoot.Spec.Provider.Workers = nil
			Expect(IsWorkerless(shoot)).To(BeTrue())
		})
	})

	Describe("#HasManagedIssuer", func() {
		It("should return false when the shoot does not have managed issuer", func() {
			Expect(HasManagedIssuer(&gardencorev1beta1.Shoot{})).To(BeFalse())
		})

		It("should return true when the shoot has managed issuer", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"authentication.gardener.cloud/issuer": "managed"},
				},
			}
			Expect(HasManagedIssuer(shoot)).To(BeTrue())
		})
	})

	DescribeTable("#ShootEnablesSSHAccess",
		func(workers []gardencorev1beta1.Worker, workersSettings *gardencorev1beta1.WorkersSettings, expectedResult bool) {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers:         workers,
						WorkersSettings: workersSettings,
					},
				},
			}
			Expect(ShootEnablesSSHAccess(shoot)).To(Equal(expectedResult))
		},

		Entry("should return false when shoot provider has zero workers", nil, nil, false),
		Entry("should return true when shoot provider has no WorkersSettings", []gardencorev1beta1.Worker{{}}, nil, true),
		Entry("should return true when shoot worker settings has no SSHAccess", []gardencorev1beta1.Worker{{}}, &gardencorev1beta1.WorkersSettings{}, true),
		Entry("should return true when shoot worker settings has SSHAccess set to true", []gardencorev1beta1.Worker{{}}, &gardencorev1beta1.WorkersSettings{SSHAccess: &gardencorev1beta1.SSHAccess{Enabled: true}}, true),
		Entry("should return false when shoot worker settings has SSHAccess set to false", []gardencorev1beta1.Worker{{}}, &gardencorev1beta1.WorkersSettings{SSHAccess: &gardencorev1beta1.SSHAccess{Enabled: false}}, false),
	)

	Describe("#GetFailureToleranceType", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return 'nil' when ControlPlane is empty", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(GetFailureToleranceType(shoot)).To(BeNil())
		})

		It("should return type 'node' when set in ControlPlane.HighAvailability", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
			}
			Expect(GetFailureToleranceType(shoot)).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeNode)))
		})
	})

	DescribeTable("#IsTopologyAwareRoutingForShootControlPlaneEnabled",
		func(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot, matcher gomegatypes.GomegaMatcher) {
			Expect(IsTopologyAwareRoutingForShootControlPlaneEnabled(seed, shoot)).To(matcher)
		},

		Entry("seed setting is nil, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is disabled, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is enabled, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeFalse(),
		),
		Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeFalse(),
		),
		Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeTrue(),
		),
	)

	DescribeTable("#ShootHasOperationType",
		func(lastOperation *gardencorev1beta1.LastOperation, lastOperationType gardencorev1beta1.LastOperationType, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootHasOperationType(lastOperation, lastOperationType)).To(matcher)
		},
		Entry("last operation nil", nil, gardencorev1beta1.LastOperationTypeCreate, BeFalse()),
		Entry("last operation type does not match", &gardencorev1beta1.LastOperation{}, gardencorev1beta1.LastOperationTypeCreate, BeFalse()),
		Entry("last operation type matches", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, gardencorev1beta1.LastOperationTypeCreate, BeTrue()),
	)

	DescribeTable("#KubeAPIServerFeatureGateDisabled",
		func(shoot *gardencorev1beta1.Shoot, featureGate string, expected bool) {
			actual := KubeAPIServerFeatureGateDisabled(shoot, featureGate)
			Expect(actual).To(Equal(expected))
		},

		Entry("with kubeAPIServerConfig=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: nil,
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("with kubeAPIServerConfig.featureGates=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: nil,
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate does not exist",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBaz": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is enabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is disabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": false,
								},
							},
						},
					},
				},
			},
			"FooBar",
			true,
		),
	)

	DescribeTable("#KubeControllerManagerFeatureGateDisabled",
		func(shoot *gardencorev1beta1.Shoot, featureGate string, expected bool) {
			actual := KubeControllerManagerFeatureGateDisabled(shoot, featureGate)
			Expect(actual).To(Equal(expected))
		},

		Entry("with kubeControllerManager=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: nil,
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("with kubeControllerManager.featureGates=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: nil,
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate does not exist",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBaz": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is enabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is disabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": false,
								},
							},
						},
					},
				},
			},
			"FooBar",
			true,
		),
	)

	DescribeTable("#KubeProxyFeatureGateDisabled",
		func(shoot *gardencorev1beta1.Shoot, featureGate string, expected bool) {
			actual := KubeProxyFeatureGateDisabled(shoot, featureGate)
			Expect(actual).To(Equal(expected))
		},

		Entry("with kubeProxy=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeProxy: nil,
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("with kubeProxy.featureGates=nil",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeProxy: &gardencorev1beta1.KubeProxyConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: nil,
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate does not exist",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeProxy: &gardencorev1beta1.KubeProxyConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBaz": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is enabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeProxy: &gardencorev1beta1.KubeProxyConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": true,
								},
							},
						},
					},
				},
			},
			"FooBar",
			false,
		),
		Entry("when feature gate exists and is disabled",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeProxy: &gardencorev1beta1.KubeProxyConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									"FooBar": false,
								},
							},
						},
					},
				},
			},
			"FooBar",
			true,
		),
	)

	Describe("#ConvertShootList", func() {
		It("should convert a list of Shoots", func() {
			shootList := &gardencorev1beta1.ShootList{
				Items: []gardencorev1beta1.Shoot{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "shoot1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "shoot2"},
					},
				},
			}

			converted := ConvertShootList(shootList.Items)
			Expect(converted).To(HaveLen(2))
			Expect(converted[0].Name).To(Equal("shoot1"))
			Expect(converted[1].Name).To(Equal("shoot2"))
		})
	})

	DescribeTable("#SumResourceReservations",
		func(left, right, expected *gardencorev1beta1.KubeletConfigReserved) {
			actual := SumResourceReservations(left, right)
			Expect(actual).To(Equal(expected))
		},

		Entry("should return right when left is nil",
			nil,
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI)},
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI)},
		),
		Entry("should return left when right is nil",
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI)},
			nil,
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI)},
		),
		Entry("should sum left and right",
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(50, resource.DecimalSI), Memory: resource.NewQuantity(55, resource.DecimalSI), EphemeralStorage: resource.NewQuantity(60, resource.DecimalSI)},
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(100, resource.DecimalSI), Memory: resource.NewQuantity(105, resource.DecimalSI), PID: resource.NewQuantity(10, resource.DecimalSI)},
			&gardencorev1beta1.KubeletConfigReserved{CPU: resource.NewQuantity(150, resource.DecimalSI), Memory: resource.NewQuantity(160, resource.DecimalSI), EphemeralStorage: resource.NewQuantity(60, resource.DecimalSI), PID: resource.NewQuantity(10, resource.DecimalSI)},
		),
	)
})
