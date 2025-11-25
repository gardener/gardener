// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#ClusterAutoscalerRequired",
		func(pools []extensionsv1alpha1.WorkerPool, expected bool) {
			Expect(ClusterAutoscalerRequired(pools)).To(Equal(expected))
		},

		Entry("no pools", []extensionsv1alpha1.WorkerPool{}, false),
		Entry("min=max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 1,
			Maximum: 1,
		}}, false),
		Entry("min<max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 0,
			Maximum: 1,
		}}, true),
	)

	Describe("#GetMachineDeploymentClusterAutoscalerAnnotations", func() {
		It("should return annotations with value \"\" when options passed is nil", func() {
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation:    "",
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "",
				extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation:             "",
				extensionsv1alpha1.MaxNodeProvisionTimeAnnotation:             "",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(nil)).To(Equal(expectedValues))
		})

		It("should return annotations with value \"\" when an empty map is passed", func() {
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation:    "",
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "",
				extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation:             "",
				extensionsv1alpha1.MaxNodeProvisionTimeAnnotation:             "",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(ptr.To(extensionsv1alpha1.ClusterAutoscalerOptions{}))).To(Equal(expectedValues))
		})

		It("should return correctly populated map when all options are passed", func() {
			caOptions := &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownUtilizationThreshold:    ptr.To("0.5"),
				ScaleDownGpuUtilizationThreshold: ptr.To("0.6"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: time.Minute}),
				ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
			}
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation:    "0.5",
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "0.6",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "1m0s",
				extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation:             "2m0s",
				extensionsv1alpha1.MaxNodeProvisionTimeAnnotation:             "3m0s",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(caOptions)).To(Equal(expectedValues))
		})

		It("should return correctly populated map when partial options are passed", func() {
			caOptions := &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownGpuUtilizationThreshold: ptr.To("0.6"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: time.Minute}),
			}
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation:    "",
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "0.6",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "1m0s",
				extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation:             "",
				extensionsv1alpha1.MaxNodeProvisionTimeAnnotation:             "",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(caOptions)).To(Equal(expectedValues))
		})
	})
})
