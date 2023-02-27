// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
})
