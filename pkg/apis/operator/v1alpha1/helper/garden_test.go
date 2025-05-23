// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	DescribeTable("#GetCARotationPhase",
		func(credentials *operatorv1alpha1.Credentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetCARotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &operatorv1alpha1.Credentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("ca nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateCARotation", func() {
		It("should do nothing when mutate function is nil", func() {
			garden := &operatorv1alpha1.Garden{}
			MutateCARotation(garden, nil)
			Expect(GetCARotationPhase(garden.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(garden *operatorv1alpha1.Garden, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateCARotation(garden, func(rotation *gardencorev1beta1.CARotation) {
					rotation.Phase = phase
				})
				Expect(garden.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &operatorv1alpha1.Garden{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#GetServiceAccountKeyRotationPhase",
		func(credentials *operatorv1alpha1.Credentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetServiceAccountKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &operatorv1alpha1.Credentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("sa nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateServiceAccountKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			garden := &operatorv1alpha1.Garden{}
			MutateServiceAccountKeyRotation(garden, nil)
			Expect(GetServiceAccountKeyRotationPhase(garden.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(garden *operatorv1alpha1.Garden, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateServiceAccountKeyRotation(garden, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
					rotation.Phase = phase
				})
				Expect(garden.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &operatorv1alpha1.Garden{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#GetETCDEncryptionKeyRotationPhase",
		func(credentials *operatorv1alpha1.Credentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetETCDEncryptionKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &operatorv1alpha1.Credentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("etcd nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateETCDEncryptionKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			garden := &operatorv1alpha1.Garden{}
			MutateETCDEncryptionKeyRotation(garden, nil)
			Expect(GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(garden *operatorv1alpha1.Garden, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateETCDEncryptionKeyRotation(garden, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
					rotation.Phase = phase
				})
				Expect(garden.Status.Credentials.Rotation.ETCDEncryptionKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &operatorv1alpha1.Garden{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#GetWorkloadIdentityKeyRotationPhase",
		func(credentials *operatorv1alpha1.Credentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetWorkloadIdentityKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &operatorv1alpha1.Credentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("workload identity nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateWorkloadIdentityKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			garden := &operatorv1alpha1.Garden{}
			MutateWorkloadIdentityKeyRotation(garden, nil)
			Expect(GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(garden *operatorv1alpha1.Garden, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateWorkloadIdentityKeyRotation(garden, func(rotation *operatorv1alpha1.WorkloadIdentityKeyRotation) {
					rotation.Phase = phase
				})
				Expect(garden.Status.Credentials.Rotation.WorkloadIdentityKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &operatorv1alpha1.Garden{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *operatorv1alpha1.Credentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &operatorv1alpha1.Credentials{}, BeFalse()),
		Entry("observability nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(-time.Minute)}}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Time}}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{LastInitiationTime: &metav1.Time{Time: metav1.Now().Time}, LastCompletionTime: &metav1.Time{Time: metav1.Now().Add(time.Minute)}}}}, BeFalse()),
	)

	Describe("#MutateObservabilityRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			garden := &operatorv1alpha1.Garden{}
			MutateObservabilityRotation(garden, nil)
			Expect(garden.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(garden *operatorv1alpha1.Garden, lastInitiationTime metav1.Time) {
				MutateObservabilityRotation(garden, func(rotation *gardencorev1beta1.ObservabilityRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(garden.Status.Credentials.Rotation.Observability.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &operatorv1alpha1.Garden{}, metav1.Now()),
			Entry("rotation nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{}}}, metav1.Now()),
			Entry("observability nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{}}}}, metav1.Now()),
			Entry("observability non-nil", &operatorv1alpha1.Garden{Status: operatorv1alpha1.GardenStatus{Credentials: &operatorv1alpha1.Credentials{Rotation: &operatorv1alpha1.CredentialsRotation{Observability: &gardencorev1beta1.ObservabilityRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#HighAvailabilityEnabled",
		func(controlPlane *operatorv1alpha1.ControlPlane, expected bool) {
			garden := &operatorv1alpha1.Garden{}
			garden.Spec.VirtualCluster.ControlPlane = controlPlane

			Expect(HighAvailabilityEnabled(garden)).To(Equal(expected))
		},

		Entry("no control-plane", nil, false),
		Entry("no high-availability", &operatorv1alpha1.ControlPlane{HighAvailability: nil}, false),
		Entry("high-availability set", &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}, true),
	)

	DescribeTable("#TopologyAwareRoutingEnabled",
		func(settings *operatorv1alpha1.Settings, expected bool) {
			Expect(TopologyAwareRoutingEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, false),
		Entry("no topology-aware routing setting", &operatorv1alpha1.Settings{}, false),
		Entry("topology-aware routing enabled", &operatorv1alpha1.Settings{TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{Enabled: true}}, true),
		Entry("topology-aware routing disabled", &operatorv1alpha1.Settings{TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{Enabled: false}}, false),
	)

	DescribeTable("#GetETCDMainBackup",
		func(garden *operatorv1alpha1.Garden, expected *operatorv1alpha1.Backup) {
			Expect(GetETCDMainBackup(garden)).To(Equal(expected))
		},
		Entry("no garden", nil, nil),
		Entry("no ETCD config", &operatorv1alpha1.Garden{}, nil),
		Entry("no ETCD Main config", &operatorv1alpha1.Garden{Spec: operatorv1alpha1.GardenSpec{VirtualCluster: operatorv1alpha1.VirtualCluster{ETCD: &operatorv1alpha1.ETCD{}}}}, nil),
		Entry("no backup config", &operatorv1alpha1.Garden{Spec: operatorv1alpha1.GardenSpec{VirtualCluster: operatorv1alpha1.VirtualCluster{ETCD: &operatorv1alpha1.ETCD{Main: &operatorv1alpha1.ETCDMain{}}}}}, nil),
		Entry("with backup config", &operatorv1alpha1.Garden{Spec: operatorv1alpha1.GardenSpec{VirtualCluster: operatorv1alpha1.VirtualCluster{ETCD: &operatorv1alpha1.ETCD{Main: &operatorv1alpha1.ETCDMain{Backup: &operatorv1alpha1.Backup{Provider: "test"}}}}}}, &operatorv1alpha1.Backup{Provider: "test"}),
	)

	DescribeTable("#GetDNSProviders",
		func(garden *operatorv1alpha1.Garden, expected []operatorv1alpha1.DNSProvider) {
			Expect(GetDNSProviders(garden)).To(Equal(expected))
		},
		Entry("no garden", nil, nil),
		Entry("no DNS config", &operatorv1alpha1.Garden{}, nil),
		Entry("no DNS providers", &operatorv1alpha1.Garden{Spec: operatorv1alpha1.GardenSpec{DNS: &operatorv1alpha1.DNSManagement{}}}, nil),
		Entry("with DNS providers", &operatorv1alpha1.Garden{Spec: operatorv1alpha1.GardenSpec{DNS: &operatorv1alpha1.DNSManagement{Providers: []operatorv1alpha1.DNSProvider{{Name: "provider-1"}, {Name: "provider-2"}}}}}, []operatorv1alpha1.DNSProvider{{Name: "provider-1"}, {Name: "provider-2"}}),
	)
})
