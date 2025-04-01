// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
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
		trueVar  = true
		falseVar = false
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

	DescribeTable("#SystemComponentsAllowed",
		func(worker *gardencorev1beta1.Worker, allowsSystemComponents bool) {
			Expect(SystemComponentsAllowed(worker)).To(Equal(allowsSystemComponents))
		},
		Entry("no systemComponents section", &gardencorev1beta1.Worker{}, true),
		Entry("systemComponents.allowed = false", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: false}}, false),
		Entry("systemComponents.allowed = true", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true}}, true),
	)

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

	Describe("#ShouldPrepareShootForMigration", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("seed"),
				},
				Status: gardencorev1beta1.ShootStatus{
					SeedName: ptr.To("seed"),
				},
			}
		})

		It("should return false if spec.seedName is not set", func() {
			shoot.Spec.SeedName = nil
			Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())

			shoot.Status.SeedName = nil
			Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
		})

		It("should return false if status.seedName is not set", func() {
			shoot.Status.SeedName = nil
			Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
		})

		It("should return false if spec.seedName and status.seedName are equal", func() {
			Expect(ShouldPrepareShootForMigration(shoot)).To(BeFalse())
		})

		It("should return true if spec.seedName and status.seedName differ", func() {
			shoot.Spec.SeedName = ptr.To("other")
			Expect(ShouldPrepareShootForMigration(shoot)).To(BeTrue())
		})
	})

	Describe("#LastInitiationTimeForWorkerPool", func() {
		var (
			poolName                 = "pool"
			globalLastInitiationTime = ptr.To(metav1.Now())
			poolLastInitiationTime   = ptr.To(metav1.Time{Time: time.Now().Add(-time.Hour)})
		)

		It("should return the global last initiation time because list is empty", func() {
			Expect(LastInitiationTimeForWorkerPool(poolName, nil, globalLastInitiationTime)).To(Equal(globalLastInitiationTime))
		})

		It("should return the global last initiation time because pool is not found in list", func() {
			Expect(LastInitiationTimeForWorkerPool(poolName, []gardencorev1beta1.PendingWorkersRollout{{}}, globalLastInitiationTime)).To(Equal(globalLastInitiationTime))
		})

		It("should return the pool-specific last initiation time because pool is found in list", func() {
			Expect(LastInitiationTimeForWorkerPool(poolName, []gardencorev1beta1.PendingWorkersRollout{{Name: poolName, LastInitiationTime: poolLastInitiationTime}}, globalLastInitiationTime)).To(Equal(poolLastInitiationTime))
		})
	})

	Describe("#IsShootAutonomous", func() {
		It("should return true (single worker pool with control plane configuration)", func() {
			shoot := &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
				{ControlPlane: &gardencorev1beta1.WorkerControlPlane{}},
			}}}}
			Expect(IsShootAutonomous(shoot)).To(BeTrue())
		})

		It("should return true (multiple worker pools, one with control plane configuration)", func() {
			shoot := &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
				{},
				{ControlPlane: &gardencorev1beta1.WorkerControlPlane{}},
				{},
			}}}}
			Expect(IsShootAutonomous(shoot)).To(BeTrue())
		})

		It("should return false (no worker pools)", func() {
			shoot := &gardencorev1beta1.Shoot{}
			Expect(IsShootAutonomous(shoot)).To(BeFalse())
		})

		It("should return false (worker pools, but none with control plane configuration)", func() {
			shoot := &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
				{},
				{},
				{},
			}}}}
			Expect(IsShootAutonomous(shoot)).To(BeFalse())
		})
	})

	Describe("#ControlPlaneWorkerPoolForShoot", func() {
		It("should return nil because shoot is not autonomous", func() {
			shoot := &gardencorev1beta1.Shoot{}
			Expect(ControlPlaneWorkerPoolForShoot(shoot)).To(BeNil())
		})

		It("should return the worker pool", func() {
			worker := gardencorev1beta1.Worker{
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
				Name:         "cp",
			}
			shoot := &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{worker}}}}
			Expect(ControlPlaneWorkerPoolForShoot(shoot)).To(PointTo(Equal(worker)))
		})
	})

	Describe("#ControlPlaneNamespaceForShoot", func() {
		It("should return kube-system for autonomous shoots", func() {
			shoot := &gardencorev1beta1.Shoot{
				Spec:   gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{{ControlPlane: &gardencorev1beta1.WorkerControlPlane{}}}}},
				Status: gardencorev1beta1.ShootStatus{TechnicalID: "shoot--foo--bar"},
			}
			Expect(ControlPlaneNamespaceForShoot(shoot)).To(Equal("kube-system"))
		})

		It("should return the technical ID for regular shoots", func() {
			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{TechnicalID: "shoot--foo--bar"},
			}
			Expect(ControlPlaneNamespaceForShoot(shoot)).To(Equal("shoot--foo--bar"))
		})
	})

	DescribeTable("#IsUpdateStrategyInPlace",
		func(updateStrategy *gardencorev1beta1.MachineUpdateStrategy, expected bool) {
			Expect(IsUpdateStrategyInPlace(updateStrategy)).To(Equal(expected))
		},

		Entry("with nil", nil, false),
		Entry("with AutoRollingUpdate update strategy", ptr.To(gardencorev1beta1.AutoRollingUpdate), false),
		Entry("with AutoInPlaceUpdate update strategy", ptr.To(gardencorev1beta1.AutoInPlaceUpdate), true),
		Entry("with ManualInPlaceUpdate  update strategy", ptr.To(gardencorev1beta1.ManualInPlaceUpdate), true),
	)

	DescribeTable("#IsShootIstioTLSTerminationEnabled",
		func(shootAnnotations map[string]string, expected bool) {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: shootAnnotations,
				},
			}
			Expect(IsShootIstioTLSTerminationEnabled(shoot)).To(Equal(expected))
		},

		Entry("shoot has Istio TLS termination if it has no annotations", nil, true),
		Entry("shoot has no Istio TLS termination if is disabled by annotation", map[string]string{"shoot.gardener.cloud/disable-istio-tls-termination": "true"}, false),
		Entry("shoot has no Istio TLS termination if is not disabled by annotation", map[string]string{"shoot.gardener.cloud/disable-istio-tls-termination": "false"}, true),
		Entry("shoot has no Istio TLS termination if it is annotated with a bogus value", map[string]string{"shoot.gardener.cloud/disable-istio-tls-termination": "foobar"}, true),
	)
})
