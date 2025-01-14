// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("#ReadManagedSeedAPIServer", func() {
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
